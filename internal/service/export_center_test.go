package service

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

func TestExportCenterServiceCreateListDetailAndSuccess(t *testing.T) {
	db := newExportCenterTestDB(t)
	svc := NewExportCenterServiceWithDeps(ExportCenterServiceDeps{DB: db})

	record, err := svc.CreateRecord(context.Background(), ExportRecordCreateInput{
		Scene:         model.ExportSceneRequestLog,
		TaskRunID:     101,
		TriggerUserID: 9,
		Queue:         "export",
		Format:        "csv",
	})
	if err != nil {
		t.Fatalf("create record failed: %v", err)
	}
	if record.Status != model.TaskRunStatusPending || record.Progress != exportProgressPending {
		t.Fatalf("unexpected initial record: %#v", record)
	}

	if err := svc.MarkRunningWithTotal(context.Background(), 101, 4); err != nil {
		t.Fatalf("mark running failed: %v", err)
	}
	if err := svc.UpdateProgress(context.Background(), 101, 2, 4); err != nil {
		t.Fatalf("update progress failed: %v", err)
	}

	file := &model.UploadFiles{
		DisplayName: "request_logs_20260518.csv",
		UUID:        "file-uuid",
		Size:        2048,
	}
	file.ID = 7
	if err := svc.MarkSuccess(context.Background(), 101, ExportRecordSuccessInput{File: file}); err != nil {
		t.Fatalf("mark success failed: %v", err)
	}

	list := svc.List(&form.ExportRecordList{
		Paginate: form.Paginate{Page: 1, PerPage: 10},
		Scene:    model.ExportSceneRequestLog,
		Status:   model.TaskRunStatusSuccess,
	})
	if list.Total != 1 || len(list.Data) != 1 {
		t.Fatalf("unexpected list result: %#v", list)
	}

	detailAny, err := svc.Detail(record.ID)
	if err != nil {
		t.Fatalf("detail failed: %v", err)
	}
	detail, ok := detailAny.(*resources.ExportRecordResources)
	if !ok {
		t.Fatalf("unexpected detail type: %T", detailAny)
	}
	if detail.FileID != 7 || detail.FileUUID != "file-uuid" || detail.FileSize != 2048 {
		t.Fatalf("unexpected detail payload: %#v", detail)
	}
	if detail.Status != model.TaskRunStatusSuccess || detail.Progress != exportProgressDone {
		t.Fatalf("unexpected detail status: %#v", detail)
	}
	if detail.TotalCount != 4 || detail.ProcessedCount != 4 {
		t.Fatalf("unexpected progress counts: %#v", detail)
	}
	if detail.ExportName != "请求日志" || detail.SceneName != "请求日志" {
		t.Fatalf("unexpected export naming: %#v", detail)
	}
	if detail.TaskCode != exportTaskCodeRequestLog {
		t.Fatalf("unexpected task code: got=%s want=%s", detail.TaskCode, exportTaskCodeRequestLog)
	}
	if detail.StatusName != "导出成功" || detail.Stage != "download_ready" || detail.StageName != "可下载" {
		t.Fatalf("unexpected status semantics: %#v", detail)
	}
	if !detail.HasFile || !detail.DownloadReady {
		t.Fatalf("expected file to be downloadable: %#v", detail)
	}
	if detail.DownloadURL == "" || detail.FileURL != detail.DownloadURL {
		t.Fatalf("unexpected download fields: %#v", detail)
	}
	if detail.FailReason != "" {
		t.Fatalf("expected empty fail reason on success: %#v", detail)
	}
}

func TestExportCenterServiceMarkFailed(t *testing.T) {
	db := newExportCenterTestDB(t)
	svc := NewExportCenterServiceWithDeps(ExportCenterServiceDeps{DB: db})

	if _, err := svc.CreateRecord(context.Background(), ExportRecordCreateInput{
		Scene:         model.ExportSceneRequestLog,
		TaskRunID:     202,
		TriggerUserID: 8,
		Queue:         "export",
		Format:        "csv",
	}); err != nil {
		t.Fatalf("create record failed: %v", err)
	}

	if err := svc.MarkFailed(context.Background(), 202, errors.New("publish failed")); err != nil {
		t.Fatalf("mark failed failed: %v", err)
	}

	var stored model.SysExportRecord
	if err := db.Where("task_run_id = ?", 202).First(&stored).Error; err != nil {
		t.Fatalf("query stored record failed: %v", err)
	}
	if stored.Status != model.TaskRunStatusFailed || stored.ErrorMessage != "publish failed" {
		t.Fatalf("unexpected failed record: %#v", stored)
	}
	if stored.FinishedAt == nil || stored.FinishedAt.IsZero() {
		t.Fatalf("expected finished_at to be set: %#v", stored)
	}

	detailAny, err := svc.Detail(stored.ID)
	if err != nil {
		t.Fatalf("detail failed: %v", err)
	}
	detail, ok := detailAny.(*resources.ExportRecordResources)
	if !ok {
		t.Fatalf("unexpected detail type: %T", detailAny)
	}
	if detail.StatusName != "导出失败" || detail.Stage != "failed" || detail.StageName != "失败" {
		t.Fatalf("unexpected failed semantics: %#v", detail)
	}
	if detail.FailReason != "publish failed" || detail.ErrorMessage != "publish failed" {
		t.Fatalf("unexpected fail reason fields: %#v", detail)
	}
	if detail.Progress != exportProgressPending || detail.TotalCount != 0 || detail.ProcessedCount != 0 {
		t.Fatalf("unexpected failed progress fields: %#v", detail)
	}
	if detail.HasFile || detail.DownloadReady || detail.DownloadURL != "" || detail.FileURL != "" {
		t.Fatalf("failed record should not expose download fields: %#v", detail)
	}
}

func TestExportCenterServiceCreateRecordForTaskAndCancel(t *testing.T) {
	db := newExportCenterTestDB(t)
	svc := NewExportCenterServiceWithDeps(ExportCenterServiceDeps{DB: db})

	record, matched, err := svc.CreateRecordForTask(context.Background(), ExportTaskBootstrapInput{
		TaskCode:      "export:request-log",
		TaskRunID:     303,
		TriggerUserID: 6,
		Queue:         "export",
	})
	if err != nil {
		t.Fatalf("create record for task failed: %v", err)
	}
	if !matched || record == nil {
		t.Fatalf("expected request-log task to create export record")
	}

	if err := svc.MarkCanceledIfExists(context.Background(), 303, "manual cancel"); err != nil {
		t.Fatalf("mark canceled failed: %v", err)
	}

	var stored model.SysExportRecord
	if err := db.Where("task_run_id = ?", 303).First(&stored).Error; err != nil {
		t.Fatalf("query canceled record failed: %v", err)
	}
	if stored.Status != model.TaskRunStatusCanceled || stored.ErrorMessage != "manual cancel" {
		t.Fatalf("unexpected canceled record: %#v", stored)
	}
}

func TestResolveExportSceneFileList(t *testing.T) {
	scene, format, ok := resolveExportScene("export:file-list")
	if !ok {
		t.Fatalf("expected export:file-list task code to resolve")
	}
	if scene != model.ExportSceneFileList {
		t.Fatalf("unexpected scene: got=%s want=%s", scene, model.ExportSceneFileList)
	}
	if format != "csv" {
		t.Fatalf("unexpected format: got=%s want=csv", format)
	}
}

func TestResolveExportSceneNameMap(t *testing.T) {
	cases := map[string]string{
		model.ExportSceneRequestLog: "请求日志",
		model.ExportSceneFileList:   "文件资源",
		"":                          "",
		"unknown":                   "",
	}
	for scene, expected := range cases {
		if got := resolveExportSceneName(scene); got != expected {
			t.Fatalf("scene=%q expected name=%q got=%q", scene, expected, got)
		}
	}
}

func TestExportCenterServiceListExposesQueuedStageAndTaskCode(t *testing.T) {
	db := newExportCenterTestDB(t)
	svc := NewExportCenterServiceWithDeps(ExportCenterServiceDeps{DB: db})

	if _, err := svc.CreateRecord(context.Background(), ExportRecordCreateInput{
		Scene:         model.ExportSceneFileList,
		TaskRunID:     404,
		TriggerUserID: 3,
		Queue:         "export",
		Format:        "csv",
	}); err != nil {
		t.Fatalf("create record failed: %v", err)
	}

	list := svc.List(&form.ExportRecordList{
		Paginate: form.Paginate{Page: 1, PerPage: 10},
		Scene:    model.ExportSceneFileList,
	})
	if list.Total != 1 || len(list.Data) != 1 {
		t.Fatalf("unexpected list result: %#v", list)
	}

	item, ok := list.Data[0].(*resources.ExportRecordListResources)
	if !ok {
		t.Fatalf("unexpected list item type: %T", list.Data[0])
	}
	if item.ExportName != "文件资源" || item.SceneName != "文件资源" {
		t.Fatalf("unexpected export naming: %#v", item)
	}
	if item.TaskCode != exportTaskCodeFileList {
		t.Fatalf("unexpected task code: got=%s want=%s", item.TaskCode, exportTaskCodeFileList)
	}
	if item.Status != model.TaskRunStatusPending || item.StatusName != "等待执行" {
		t.Fatalf("unexpected queued status: %#v", item)
	}
	if item.Stage != "queued" || item.StageName != "排队中" {
		t.Fatalf("unexpected queued stage: %#v", item)
	}
	if item.TotalCount != 0 || item.ProcessedCount != 0 {
		t.Fatalf("expected queued counts to be zero: %#v", item)
	}
	if item.HasFile || item.DownloadReady || item.DownloadURL != "" || item.FileURL != "" {
		t.Fatalf("queued record should not expose download fields: %#v", item)
	}
}

// TestExportCenterServiceReassignTaskRun 验证重试时复用同一条导出记录、重置状态、改挂新的 task_run_id。
func TestExportCenterServiceReassignTaskRun(t *testing.T) {
	db := newExportCenterTestDB(t)
	svc := NewExportCenterServiceWithDeps(ExportCenterServiceDeps{DB: db})

	record, err := svc.CreateRecord(context.Background(), ExportRecordCreateInput{
		Scene:         model.ExportSceneRequestLog,
		TaskRunID:     101,
		TriggerUserID: 9,
		Queue:         "export",
		Format:        "csv",
	})
	if err != nil {
		t.Fatalf("create record failed: %v", err)
	}
	// 模拟首次 run 失败，写入失败态。
	if err := svc.MarkFailed(context.Background(), 101, errors.New("boom")); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	reassigned, err := svc.ReassignTaskRun(context.Background(), 101, 202, "export-retry")
	if err != nil {
		t.Fatalf("reassign failed: %v", err)
	}
	if !reassigned {
		t.Fatalf("expected reassigned=true for existing record")
	}

	// 旧 task_run_id 应该已经查不到
	oldID := uint(101)
	listOld := svc.List(&form.ExportRecordList{
		Paginate:  form.Paginate{Page: 1, PerPage: 10},
		TaskRunID: &oldID,
	})
	if listOld.Total != 0 {
		t.Fatalf("expected old task_run_id 101 detached, got total=%d", listOld.Total)
	}

	newID := uint(202)
	listNew := svc.List(&form.ExportRecordList{
		Paginate:  form.Paginate{Page: 1, PerPage: 10},
		TaskRunID: &newID,
	})
	if listNew.Total != 1 || len(listNew.Data) != 1 {
		t.Fatalf("expected one record on new task_run_id 202, got %#v", listNew)
	}
	item, ok := listNew.Data[0].(*resources.ExportRecordListResources)
	if !ok {
		t.Fatalf("unexpected list item type: %T", listNew.Data[0])
	}
	if item.ID != record.ID {
		t.Fatalf("expected same record id (no new row), got %d want %d", item.ID, record.ID)
	}
	if item.TaskRunID != 202 {
		t.Fatalf("expected task_run_id=202, got %d", item.TaskRunID)
	}
	if item.Status != model.TaskRunStatusPending || item.Progress != exportProgressPending {
		t.Fatalf("expected status reset to pending, got %#v", item)
	}
	if item.TotalCount != 0 || item.ProcessedCount != 0 {
		t.Fatalf("expected counts reset to zero, got %#v", item)
	}
	if item.Queue != "export-retry" {
		t.Fatalf("expected queue updated to export-retry, got %s", item.Queue)
	}
	if item.FileID != 0 || item.FileUUID != "" || item.FileName != "" || item.FileSize != 0 {
		t.Fatalf("expected file fields cleared, got %#v", item)
	}
	if item.ErrorMessage != "" {
		t.Fatalf("expected error_message cleared, got %q", item.ErrorMessage)
	}

	// 不存在原 task_run_id 时返回 false 而非报错
	missing, err := svc.ReassignTaskRun(context.Background(), 9999, 8888, "export")
	if err != nil {
		t.Fatalf("expected nil error when no existing record, got %v", err)
	}
	if missing {
		t.Fatalf("expected reassigned=false when no existing record")
	}
}

func newExportCenterTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	// 重置 sys_export_records 表探测缓存：包级 sync.Once 会跨用例残留，
	// 这里每次创建测试 db 时显式清空，避免不同测试用例的 in-memory sqlite 相互影响。
	ResetExportRecordsTableCacheForTest()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	statements := []string{
		`CREATE TABLE sys_export_records (
			id integer primary key autoincrement,
			scene text,
			task_run_id integer,
			trigger_user_id integer,
			status text,
			queue text,
			progress integer,
			total_count integer,
			processed_count integer,
			file_id integer,
			file_uuid text,
			file_name text,
			file_size integer,
			format text,
			error_message text,
			started_at datetime,
			finished_at datetime,
			created_at datetime,
			updated_at datetime
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create export center test table failed: %v", err)
		}
	}
	return db
}
