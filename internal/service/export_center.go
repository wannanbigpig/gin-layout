package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/query_builder"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

const (
	exportTaskCodeRequestLog = "export:request-log"
	exportTaskCodeFileList   = "export:file-list"

	exportProgressPending uint8 = 0
	exportProgressDone    uint8 = 100

	exportManualRetryLimit = 3
)

var (
	ErrExportRecordTableMissing = errors.New("sys_export_records table is missing")
	ErrExportRecordFinalized    = errors.New("export record is already finalized")
)

// ExportCenterService 统一导出中心服务。
type ExportCenterService struct {
	db *gorm.DB
}

type ExportCenterServiceDeps struct {
	DB *gorm.DB
}

type ExportRecordCreateInput struct {
	Scene         string
	TaskRunID     uint
	TriggerUserID uint
	Queue         string
	Format        string
}

type ExportRecordSuccessInput struct {
	File     *model.UploadFiles
	FileName string
}

type ExportTaskBootstrapInput struct {
	TaskCode      string
	TaskRunID     uint
	TriggerUserID uint
	Queue         string
}

func NewExportCenterService() *ExportCenterService {
	return NewExportCenterServiceWithDeps(ExportCenterServiceDeps{})
}

func NewExportCenterServiceWithDeps(deps ExportCenterServiceDeps) *ExportCenterService {
	return &ExportCenterService{db: deps.DB}
}

func (s *ExportCenterService) CreateRecordForTask(ctx context.Context, input ExportTaskBootstrapInput) (*model.SysExportRecord, bool, error) {
	scene, format, ok := resolveExportScene(strings.TrimSpace(input.TaskCode))
	if !ok {
		return nil, false, nil
	}
	record, err := s.CreateRecord(ctx, ExportRecordCreateInput{
		Scene:         scene,
		TaskRunID:     input.TaskRunID,
		TriggerUserID: input.TriggerUserID,
		Queue:         input.Queue,
		Format:        format,
	})
	if err != nil {
		return nil, true, err
	}
	return record, true, nil
}

func (s *ExportCenterService) CreateRecord(ctx context.Context, input ExportRecordCreateInput) (*model.SysExportRecord, error) {
	if strings.TrimSpace(input.Scene) == "" {
		return nil, fmt.Errorf("export scene is required")
	}
	if input.TaskRunID == 0 {
		return nil, fmt.Errorf("task run id is required")
	}
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return nil, err
	}
	if err := ensureExportRecordTable(db); err != nil {
		return nil, err
	}

	record := model.NewSysExportRecord()
	record.PublicID = uuid.NewString()
	record.Scene = strings.TrimSpace(input.Scene)
	record.TaskRunID = input.TaskRunID
	record.TriggerUserID = input.TriggerUserID
	record.Status = model.TaskRunStatusPending
	record.Queue = strings.TrimSpace(input.Queue)
	record.Progress = exportProgressPending
	record.Format = strings.TrimSpace(input.Format)

	if err := db.Create(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

// ClaimExecution 原子占有一次导出执行。正常并发只能有一个 pending 记录进入 running；
// 相同 executionToken 的 Asynq 后续重试可接管自身遗留的 running 状态。
func (s *ExportCenterService) ClaimExecution(ctx context.Context, taskRunID uint, executionToken string) (*model.SysExportRecord, bool, error) {
	if taskRunID == 0 {
		return nil, false, nil
	}
	executionToken = strings.TrimSpace(executionToken)
	if executionToken == "" {
		return nil, false, fmt.Errorf("export execution token is required")
	}
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return nil, false, err
	}
	if err := ensureExportRecordTable(db); err != nil {
		return nil, false, err
	}

	now := utils.FormatDate{Time: time.Now()}
	query := db.Model(&model.SysExportRecord{}).
		Where("task_run_id = ?", taskRunID).
		Where("status IN ? OR (status = ? AND execution_token = ?)",
			[]string{model.TaskRunStatusPending, model.TaskRunStatusRetrying},
			model.TaskRunStatusRunning,
			executionToken)
	result := query.Updates(map[string]any{
		"status":          model.TaskRunStatusRunning,
		"execution_token": executionToken,
		"progress":        exportProgressPending,
		"processed_count": uint64(0),
		"started_at":      &now,
		"finished_at":     nil,
		"error_message":   "",
	})
	if result.Error != nil {
		return nil, false, result.Error
	}
	record := model.NewSysExportRecord()
	if err := db.Where("task_run_id = ?", taskRunID).First(record).Error; err != nil {
		return nil, false, err
	}
	claimed := result.RowsAffected > 0
	// MySQL DATETIME 只有秒精度；重试在同一秒接管 running 时各字段可能未变化，
	// RowsAffected 会是 0。Asynq 不会在上一尝试仍运行时启动其重试，因此该分支可视为已接管。
	if !claimed && record.Status == model.TaskRunStatusRunning && record.ExecutionToken == executionToken {
		claimed = true
	}
	return record, claimed, nil
}

// MarkPendingForRetry 释放本次执行占有，让 Asynq 的下一次自动重试重新领取。
func (s *ExportCenterService) MarkPendingForRetry(ctx context.Context, taskRunID uint, taskErr error) error {
	if taskRunID == 0 {
		return nil
	}
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return err
	}
	if err := ensureExportRecordTable(db); err != nil {
		return err
	}
	updates := map[string]any{
		"status":          model.TaskRunStatusPending,
		"execution_token": "",
		"finished_at":     nil,
	}
	if taskErr != nil {
		updates["error_message"] = strings.TrimSpace(taskErr.Error())
	}
	result := db.Model(&model.SysExportRecord{}).
		Where("task_run_id = ? AND status = ?", taskRunID, model.TaskRunStatusRunning).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}
	var record model.SysExportRecord
	if err := db.Select("status").Where("task_run_id = ?", taskRunID).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("export record for task run %d not found", taskRunID)
		}
		return err
	}
	if record.Status != model.TaskRunStatusSuccess && record.Status != model.TaskRunStatusCanceled {
		return nil
	}
	return fmt.Errorf("%w: task run %d", ErrExportRecordFinalized, taskRunID)
}

func (s *ExportCenterService) MarkRunning(ctx context.Context, taskRunID uint) error {
	if taskRunID == 0 {
		return nil
	}
	now := utils.FormatDate{Time: time.Now()}
	return s.updateActiveByTaskRunID(ctx, taskRunID, map[string]any{
		"status":          model.TaskRunStatusRunning,
		"progress":        exportProgressPending,
		"processed_count": uint64(0),
		"started_at":      &now,
		"error_message":   "",
	})
}

func (s *ExportCenterService) MarkRunningWithTotal(ctx context.Context, taskRunID uint, totalCount uint64) error {
	if taskRunID == 0 {
		return nil
	}
	now := utils.FormatDate{Time: time.Now()}
	_, normalizedTotal, progress := normalizeExportProgressCounts(0, totalCount)
	return s.updateActiveByTaskRunID(ctx, taskRunID, map[string]any{
		"status":          model.TaskRunStatusRunning,
		"progress":        progress,
		"total_count":     normalizedTotal,
		"processed_count": uint64(0),
		"started_at":      &now,
		"error_message":   "",
	})
}

func (s *ExportCenterService) UpdateProgress(ctx context.Context, taskRunID uint, processedCount, totalCount uint64) error {
	if taskRunID == 0 {
		return nil
	}
	normalizedProcessed, normalizedTotal, progress := normalizeExportProgressCounts(processedCount, totalCount)
	return s.updateActiveByTaskRunID(ctx, taskRunID, map[string]any{
		"progress":        progress,
		"total_count":     normalizedTotal,
		"processed_count": normalizedProcessed,
	})
}

func (s *ExportCenterService) MarkSuccess(ctx context.Context, taskRunID uint, input ExportRecordSuccessInput) error {
	_, _, err := s.CommitSuccess(ctx, taskRunID, input)
	return err
}

// CommitSuccess 以条件更新提交导出结果。并发 worker 只有第一个提交者获胜，
// 后续提交复用已完成记录，调用方可据此清理自己生成但未被采用的文件。
func (s *ExportCenterService) CommitSuccess(ctx context.Context, taskRunID uint, input ExportRecordSuccessInput) (*model.SysExportRecord, bool, error) {
	if taskRunID == 0 {
		return nil, false, nil
	}
	if input.File == nil || input.File.ID == 0 {
		return nil, false, fmt.Errorf("export file is required")
	}
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return nil, false, err
	}
	if err := ensureExportRecordTable(db); err != nil {
		return nil, false, err
	}

	now := utils.FormatDate{Time: time.Now()}
	fileName := strings.TrimSpace(input.FileName)
	if fileName == "" {
		fileName = resolveExportFileName(input.File)
	}

	updates := map[string]any{
		"status":          model.TaskRunStatusSuccess,
		"execution_token": "",
		"progress":        exportProgressDone,
		"file_id":         input.File.ID,
		"file_uuid":       input.File.UUID,
		"file_name":       fileName,
		"file_size":       uint64(input.File.Size),
		"error_message":   "",
		"finished_at":     &now,
		"processed_count": gorm.Expr("CASE WHEN total_count > 0 THEN total_count ELSE processed_count END"),
	}
	if exportRecordFileExpirySupported(db) {
		updates["file_expires_at"] = input.File.ExpiresAt
	}
	result := db.Model(&model.SysExportRecord{}).
		Where("task_run_id = ? AND status NOT IN ?", taskRunID, []string{model.TaskRunStatusSuccess, model.TaskRunStatusCanceled}).
		Updates(updates)
	if result.Error != nil {
		return nil, false, result.Error
	}
	record := model.NewSysExportRecord()
	if err := db.Where("task_run_id = ?", taskRunID).First(record).Error; err != nil {
		return nil, false, err
	}
	if result.RowsAffected > 0 {
		return record, true, nil
	}
	if record.Status == model.TaskRunStatusSuccess {
		return record, false, nil
	}
	return record, false, fmt.Errorf("%w: task run %d status is %s", ErrExportRecordFinalized, taskRunID, record.Status)
}

func (s *ExportCenterService) MarkFailed(ctx context.Context, taskRunID uint, taskErr error) error {
	if taskRunID == 0 {
		return nil
	}
	now := utils.FormatDate{Time: time.Now()}
	updates := map[string]any{
		"status":          model.TaskRunStatusFailed,
		"execution_token": "",
		"finished_at":     &now,
	}
	if taskErr != nil {
		updates["error_message"] = strings.TrimSpace(taskErr.Error())
	}
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return err
	}
	if err := ensureExportRecordTable(db); err != nil {
		return err
	}
	result := db.Model(&model.SysExportRecord{}).
		Where("task_run_id = ? AND status NOT IN ?", taskRunID, []string{model.TaskRunStatusSuccess, model.TaskRunStatusCanceled}).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}
	var count int64
	if err := db.Model(&model.SysExportRecord{}).Where("task_run_id = ?", taskRunID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("export record for task run %d not found", taskRunID)
	}
	return nil
}

func (s *ExportCenterService) MarkCanceledIfExists(ctx context.Context, taskRunID uint, cancelReason string) error {
	if taskRunID == 0 {
		return nil
	}
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return err
	}
	if err := ensureExportRecordTable(db); err != nil {
		if errors.Is(err, ErrExportRecordTableMissing) {
			return nil
		}
		return err
	}
	now := utils.FormatDate{Time: time.Now()}
	updates := map[string]any{
		"status":          model.TaskRunStatusCanceled,
		"execution_token": "",
		"finished_at":     &now,
	}
	if strings.TrimSpace(cancelReason) != "" {
		updates["error_message"] = strings.TrimSpace(cancelReason)
	}
	return db.Model(&model.SysExportRecord{}).
		Where("task_run_id = ? AND status <> ?", taskRunID, model.TaskRunStatusSuccess).
		Updates(updates).Error
}

// HasRecordForTaskRun 判断指定 task_run_id 是否仍关联导出记录。
func (s *ExportCenterService) HasRecordForTaskRun(ctx context.Context, taskRunID uint) (bool, error) {
	if taskRunID == 0 {
		return false, nil
	}
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return false, err
	}
	if err := ensureExportRecordTable(db); err != nil {
		return false, err
	}
	var count int64
	if err := db.Model(&model.SysExportRecord{}).Where("task_run_id = ?", taskRunID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// CompletedRecordForTaskRun 返回已完成的导出记录，用于 worker 至少一次重投时复用结果。
func (s *ExportCenterService) CompletedRecordForTaskRun(ctx context.Context, taskRunID uint) (*model.SysExportRecord, error) {
	if taskRunID == 0 {
		return nil, nil
	}
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return nil, err
	}
	if err := ensureExportRecordTable(db); err != nil {
		return nil, err
	}
	record := model.NewSysExportRecord()
	err = db.Where("task_run_id = ? AND status = ?", taskRunID, model.TaskRunStatusSuccess).First(record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

// ReassignTaskRun 将已有导出记录改绑到新的 retry run，并重置为待执行状态。
func (s *ExportCenterService) ReassignTaskRun(ctx context.Context, oldTaskRunID, newTaskRunID uint, queueName string) (bool, error) {
	if oldTaskRunID == 0 || newTaskRunID == 0 {
		return false, nil
	}
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return false, err
	}
	if err := ensureExportRecordTable(db); err != nil {
		return false, err
	}

	updates := map[string]any{
		"task_run_id":     newTaskRunID,
		"status":          model.TaskRunStatusPending,
		"execution_token": "",
		"queue":           strings.TrimSpace(queueName),
		"progress":        exportProgressPending,
		"total_count":     uint64(0),
		"processed_count": uint64(0),
		"file_id":         uint(0),
		"file_uuid":       "",
		"file_name":       "",
		"file_size":       uint64(0),
		"error_message":   "",
		"started_at":      nil,
		"finished_at":     nil,
	}
	if exportRecordFileExpirySupported(db) {
		updates["file_expires_at"] = nil
	}
	result := db.Model(&model.SysExportRecord{}).Where("task_run_id = ?", oldTaskRunID).Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// listInternal 内部底层全量查询，对外统一使用强制属主限定的 MyList。
func (s *ExportCenterService) listInternal(params *form.ExportRecordList) (*resources.Collection, error) {
	if params == nil || params.TriggerUserID == nil || *params.TriggerUserID == 0 {
		return nil, e.NewBusinessError(e.InvalidParameter)
	}
	query := query_builder.New().
		AddEq("scene", strings.TrimSpace(params.Scene)).
		AddEq("status", strings.TrimSpace(params.Status)).
		AddEq("task_run_id", params.TaskRunID).
		AddEq("trigger_user_id", *params.TriggerUserID)
	condition, args := query.Build()

	recordModel := model.NewSysExportRecord()
	listOptionalParams := model.ListOptionalParams{
		SelectFields: []string{
			"id",
			"scene",
			"task_run_id",
			"trigger_user_id",
			"status",
			"queue",
			"progress",
			"total_count",
			"processed_count",
			"file_id",
			"file_uuid",
			"file_name",
			"file_size",
			"format",
			"error_message",
			"started_at",
			"finished_at",
			"created_at",
		},
		OrderBy: "created_at DESC, id DESC",
	}
	if exportRecordFileExpirySupported(s.db) {
		listOptionalParams.SelectFields = append(listOptionalParams.SelectFields, "file_expires_at")
	}

	transformer := resources.NewExportRecordTransformer()
	if s.db != nil {
		if err := ensureExportRecordTable(s.db); err != nil {
			return nil, err
		}
	}
	if s.db != nil {
		var total int64
		db := s.db.Model(&model.SysExportRecord{})
		if condition != "" {
			db = db.Where(condition, args...)
		}
		if err := db.Count(&total).Error; err != nil {
			return nil, err
		}
		if total == 0 {
			return transformer.ToCollection(params.Page, params.PerPage, total, nil), nil
		}

		rows := make([]*model.SysExportRecord, 0, params.PerPage)
		queryDB := s.db.Model(&model.SysExportRecord{}).Select(listOptionalParams.SelectFields).Order(listOptionalParams.OrderBy)
		if condition != "" {
			queryDB = queryDB.Where(condition, args...)
		}
		if err := queryDB.Scopes(recordModel.Paginate(params.Page, params.PerPage)).Find(&rows).Error; err != nil {
			return nil, err
		}
		s.decorateExportRetryState(context.Background(), s.db, rows)
		return transformer.ToCollection(params.Page, params.PerPage, total, rows), nil
	}

	db, err := model.GetDB()
	if err != nil {
		return nil, err
	}
	if !exportRecordsTableReady(db) {
		return nil, ErrExportRecordTableMissing
	}
	if exportRecordFileExpirySupported(db) {
		listOptionalParams.SelectFields = append(listOptionalParams.SelectFields, "file_expires_at")
	}

	recordModel.SetDB(db)
	total, collection, err := model.ListPageE(recordModel, params.Page, params.PerPage, condition, args, listOptionalParams)
	if err != nil {
		return nil, err
	}
	s.decorateExportRetryState(context.Background(), db, collection)
	return transformer.ToCollection(params.Page, params.PerPage, total, collection), nil
}

// MyList 在查询层强制限定当前用户，避免调用方通过请求参数覆盖数据范围。
func (s *ExportCenterService) MyList(ctx context.Context, userID uint, params *form.ExportRecordList) (*resources.Collection, error) {
	if userID == 0 {
		return nil, e.NewBusinessError(e.NotFound)
	}
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return nil, err
	}
	if err := ensureExportRecordTable(db); err != nil {
		return nil, err
	}
	query := db.Model(&model.SysExportRecord{}).Where("trigger_user_id = ?", userID)
	if scene := strings.TrimSpace(params.Scene); scene != "" {
		query = query.Where("scene = ?", scene)
	}
	if status := strings.TrimSpace(params.Status); status != "" {
		query = query.Where("status = ?", status)
	}
	if publicID := strings.TrimSpace(params.PublicID); publicID != "" {
		query = query.Where("public_id = ?", publicID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	if total == 0 {
		return resources.NewMyExportRecordCollection(params.Page, params.PerPage, 0, nil), nil
	}
	fields := []string{"public_id", "scene", "status", "progress", "total_count", "processed_count", "file_uuid", "file_name", "file_size", "file_expires_at", "format", "created_at", "finished_at"}
	rows := make([]*model.SysExportRecord, 0, params.PerPage)
	if err := query.Select(fields).Order("created_at DESC, id DESC").Scopes(model.NewSysExportRecord().Paginate(params.Page, params.PerPage)).Find(&rows).Error; err != nil {
		return nil, err
	}
	return resources.NewMyExportRecordCollection(params.Page, params.PerPage, total, rows), nil
}

func (s *ExportCenterService) MyDetail(ctx context.Context, userID uint, publicID string) (*resources.MyExportRecordResources, error) {
	if userID == 0 || strings.TrimSpace(publicID) == "" {
		return nil, e.NewBusinessError(e.NotFound)
	}
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return nil, err
	}
	if err := ensureExportRecordTable(db); err != nil {
		return nil, err
	}
	var record model.SysExportRecord
	if err := db.Where("public_id = ? AND trigger_user_id = ?", strings.TrimSpace(publicID), userID).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, e.NewBusinessError(e.NotFound)
		}
		return nil, err
	}
	return resources.NewMyExportRecordResource(&record), nil
}

// exportRecordFileExpirySupported 兼容开发期已执行过旧版初始迁移的数据库；
// 新安装会由初始迁移直接创建该字段，旧开发库在重置前仍可读取导出列表。
func exportRecordFileExpirySupported(db *gorm.DB) bool {
	return db != nil && db.Migrator().HasColumn(&model.SysExportRecord{}, "file_expires_at")
}

func (s *ExportCenterService) Detail(id uint) (any, error) {
	record := model.NewSysExportRecord()
	var db *gorm.DB
	if s.db != nil {
		if err := ensureExportRecordTable(s.db); err != nil {
			return nil, e.NewDependencyNotReadyError()
		}
		db = s.db
		record.SetDB(s.db)
	} else {
		var err error
		db, err = model.GetDB()
		if err != nil {
			return nil, err
		}
		if err := ensureExportRecordTable(db); err != nil {
			return nil, e.NewDependencyNotReadyError()
		}
	}
	if err := record.GetById(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, e.NewBusinessError(e.NotFound)
		}
		return nil, err
	}
	if record.ID == 0 {
		return nil, e.NewBusinessError(e.NotFound)
	}
	s.decorateExportRetryState(context.Background(), db, []*model.SysExportRecord{record})
	return resources.NewExportRecordTransformer().ToStruct(record), nil
}

func (s *ExportCenterService) decorateExportRetryState(ctx context.Context, db *gorm.DB, records []*model.SysExportRecord) {
	if len(records) == 0 || db == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	taskRunIDs := make([]uint, 0, len(records))
	taskRunSeen := make(map[uint]struct{}, len(records))
	for _, record := range records {
		if record == nil {
			continue
		}
		record.CanRetry = false
		if record.TaskRunID == 0 {
			record.RetryBlockReason = "导出记录未关联执行记录"
			continue
		}
		if record.Status != model.TaskRunStatusFailed {
			record.RetryBlockReason = "仅失败导出任务允许重试"
			continue
		}
		if _, ok := taskRunSeen[record.TaskRunID]; ok {
			continue
		}
		taskRunSeen[record.TaskRunID] = struct{}{}
		taskRunIDs = append(taskRunIDs, record.TaskRunID)
	}
	if len(taskRunIDs) == 0 {
		return
	}

	var runs []*model.TaskRun
	if err := db.WithContext(ctx).
		Select("id", "task_code", "status", "max_retry", "retry_root_run_id").
		Where("id IN ?", taskRunIDs).
		Find(&runs).Error; err != nil {
		for _, record := range records {
			if record != nil && record.Status == model.TaskRunStatusFailed {
				record.RetryBlockReason = "读取执行记录失败"
			}
		}
		return
	}
	runMap := make(map[uint]*model.TaskRun, len(runs))
	taskCodes := make([]string, 0, len(runs))
	taskCodeSeen := make(map[string]struct{}, len(runs))
	rootIDs := make([]uint, 0, len(runs))
	rootSeen := make(map[uint]struct{}, len(runs))
	for _, run := range runs {
		if run == nil {
			continue
		}
		runMap[run.ID] = run
		if strings.TrimSpace(run.TaskCode) != "" {
			if _, ok := taskCodeSeen[run.TaskCode]; !ok {
				taskCodeSeen[run.TaskCode] = struct{}{}
				taskCodes = append(taskCodes, run.TaskCode)
			}
		}
		rootID := run.RetryRootRunID
		if rootID == 0 {
			rootID = run.ID
		}
		if _, ok := rootSeen[rootID]; !ok {
			rootSeen[rootID] = struct{}{}
			rootIDs = append(rootIDs, rootID)
		}
	}

	definitions := make(map[string]*model.TaskDefinition, len(taskCodes))
	if len(taskCodes) > 0 {
		var rows []*model.TaskDefinition
		if err := db.WithContext(ctx).Where("code IN ? AND deleted_at = 0", taskCodes).Find(&rows).Error; err == nil {
			for _, item := range rows {
				definitions[item.Code] = item
			}
		}
	}
	summaries := make(map[uint]exportRetryChainSummary, len(rootIDs))
	if len(rootIDs) > 0 {
		type row struct {
			RetryRootRunID uint
			MaxSeq         int
			SuccessCount   int64
			ActiveCount    int64
		}
		rows := make([]row, 0, len(rootIDs))
		if err := db.WithContext(ctx).
			Model(&model.TaskRun{}).
			Select(
				`retry_root_run_id,
				COALESCE(MAX(retry_seq), 0) AS max_seq,
				COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS success_count,
				COALESCE(SUM(CASE WHEN status IN (?, ?, ?) THEN 1 ELSE 0 END), 0) AS active_count`,
				model.TaskRunStatusSuccess,
				model.TaskRunStatusPending,
				model.TaskRunStatusRunning,
				model.TaskRunStatusRetrying,
			).
			Where("retry_root_run_id IN ?", rootIDs).
			Group("retry_root_run_id").
			Scan(&rows).Error; err == nil {
			for _, item := range rows {
				summaries[item.RetryRootRunID] = exportRetryChainSummary{
					MaxSeq:       item.MaxSeq,
					SuccessCount: item.SuccessCount,
					ActiveCount:  item.ActiveCount,
				}
			}
		}
	}

	for _, record := range records {
		if record == nil || record.Status != model.TaskRunStatusFailed || record.TaskRunID == 0 {
			continue
		}
		run := runMap[record.TaskRunID]
		if run == nil {
			record.RetryBlockReason = "执行记录不存在"
			continue
		}
		rootID := run.RetryRootRunID
		if rootID == 0 {
			rootID = run.ID
		}
		canRetry, reason := evaluateExportRetryState(run, definitions[run.TaskCode], summaries[rootID])
		record.CanRetry = canRetry
		record.RetryBlockReason = reason
	}
}

type exportRetryChainSummary struct {
	MaxSeq       int
	SuccessCount int64
	ActiveCount  int64
}

func evaluateExportRetryState(run *model.TaskRun, definition *model.TaskDefinition, summary exportRetryChainSummary) (bool, string) {
	if run == nil || run.ID == 0 {
		return false, "执行记录不存在"
	}
	switch run.Status {
	case model.TaskRunStatusFailed, model.TaskRunStatusTimeout, model.TaskRunStatusInterrupted:
	default:
		return false, "仅失败、超时或中断任务允许重试"
	}
	if definition == nil {
		return false, "任务定义不存在"
	}
	if definition.Status != model.TaskStatusEnabled {
		return false, "任务已停用，无法重试"
	}
	if definition.AllowRetry != model.TaskRetryAllowed {
		return false, "任务不允许重试"
	}
	if summary.ActiveCount > 0 {
		return false, "该任务已有重试记录正在执行或排队"
	}
	if summary.SuccessCount > 0 {
		return false, "该任务重试链路已成功，不允许继续重试"
	}
	if summary.MaxSeq >= exportManualRetryLimit {
		return false, fmt.Sprintf("已达到最大手动重试次数 %d", exportManualRetryLimit)
	}
	return true, ""
}

func (s *ExportCenterService) updateActiveByTaskRunID(ctx context.Context, taskRunID uint, updates map[string]any) error {
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return err
	}
	if err := ensureExportRecordTable(db); err != nil {
		return err
	}
	result := db.Model(&model.SysExportRecord{}).
		Where("task_run_id = ? AND status NOT IN ?", taskRunID, []string{model.TaskRunStatusSuccess, model.TaskRunStatusCanceled}).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}
	var record model.SysExportRecord
	if err := db.Select("status").Where("task_run_id = ?", taskRunID).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("export record for task run %d not found", taskRunID)
		}
		return err
	}
	if record.Status != model.TaskRunStatusSuccess && record.Status != model.TaskRunStatusCanceled {
		return nil
	}
	return fmt.Errorf("%w: task run %d", ErrExportRecordFinalized, taskRunID)
}

func normalizeExportProgressCounts(processedCount, totalCount uint64) (uint64, uint64, uint8) {
	if totalCount == 0 {
		return 0, 0, exportProgressPending
	}
	if processedCount > totalCount {
		processedCount = totalCount
	}
	progress := uint8((processedCount * 100) / totalCount)
	if processedCount == totalCount && progress >= exportProgressDone {
		progress = exportProgressDone - 1
	}
	return processedCount, totalCount, progress
}

// sys_export_records 表存在性探测使用 sync.Once + atomic.Bool 缓存。
//
// 背景：原实现中 ensureExportRecordTable 在每次 CreateRecord / UpdateProgress /
// MarkSuccess / MarkFailed / List / Detail 等
// 路径都会调用 db.Migrator().HasTable(&model.SysExportRecord{})。在 MySQL 下，
// 该调用会真实下发一次 INFORMATION_SCHEMA 查询，纯属重复劳动，因为应用启动后
// 该表是否存在不会再发生变化（不会被在线 drop）。
//
// 缓存策略：
//   - hasExportRecordsTable: 当前进程缓存的就绪状态，原子读。
//   - exportRecordsTableProbe: 保证整个进程内只真正探测一次。
//   - 首次探测时若 db 为 nil 或 HasTable 返回 false，将缓存 false；
//     后续传入有效 db 时不再回探（保持简单与一致）。
//
// 单元测试可调用 ResetExportRecordsTableCacheForTest 重置缓存以隔离用例。
var (
	hasExportRecordsTable   atomic.Bool
	exportRecordsTableProbe sync.Once
)

// exportRecordsTableReady 返回 sys_export_records 表是否就绪。
// 首次调用会执行真实 HasTable 探测，之后仅做一次 atomic load，开销可忽略。
func exportRecordsTableReady(db *gorm.DB) bool {
	exportRecordsTableProbe.Do(func() {
		if db == nil {
			hasExportRecordsTable.Store(false)
			return
		}
		hasExportRecordsTable.Store(db.Migrator().HasTable(&model.SysExportRecord{}))
	})
	return hasExportRecordsTable.Load()
}

// ResetExportRecordsTableCacheForTest 重置探测缓存，仅供测试代码调用。
func ResetExportRecordsTableCacheForTest() {
	hasExportRecordsTable.Store(false)
	exportRecordsTableProbe = sync.Once{}
}

func ensureExportRecordTable(db *gorm.DB) error {
	if !exportRecordsTableReady(db) {
		return ErrExportRecordTableMissing
	}
	return nil
}

func resolveExportScene(taskCode string) (scene string, format string, ok bool) {
	switch taskCode {
	case exportTaskCodeRequestLog:
		return model.ExportSceneRequestLog, "csv", true
	case exportTaskCodeFileList:
		return model.ExportSceneFileList, "csv", true
	default:
		return "", "", false
	}
}

// resolveExportSceneName 返回导出场景的中文名称，便于前端直接展示。
func resolveExportSceneName(scene string) string {
	switch strings.TrimSpace(scene) {
	case model.ExportSceneRequestLog:
		return "请求日志"
	case model.ExportSceneFileList:
		return "文件资源"
	default:
		return ""
	}
}

func (s *ExportCenterService) dbOrDefault(ctx context.Context) (*gorm.DB, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if s.db != nil {
		return s.db.WithContext(ctx), nil
	}
	db, err := model.GetDB()
	if err != nil {
		return nil, err
	}
	return db.WithContext(ctx), nil
}

func resolveExportFileName(file *model.UploadFiles) string {
	if file == nil {
		return ""
	}
	for _, candidate := range []string{file.DisplayName, file.OriginName, file.Name} {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
