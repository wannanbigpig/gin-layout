package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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
)

var ErrExportRecordTableMissing = errors.New("sys_export_records table is missing")

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

func (s *ExportCenterService) MarkRunning(ctx context.Context, taskRunID uint) error {
	if taskRunID == 0 {
		return nil
	}
	now := utils.FormatDate{Time: time.Now()}
	return s.updateByTaskRunID(ctx, taskRunID, map[string]any{
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
	return s.updateByTaskRunID(ctx, taskRunID, map[string]any{
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
	return s.updateByTaskRunID(ctx, taskRunID, map[string]any{
		"progress":        progress,
		"total_count":     normalizedTotal,
		"processed_count": normalizedProcessed,
	})
}

func (s *ExportCenterService) MarkSuccess(ctx context.Context, taskRunID uint, input ExportRecordSuccessInput) error {
	if taskRunID == 0 {
		return nil
	}
	if input.File == nil || input.File.ID == 0 {
		return fmt.Errorf("export file is required")
	}

	now := utils.FormatDate{Time: time.Now()}
	fileName := strings.TrimSpace(input.FileName)
	if fileName == "" {
		fileName = resolveExportFileName(input.File)
	}

	return s.updateByTaskRunID(ctx, taskRunID, map[string]any{
		"status":          model.TaskRunStatusSuccess,
		"progress":        exportProgressDone,
		"file_id":         input.File.ID,
		"file_uuid":       input.File.UUID,
		"file_name":       fileName,
		"file_size":       uint64(input.File.Size),
		"error_message":   "",
		"finished_at":     &now,
		"processed_count": gorm.Expr("CASE WHEN total_count > 0 THEN total_count ELSE processed_count END"),
	})
}

func (s *ExportCenterService) MarkFailed(ctx context.Context, taskRunID uint, taskErr error) error {
	if taskRunID == 0 {
		return nil
	}
	now := utils.FormatDate{Time: time.Now()}
	updates := map[string]any{
		"status":      model.TaskRunStatusFailed,
		"finished_at": &now,
	}
	if taskErr != nil {
		updates["error_message"] = strings.TrimSpace(taskErr.Error())
	}
	return s.updateByTaskRunID(ctx, taskRunID, updates)
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
		"status":      model.TaskRunStatusCanceled,
		"finished_at": &now,
	}
	if strings.TrimSpace(cancelReason) != "" {
		updates["error_message"] = strings.TrimSpace(cancelReason)
	}
	return db.Model(&model.SysExportRecord{}).Where("task_run_id = ?", taskRunID).Updates(updates).Error
}

func (s *ExportCenterService) List(params *form.ExportRecordList) *resources.Collection {
	query := query_builder.New().
		AddEq("scene", strings.TrimSpace(params.Scene)).
		AddEq("status", strings.TrimSpace(params.Status)).
		AddEq("task_run_id", params.TaskRunID).
		AddEq("trigger_user_id", params.TriggerUserID)
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

	transformer := resources.NewExportRecordTransformer()
	if s.db != nil {
		if err := ensureExportRecordTable(s.db); err != nil {
			return transformer.ToCollection(params.Page, params.PerPage, 0, nil)
		}
	}
	if s.db != nil {
		var total int64
		db := s.db.Model(&model.SysExportRecord{})
		if condition != "" {
			db = db.Where(condition, args...)
		}
		if err := db.Count(&total).Error; err != nil || total == 0 {
			return transformer.ToCollection(params.Page, params.PerPage, total, nil)
		}

		rows := make([]*model.SysExportRecord, 0, params.PerPage)
		queryDB := s.db.Model(&model.SysExportRecord{}).Select(listOptionalParams.SelectFields).Order(listOptionalParams.OrderBy)
		if condition != "" {
			queryDB = queryDB.Where(condition, args...)
		}
		if err := queryDB.Scopes(recordModel.Paginate(params.Page, params.PerPage)).Find(&rows).Error; err != nil {
			return transformer.ToCollection(params.Page, params.PerPage, 0, nil)
		}
		return transformer.ToCollection(params.Page, params.PerPage, total, rows)
	}

	if db, err := model.GetDB(); err != nil || !db.Migrator().HasTable(&model.SysExportRecord{}) {
		return transformer.ToCollection(params.Page, params.PerPage, 0, nil)
	}

	recordModel.SetDB(s.db)
	total, collection, err := model.ListPageE(recordModel, params.Page, params.PerPage, condition, args, listOptionalParams)
	if err != nil {
		return transformer.ToCollection(params.Page, params.PerPage, 0, nil)
	}
	return transformer.ToCollection(params.Page, params.PerPage, total, collection)
}

func (s *ExportCenterService) Detail(id uint) (any, error) {
	record := model.NewSysExportRecord()
	if s.db != nil {
		if err := ensureExportRecordTable(s.db); err != nil {
			return nil, e.NewBusinessError(e.NotFound)
		}
		record.SetDB(s.db)
	} else {
		db, err := model.GetDB()
		if err != nil {
			return nil, err
		}
		if err := ensureExportRecordTable(db); err != nil {
			return nil, e.NewBusinessError(e.NotFound)
		}
	}
	if err := record.GetById(id); err != nil || record.ID == 0 {
		return nil, e.NewBusinessError(e.NotFound)
	}
	return resources.NewExportRecordTransformer().ToStruct(record), nil
}

// ReassignTaskRun 将 oldTaskRunID 对应的导出记录改挂到 newTaskRunID 并重置为 pending 待执行态。
// 返回 reassigned=true 表示已复用旧记录；返回 reassigned=false 表示原 task_run 没有对应导出记录，
// 调用方应回退到 CreateRecordForTask（兼容历史无 export_record 的老 run）。
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
	var record model.SysExportRecord
	if err := db.Where("task_run_id = ?", oldTaskRunID).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	updates := map[string]any{
		"task_run_id":     newTaskRunID,
		"status":          model.TaskRunStatusPending,
		"progress":        exportProgressPending,
		"total_count":     uint64(0),
		"processed_count": uint64(0),
		"started_at":      nil,
		"finished_at":     nil,
		"file_id":         0,
		"file_uuid":       "",
		"file_name":       "",
		"file_size":       uint64(0),
		"error_message":   "",
	}
	if trimmed := strings.TrimSpace(queueName); trimmed != "" {
		updates["queue"] = trimmed
	}
	if err := db.Model(&model.SysExportRecord{}).Where("id = ?", record.ID).Updates(updates).Error; err != nil {
		return false, err
	}
	return true, nil
}

func (s *ExportCenterService) updateByTaskRunID(ctx context.Context, taskRunID uint, updates map[string]any) error {
	db, err := s.dbOrDefault(ctx)
	if err != nil {
		return err
	}
	if err := ensureExportRecordTable(db); err != nil {
		return err
	}
	result := db.Model(&model.SysExportRecord{}).Where("task_run_id = ?", taskRunID).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("export record for task run %d not found", taskRunID)
	}
	return nil
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

func ensureExportRecordTable(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&model.SysExportRecord{}) {
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
