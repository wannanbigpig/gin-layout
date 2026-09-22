package taskcron

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/jobs"
	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/service/sys_config"
	"go.uber.org/zap"
)

const (
	TaskCodeCronDemo              = "cron:demo"
	TaskCodeCronResetSystemData   = "cron:reset-system-data"
	TaskCodeCronFileAccessFlush   = "cron:file-access-flush"
	TaskCodeCronCleanExpiredFiles = "cron:clean-expired-files"
	TaskCodeCronCleanAuditLogs    = "cron:clean-audit-logs"
	TaskCodeCronTodoReminder      = "cron:todo-reminder"

	HandlerCronDemo              = "cron.demo"
	HandlerCronResetSystemData   = "cron.reset-system-data"
	HandlerCronFileAccessFlush   = "cron.file-access-flush"
	HandlerCronCleanExpiredFiles = "cron.clean-expired-files"
	HandlerCronCleanAuditLogs    = "cron.clean-audit-logs"
	HandlerCronTodoReminder      = "cron.todo-reminder"
)

const cronLogTimeFormat = "2006-01-02 15:04:05"

type HandlerFunc func(ctx context.Context, payload map[string]any) error

// TaskDefinitionSyncReport 描述一次内置任务定义同步结果。
type TaskDefinitionSyncReport struct {
	TableAvailable bool
	Total          int
	Inserted       int
	Updated        int
	Unchanged      int
}

var (
	handlersMu sync.RWMutex
	handlers   = map[string]HandlerFunc{
		HandlerCronDemo: func(_ context.Context, _ map[string]any) error {
			log.Logger.Info("计划任务 demo 执行：", zap.String("time", time.Now().Format(cronLogTimeFormat)))
			return nil
		},
		HandlerCronCleanExpiredFiles: CleanExpiredFilesHandler,
		HandlerCronCleanAuditLogs:    CleanAuditLogsHandler,
		HandlerCronTodoReminder:      TodoReminderHandler,
	}
)

func RegisterHandler(handler string, fn HandlerFunc) error {
	handler = strings.TrimSpace(handler)
	if handler == "" || fn == nil {
		return fmt.Errorf("handler name and function must not be empty")
	}
	handlersMu.Lock()
	defer handlersMu.Unlock()
	if _, exists := handlers[handler]; exists {
		return fmt.Errorf("duplicate cron handler registration: %s", handler)
	}
	handlers[handler] = fn
	return nil
}

func ExecuteHandler(ctx context.Context, handler string, payload map[string]any) error {
	handler = strings.TrimSpace(handler)
	handlersMu.RLock()
	fn, ok := handlers[handler]
	handlersMu.RUnlock()
	if !ok {
		return fmt.Errorf("unsupported cron handler: %s", handler)
	}
	return fn(ctx, payload)
}

// RegisterBuiltinHandlers 集中注册系统内置 cron handler。
// 调用方需保证此函数仅被调用一次（通常由 bootstrap 在数据源初始化完成后触发）。
func RegisterBuiltinHandlers() error {
	return RegisterHandler(HandlerCronFileAccessFlush, func(ctx context.Context, payload map[string]any) error {
		_ = payload
		return service.FlushFileAccessPending(ctx)
	})
}

// BuiltinTaskDefinitions 返回系统内置任务定义（任务中心列表与 cron 注册共用此定义源）。
func BuiltinTaskDefinitions(cfg *config.Conf) []model.TaskDefinition {
	// 高风险“系统重建”任务默认禁用，仅在配置显式开启时启用调度。
	resetStatus := model.TaskStatusDisabled
	if cfg != nil && cfg.EnableResetSystemCron {
		resetStatus = model.TaskStatusEnabled
	}
	demoStatus := model.TaskStatusEnabled
	if !sys_config.BoolValue(sys_config.TaskCronDemoEnabledConfigKey, false) {
		demoStatus = model.TaskStatusDisabled
	}

	return []model.TaskDefinition{
		{
			Code:                         jobs.AuditLogTaskType,
			Name:                         "请求日志异步落库",
			Kind:                         model.TaskKindAsync,
			Queue:                        jobs.AuditQueueName,
			CronSpec:                     "",
			Handler:                      "jobs.audit_log.write",
			Status:                       model.TaskStatusEnabled,
			AllowManual:                  model.TaskManualNotAllowed,
			AllowRetry:                   model.TaskRetryAllowed,
			IsHighRisk:                   model.TaskNotHighRisk,
			RecordSuccessMode:            model.TaskRecordSuccessNone,
			RecordSuccessRate:            0,
			RecordSuccessIntervalSeconds: 0,
			RecordDetailOnManual:         1,
			RecordDetailOnFailure:        1,
			Remark:                       "写入 request_logs 审计日志",
		},
		{
			Code:                         jobs.TypeExportRequestLog,
			Name:                         "请求日志导出",
			Kind:                         model.TaskKindAsync,
			Queue:                        jobs.ExportRequestLogQueue,
			CronSpec:                     "",
			Handler:                      "jobs.export_request_log",
			Status:                       model.TaskStatusEnabled,
			AllowManual:                  model.TaskManualNotAllowed,
			AllowRetry:                   model.TaskRetryAllowed,
			IsHighRisk:                   model.TaskNotHighRisk,
			RecordSuccessMode:            model.TaskRecordSuccessAll,
			RecordSuccessRate:            100,
			RecordSuccessIntervalSeconds: 0,
			RecordDetailOnManual:         1,
			RecordDetailOnFailure:        1,
			Remark:                       "异步导出请求日志为 CSV 文件",
		},
		{
			Code:                         jobs.TypeExportFileList,
			Name:                         "文件资源导出",
			Kind:                         model.TaskKindAsync,
			Queue:                        jobs.ExportFileListQueue,
			CronSpec:                     "",
			Handler:                      "jobs.export_file_list",
			Status:                       model.TaskStatusEnabled,
			AllowManual:                  model.TaskManualNotAllowed,
			AllowRetry:                   model.TaskRetryAllowed,
			IsHighRisk:                   model.TaskNotHighRisk,
			RecordSuccessMode:            model.TaskRecordSuccessAll,
			RecordSuccessRate:            100,
			RecordSuccessIntervalSeconds: 0,
			RecordDetailOnManual:         1,
			RecordDetailOnFailure:        1,
			Remark:                       "异步导出文件资源为 CSV 文件",
		},
		{
			Code:                         TaskCodeCronDemo,
			Name:                         "演示定时任务",
			Kind:                         model.TaskKindCron,
			Queue:                        "",
			CronSpec:                     "0/5 * * * * *",
			Handler:                      HandlerCronDemo,
			Status:                       demoStatus,
			AllowManual:                  model.TaskManualAllowed,
			AllowRetry:                   model.TaskRetryNotAllowed,
			IsHighRisk:                   model.TaskNotHighRisk,
			RecordSuccessMode:            model.TaskRecordSuccessInterval,
			RecordSuccessRate:            100,
			RecordSuccessIntervalSeconds: 300,
			RecordDetailOnManual:         1,
			RecordDetailOnFailure:        1,
			Remark:                       "开发演示任务",
		},
		{
			Code:     TaskCodeCronResetSystemData,
			Name:     "系统重建定时任务",
			Kind:     model.TaskKindCron,
			Queue:    "",
			CronSpec: "0 0 2 * * *",
			Handler:  HandlerCronResetSystemData,
			// 任务定义保留在任务中心可见；是否参与 cron 调度由 status 决定。
			Status:                       resetStatus,
			AllowManual:                  model.TaskManualAllowed,
			AllowRetry:                   model.TaskRetryNotAllowed,
			IsHighRisk:                   model.TaskHighRisk,
			RecordSuccessMode:            model.TaskRecordSuccessAll,
			RecordSuccessRate:            100,
			RecordSuccessIntervalSeconds: 0,
			RecordDetailOnManual:         1,
			RecordDetailOnFailure:        1,
			Remark:                       "高风险任务，默认关闭",
		},
		{
			Code:                         TaskCodeCronFileAccessFlush,
			Name:                         "文件访问时间聚合落库",
			Kind:                         model.TaskKindCron,
			Queue:                        "",
			CronSpec:                     "0 * * * * *",
			Handler:                      HandlerCronFileAccessFlush,
			Status:                       model.TaskStatusEnabled,
			AllowManual:                  model.TaskManualAllowed,
			AllowRetry:                   model.TaskRetryNotAllowed,
			IsHighRisk:                   model.TaskNotHighRisk,
			RecordSuccessMode:            model.TaskRecordSuccessInterval,
			RecordSuccessRate:            100,
			RecordSuccessIntervalSeconds: 900,
			RecordDetailOnManual:         1,
			RecordDetailOnFailure:        1,
			Remark:                       "每分钟将 upload_files.last_accessed_at 从 Redis ZSet 批量落库",
		},
		{
			Code:                         TaskCodeCronCleanExpiredFiles,
			Name:                         "自动清理过期临时文件",
			Kind:                         model.TaskKindCron,
			Queue:                        "",
			CronSpec:                     "0 0 3 * * *",
			Handler:                      HandlerCronCleanExpiredFiles,
			Status:                       model.TaskStatusEnabled,
			AllowManual:                  model.TaskManualAllowed,
			AllowRetry:                   model.TaskRetryNotAllowed,
			IsHighRisk:                   model.TaskNotHighRisk,
			RecordSuccessMode:            model.TaskRecordSuccessAll,
			RecordSuccessRate:            100,
			RecordSuccessIntervalSeconds: 0,
			RecordDetailOnManual:         1,
			RecordDetailOnFailure:        1,
			Remark:                       "自动物理清理系统内所有已过期的临时导出/上传文件及物理介质",
		},
		{
			Code:                         TaskCodeCronTodoReminder,
			Name:                         "待办到期提醒",
			Kind:                         model.TaskKindCron,
			Queue:                        "",
			CronSpec:                     "0 */5 * * * *",
			Handler:                      HandlerCronTodoReminder,
			Status:                       model.TaskStatusEnabled,
			AllowManual:                  model.TaskManualAllowed,
			AllowRetry:                   model.TaskRetryNotAllowed,
			IsHighRisk:                   model.TaskNotHighRisk,
			RecordSuccessMode:            model.TaskRecordSuccessInterval,
			RecordSuccessRate:            100,
			RecordSuccessIntervalSeconds: 3600,
			RecordDetailOnManual:         1,
			RecordDetailOnFailure:        1,
			Remark:                       "每 5 分钟扫描临期和逾期待办；站内消息与短信 Outbox 都使用业务唯一键防重，当前仍只支持单 cron 实例",
		},
		{
			Code:                         TaskCodeCronCleanAuditLogs,
			Name:                         "自动清理过期审计日志",
			Kind:                         model.TaskKindCron,
			Queue:                        "",
			CronSpec:                     "0 0 4 * * *",
			Handler:                      HandlerCronCleanAuditLogs,
			Status:                       model.TaskStatusEnabled,
			AllowManual:                  model.TaskManualAllowed,
			AllowRetry:                   model.TaskRetryNotAllowed,
			IsHighRisk:                   model.TaskNotHighRisk,
			RecordSuccessMode:            model.TaskRecordSuccessAll,
			RecordSuccessRate:            100,
			RecordSuccessIntervalSeconds: 0,
			RecordDetailOnManual:         1,
			RecordDetailOnFailure:        1,
			Remark:                       "每天凌晨 4 点自动清理超过保留天数的请求日志和登录日志",
		},
	}
}

// SyncBuiltinDefinitionsIfAvailable 在任务定义表存在时同步内置任务定义。
func SyncBuiltinDefinitionsIfAvailable(cfg *config.Conf) error {
	_, err := SyncBuiltinDefinitionsWithReportIfAvailable(cfg)
	return err
}

// SyncBuiltinDefinitionsWithReportIfAvailable 在任务定义表存在时同步内置任务定义并返回结果摘要。
func SyncBuiltinDefinitionsWithReportIfAvailable(cfg *config.Conf) (TaskDefinitionSyncReport, error) {
	db, err := model.GetDB()
	if err != nil {
		return TaskDefinitionSyncReport{}, err
	}
	if !db.Migrator().HasTable(model.NewTaskDefinition().TableName()) {
		return TaskDefinitionSyncReport{}, nil
	}
	report, err := syncBuiltinDefinitionsWithReport(db, cfg)
	report.TableAvailable = true
	return report, err
}

func syncBuiltinDefinitionsWithReport(db *gorm.DB, cfg *config.Conf) (TaskDefinitionSyncReport, error) {
	definitions := BuiltinTaskDefinitions(cfg)
	report := TaskDefinitionSyncReport{Total: len(definitions)}
	if db == nil {
		return report, nil
	}

	for _, definition := range definitions {
		result, err := syncOneBuiltinDefinition(db, definition)
		if err != nil {
			return report, err
		}
		switch result {
		case taskDefinitionSyncInserted:
			report.Inserted++
		case taskDefinitionSyncUpdated:
			report.Updated++
		case taskDefinitionSyncUnchanged:
			report.Unchanged++
		}
	}
	return report, nil
}

type taskDefinitionSyncResult string

const (
	taskDefinitionSyncInserted  taskDefinitionSyncResult = "inserted"
	taskDefinitionSyncUpdated   taskDefinitionSyncResult = "updated"
	taskDefinitionSyncUnchanged taskDefinitionSyncResult = "unchanged"
)

func syncOneBuiltinDefinition(db *gorm.DB, definition model.TaskDefinition) (taskDefinitionSyncResult, error) {
	var current model.TaskDefinition
	err := db.Where("code = ? AND deleted_at = 0", definition.Code).First(&current).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return taskDefinitionSyncInserted, db.Create(&definition).Error
	}
	if err != nil {
		return "", err
	}
	if taskDefinitionManagedFieldsEqual(current, definition) {
		return taskDefinitionSyncUnchanged, nil
	}

	updates := map[string]any{
		"name":         definition.Name,
		"kind":         definition.Kind,
		"queue":        definition.Queue,
		"handler":      definition.Handler,
		"is_high_risk": definition.IsHighRisk,
		"remark":       definition.Remark,
	}
	if err := db.Model(&current).Updates(updates).Error; err != nil {
		return "", err
	}
	return taskDefinitionSyncUpdated, nil
}

func taskDefinitionManagedFieldsEqual(current, next model.TaskDefinition) bool {
	return current.Name == next.Name &&
		current.Kind == next.Kind &&
		current.Queue == next.Queue &&
		current.Handler == next.Handler &&
		current.IsHighRisk == next.IsHighRisk &&
		current.Remark == next.Remark
}
