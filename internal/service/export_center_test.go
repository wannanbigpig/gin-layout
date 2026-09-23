package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

func TestMyExportRecordsAreOwnerScopedAndHideDiagnostics(t *testing.T) {
	db := newExportCenterTestDB(t)
	svc := NewExportCenterServiceWithDeps(ExportCenterServiceDeps{DB: db})
	first, err := svc.CreateRecord(context.Background(), ExportRecordCreateInput{Scene: model.ExportSceneRequestLog, TaskRunID: 101, TriggerUserID: 9, Queue: "export", Format: "csv"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.CreateRecord(context.Background(), ExportRecordCreateInput{Scene: model.ExportSceneFileList, TaskRunID: 102, TriggerUserID: 10, Queue: "export", Format: "csv"})
	if err != nil {
		t.Fatal(err)
	}
	if first.PublicID == "" || second.PublicID == "" || first.PublicID == second.PublicID {
		t.Fatalf("public identifiers must be unique: %q %q", first.PublicID, second.PublicID)
	}
	otherUserID := uint(10)
	params := &form.ExportRecordList{Paginate: form.Paginate{Page: 1, PerPage: 10}, TriggerUserID: &otherUserID}
	list, err := svc.MyList(context.Background(), 9, params)
	if err != nil || list.Total != 1 || len(list.Data) != 1 {
		t.Fatalf("owner list mismatch: %#v, %v", list, err)
	}
	encoded, err := json.Marshal(list.Data[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"\"task_run_id\"", "\"trigger_user_id\"", "\"queue\"", "\"error_message\"", "\"file_id\"", "\"id\""} {
		if strings.Contains(string(encoded), field) {
			t.Errorf("user export response exposes %s: %s", field, encoded)
		}
	}
	if !strings.Contains(string(encoded), first.PublicID) {
		t.Fatalf("public id missing: %s", encoded)
	}
	if _, err := svc.MyDetail(context.Background(), 9, second.PublicID); err == nil {
		t.Fatal("cross-user detail must not be returned")
	}
	if detail, err := svc.MyDetail(context.Background(), 9, first.PublicID); err != nil || detail.PublicID != first.PublicID {
		t.Fatalf("own detail mismatch: %#v, %v", detail, err)
	} else if detailJSON, err := json.Marshal(detail); err != nil {
		t.Fatal(err)
	} else if strings.Contains(string(detailJSON), "task_run_id") || strings.Contains(string(detailJSON), "error_message") {
		t.Fatalf("own detail exposed diagnostics: %s", detailJSON)
	}
}

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
	completed, err := svc.CompletedRecordForTaskRun(context.Background(), 101)
	if err != nil {
		t.Fatalf("query completed record failed: %v", err)
	}
	if completed == nil || completed.FileID != file.ID || completed.FileUUID != file.UUID {
		t.Fatalf("completed record should be reusable for redelivery: %#v", completed)
	}
	missing, err := svc.CompletedRecordForTaskRun(context.Background(), 999)
	if err != nil || missing != nil {
		t.Fatalf("missing completed record mismatch: record=%#v err=%v", missing, err)
	}

	// 验证无属主调用 listInternal 会被安全拦截
	if _, err := svc.listInternal(&form.ExportRecordList{
		Paginate: form.Paginate{Page: 1, PerPage: 10},
	}); err == nil {
		t.Fatal("expected error when listing export records without owner trigger_user_id, got nil")
	}

	list, err := svc.MyList(context.Background(), record.TriggerUserID, &form.ExportRecordList{
		Paginate: form.Paginate{Page: 1, PerPage: 10},
		Scene:    model.ExportSceneRequestLog,
		Status:   model.TaskRunStatusSuccess,
	})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
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

func TestExportCenterDetailPreservesDatabaseFailureAfterTableProbe(t *testing.T) {
	db := newExportCenterTestDB(t)
	ResetExportRecordsTableCacheForTest()
	if err := ensureExportRecordTable(db); err != nil {
		t.Fatalf("probe export table: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = NewExportCenterServiceWithDeps(ExportCenterServiceDeps{DB: db}).Detail(1)
	if err == nil {
		t.Fatal("expected database error")
	}
	var businessErr *e.BusinessError
	if errors.As(err, &businessErr) && businessErr.GetCode() == e.NotFound {
		t.Fatalf("database failure must not be reported as NotFound: %v", err)
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

func TestExportCenterServiceExpiredFileIsNotDownloadable(t *testing.T) {
	db := newExportCenterTestDB(t)
	svc := NewExportCenterServiceWithDeps(ExportCenterServiceDeps{DB: db})

	if _, err := svc.CreateRecord(context.Background(), ExportRecordCreateInput{
		Scene: model.ExportSceneRequestLog, TaskRunID: 210, Queue: "export", Format: "csv",
	}); err != nil {
		t.Fatalf("create record failed: %v", err)
	}
	file := &model.UploadFiles{UUID: "expired-file", Size: 128, ExpiresAt: utils.FormatDate{Time: time.Now().Add(-time.Minute)}}
	file.ID = 30
	if err := svc.MarkSuccess(context.Background(), 210, ExportRecordSuccessInput{File: file, FileName: "expired.csv"}); err != nil {
		t.Fatalf("mark success failed: %v", err)
	}

	detailAny, err := svc.Detail(1)
	if err != nil {
		t.Fatalf("detail failed: %v", err)
	}
	detail := detailAny.(*resources.ExportRecordResources)
	if !detail.FileExpired || detail.DownloadReady || detail.DownloadURL != "" {
		t.Fatalf("expired file must not be downloadable: %#v", detail)
	}
	if detail.Stage != "expired" || detail.StageName != "已过期" || detail.FileExpiresAt == nil {
		t.Fatalf("unexpected expired file semantics: %#v", detail)
	}
	personal := resources.NewMyExportRecordResource(&model.SysExportRecord{
		Status:        detail.Status,
		FileUUID:      detail.FileUUID,
		FileExpiresAt: *detail.FileExpiresAt,
	})
	if personal.FileUUID != "" || personal.DownloadReady {
		t.Fatalf("expired personal export must not expose a download identifier: %#v", personal)
	}
}

func TestExportCenterServiceCommitSuccessOnlyFirstFileWins(t *testing.T) {
	db := newExportCenterTestDB(t)
	svc := NewExportCenterServiceWithDeps(ExportCenterServiceDeps{DB: db})

	if _, err := svc.CreateRecord(context.Background(), ExportRecordCreateInput{
		Scene:     model.ExportSceneRequestLog,
		TaskRunID: 211,
		Queue:     "export",
		Format:    "csv",
	}); err != nil {
		t.Fatalf("create record failed: %v", err)
	}
	first := &model.UploadFiles{UUID: "winner", Size: 128}
	first.ID = 31
	second := &model.UploadFiles{UUID: "loser", Size: 256}
	second.ID = 32

	winner, committed, err := svc.CommitSuccess(context.Background(), 211, ExportRecordSuccessInput{File: first, FileName: "first.csv"})
	if err != nil || !committed {
		t.Fatalf("first commit failed: committed=%v record=%#v err=%v", committed, winner, err)
	}
	winner, committed, err = svc.CommitSuccess(context.Background(), 211, ExportRecordSuccessInput{File: second, FileName: "second.csv"})
	if err != nil || committed {
		t.Fatalf("second commit should reuse winner: committed=%v record=%#v err=%v", committed, winner, err)
	}
	if winner == nil || winner.FileID != first.ID || winner.FileUUID != first.UUID || winner.FileName != "first.csv" {
		t.Fatalf("unexpected winning export record: %#v", winner)
	}
}

func TestExportCenterServiceClaimExecutionPreventsConcurrentOwners(t *testing.T) {
	db := newExportCenterTestDB(t)
	svc := NewExportCenterServiceWithDeps(ExportCenterServiceDeps{DB: db})

	if _, err := svc.CreateRecord(context.Background(), ExportRecordCreateInput{
		Scene:     model.ExportSceneRequestLog,
		TaskRunID: 213,
		Queue:     "export",
		Format:    "csv",
	}); err != nil {
		t.Fatalf("create record failed: %v", err)
	}
	claimedRecord, claimed, err := svc.ClaimExecution(context.Background(), 213, "task-a")
	if err != nil || !claimed || claimedRecord == nil || claimedRecord.Status != model.TaskRunStatusRunning {
		t.Fatalf("first claim failed: claimed=%v record=%#v err=%v", claimed, claimedRecord, err)
	}
	claimedRecord, claimed, err = svc.ClaimExecution(context.Background(), 213, "task-b")
	if err != nil || claimed || claimedRecord == nil || claimedRecord.Status != model.TaskRunStatusRunning {
		t.Fatalf("concurrent claim should observe active owner: claimed=%v record=%#v err=%v", claimed, claimedRecord, err)
	}
	_, claimed, err = svc.ClaimExecution(context.Background(), 213, "task-a")
	if err != nil || !claimed {
		t.Fatalf("Asynq retry should take over running state: claimed=%v err=%v", claimed, err)
	}
	if err := svc.MarkPendingForRetry(context.Background(), 213, errors.New("temporary failure")); err != nil {
		t.Fatalf("release for retry failed: %v", err)
	}
	_, claimed, err = svc.ClaimExecution(context.Background(), 213, "task-b")
	if err != nil || !claimed {
		t.Fatalf("pending retry should be claimable: claimed=%v err=%v", claimed, err)
	}
}

func TestExportCenterServiceLateFailureAndProgressDoNotDowngradeSuccess(t *testing.T) {
	db := newExportCenterTestDB(t)
	svc := NewExportCenterServiceWithDeps(ExportCenterServiceDeps{DB: db})

	if _, err := svc.CreateRecord(context.Background(), ExportRecordCreateInput{
		Scene:     model.ExportSceneRequestLog,
		TaskRunID: 212,
		Queue:     "export",
		Format:    "csv",
	}); err != nil {
		t.Fatalf("create record failed: %v", err)
	}
	file := &model.UploadFiles{UUID: "winner", Size: 128}
	file.ID = 41
	if _, committed, err := svc.CommitSuccess(context.Background(), 212, ExportRecordSuccessInput{File: file, FileName: "winner.csv"}); err != nil || !committed {
		t.Fatalf("commit success failed: committed=%v err=%v", committed, err)
	}
	if err := svc.MarkFailed(context.Background(), 212, errors.New("late worker failure")); err != nil {
		t.Fatalf("late failure should be ignored: %v", err)
	}
	if err := svc.UpdateProgress(context.Background(), 212, 1, 10); !errors.Is(err, ErrExportRecordFinalized) {
		t.Fatalf("late progress should stop with finalized error, got %v", err)
	}

	var stored model.SysExportRecord
	if err := db.Where("task_run_id = ?", 212).First(&stored).Error; err != nil {
		t.Fatalf("query stored record failed: %v", err)
	}
	if stored.Status != model.TaskRunStatusSuccess || stored.Progress != exportProgressDone || stored.FileID != file.ID {
		t.Fatalf("completed export was downgraded: %#v", stored)
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

	userID := uint(3)
	list, err := svc.MyList(context.Background(), userID, &form.ExportRecordList{
		Paginate: form.Paginate{Page: 1, PerPage: 10},
		Scene:    model.ExportSceneFileList,
	})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
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

func TestExportCenterServiceReassignTaskRunKeepsSameRecord(t *testing.T) {
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
		t.Fatalf("reassign retry record failed: %v", err)
	}
	if !reassigned {
		t.Fatal("expected retry export task to reassign existing export record")
	}

	oldID := uint(101)
	listOld, err := svc.MyList(context.Background(), record.TriggerUserID, &form.ExportRecordList{
		Paginate:  form.Paginate{Page: 1, PerPage: 10},
		TaskRunID: &oldID,
	})
	if err != nil {
		t.Fatalf("list old task failed: %v", err)
	}
	if listOld.Total != 0 || len(listOld.Data) != 0 {
		t.Fatalf("expected old task_run_id 101 to have no export record after reassign, got %#v", listOld)
	}

	newID := uint(202)
	listNew, err := svc.MyList(context.Background(), record.TriggerUserID, &form.ExportRecordList{
		Paginate:  form.Paginate{Page: 1, PerPage: 10},
		TaskRunID: &newID,
	})
	if err != nil {
		t.Fatalf("list new task failed: %v", err)
	}
	if listNew.Total != 1 || len(listNew.Data) != 1 {
		t.Fatalf("expected one record on new task_run_id 202, got %#v", listNew)
	}
	item, ok := listNew.Data[0].(*resources.ExportRecordListResources)
	if !ok {
		t.Fatalf("unexpected list item type: %T", listNew.Data[0])
	}
	if item.ID != record.ID {
		t.Fatalf("expected retry to keep same record id %d, got %d", record.ID, item.ID)
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
			public_id text unique,
			scene text,
			task_run_id integer,
			trigger_user_id integer,
			status text,
			queue text,
			execution_token text,
			progress integer,
			total_count integer,
			processed_count integer,
			file_id integer,
			file_uuid text,
			file_name text,
			file_size integer,
			file_expires_at datetime,
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
