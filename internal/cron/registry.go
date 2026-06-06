package taskcron

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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

	HandlerCronDemo              = "cron.demo"
	HandlerCronResetSystemData   = "cron.reset-system-data"
	HandlerCronFileAccessFlush   = "cron.file-access-flush"
	HandlerCronCleanExpiredFiles = "cron.clean-expired-files"
	HandlerCronCleanAuditLogs    = "cron.clean-audit-logs"
)

const cronLogTimeFormat = "2006-01-02 15:04:05"

type HandlerFunc func(ctx context.Context, payload map[string]any) error

var (
	handlersMu sync.RWMutex
	handlers   = map[string]HandlerFunc{
		HandlerCronDemo: func(_ context.Context, _ map[string]any) error {
			log.Logger.Info("计划任务 demo 执行：", zap.String("time", time.Now().Format(cronLogTimeFormat)))
			return nil
		},
		HandlerCronCleanExpiredFiles: CleanExpiredFilesHandler,
		HandlerCronCleanAuditLogs:    CleanAuditLogsHandler,
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
			Code:        jobs.AuditLogTaskType,
			Name:        "请求日志异步落库",
			Kind:        model.TaskKindAsync,
			Queue:       jobs.AuditQueueName,
			CronSpec:    "",
			Handler:     "jobs.audit_log.write",
			Status:      model.TaskStatusEnabled,
			AllowManual: model.TaskManualNotAllowed,
			AllowRetry:  model.TaskRetryAllowed,
			IsHighRisk:  model.TaskNotHighRisk,
			Remark:      "写入 request_logs 审计日志",
		},
		{
			Code:        jobs.TypeExportRequestLog,
			Name:        "请求日志导出",
			Kind:        model.TaskKindAsync,
			Queue:       jobs.ExportRequestLogQueue,
			CronSpec:    "",
			Handler:     "jobs.export_request_log",
			Status:      model.TaskStatusEnabled,
			AllowManual: model.TaskManualNotAllowed,
			AllowRetry:  model.TaskRetryAllowed,
			IsHighRisk:  model.TaskNotHighRisk,
			Remark:      "异步导出请求日志为 CSV 文件",
		},
		{
			Code:        jobs.TypeExportFileList,
			Name:        "文件资源导出",
			Kind:        model.TaskKindAsync,
			Queue:       jobs.ExportFileListQueue,
			CronSpec:    "",
			Handler:     "jobs.export_file_list",
			Status:      model.TaskStatusEnabled,
			AllowManual: model.TaskManualNotAllowed,
			AllowRetry:  model.TaskRetryAllowed,
			IsHighRisk:  model.TaskNotHighRisk,
			Remark:      "异步导出文件资源为 CSV 文件",
		},
		{
			Code:        TaskCodeCronDemo,
			Name:        "演示定时任务",
			Kind:        model.TaskKindCron,
			Queue:       "",
			CronSpec:    "0/5 * * * * *",
			Handler:     HandlerCronDemo,
			Status:      demoStatus,
			AllowManual: model.TaskManualAllowed,
			AllowRetry:  model.TaskRetryNotAllowed,
			IsHighRisk:  model.TaskNotHighRisk,
			Remark:      "开发演示任务",
		},
		{
			Code:     TaskCodeCronResetSystemData,
			Name:     "系统重建定时任务",
			Kind:     model.TaskKindCron,
			Queue:    "",
			CronSpec: "0 0 2 * * *",
			Handler:  HandlerCronResetSystemData,
			// 任务定义保留在任务中心可见；是否参与 cron 调度由 status 决定。
			Status:      resetStatus,
			AllowManual: model.TaskManualAllowed,
			AllowRetry:  model.TaskRetryNotAllowed,
			IsHighRisk:  model.TaskHighRisk,
			Remark:      "高风险任务，默认关闭",
		},
		{
			Code:        TaskCodeCronFileAccessFlush,
			Name:        "文件访问时间聚合落库",
			Kind:        model.TaskKindCron,
			Queue:       "",
			CronSpec:    "0 * * * * *",
			Handler:     HandlerCronFileAccessFlush,
			Status:      model.TaskStatusEnabled,
			AllowManual: model.TaskManualAllowed,
			AllowRetry:  model.TaskRetryNotAllowed,
			IsHighRisk:  model.TaskNotHighRisk,
			Remark:      "每分钟将 upload_files.last_accessed_at 从 Redis ZSet 批量落库",
		},
		{
			Code:        TaskCodeCronCleanExpiredFiles,
			Name:        "自动清理过期临时文件",
			Kind:        model.TaskKindCron,
			Queue:       "",
			CronSpec:    "0 0 3 * * *",
			Handler:     HandlerCronCleanExpiredFiles,
			Status:      model.TaskStatusEnabled,
			AllowManual: model.TaskManualAllowed,
			AllowRetry:  model.TaskRetryNotAllowed,
			IsHighRisk:  model.TaskNotHighRisk,
			Remark:      "自动物理清理系统内所有已过期的临时导出/上传文件及物理介质",
		},
		{
			Code:        TaskCodeCronCleanAuditLogs,
			Name:        "自动清理过期审计日志",
			Kind:        model.TaskKindCron,
			Queue:       "",
			CronSpec:    "0 0 4 * * *",
			Handler:     HandlerCronCleanAuditLogs,
			Status:      model.TaskStatusEnabled,
			AllowManual: model.TaskManualAllowed,
			AllowRetry:  model.TaskRetryNotAllowed,
			IsHighRisk:  model.TaskNotHighRisk,
			Remark:      "每天凌晨 4 点自动清理超过保留天数的请求日志和登录日志",
		},
	}
}

// SyncBuiltinDefinitionsIfAvailable 在任务定义表存在时同步内置任务定义。
func SyncBuiltinDefinitionsIfAvailable(cfg *config.Conf) error {
	db, err := model.GetDB()
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(model.NewTaskDefinition().TableName()) {
		return nil
	}
	return syncBuiltinDefinitions(db, cfg)
}

func syncBuiltinDefinitions(db *gorm.DB, cfg *config.Conf) error {
	if db == nil {
		return nil
	}

	for _, definition := range BuiltinTaskDefinitions(cfg) {
		if err := upsertTaskDefinition(db, definition); err != nil {
			return err
		}
	}
	return nil
}

func upsertTaskDefinition(db *gorm.DB, definition model.TaskDefinition) error {
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "code"},
			{Name: "deleted_at"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"name",
			"kind",
			"queue",
			"cron_spec",
			"handler",
			"status",
			"allow_manual",
			"allow_retry",
			"is_high_risk",
			"remark",
			"updated_at",
		}),
	}).Create(&definition).Error
}
