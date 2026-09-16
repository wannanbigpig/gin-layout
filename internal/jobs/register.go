package jobs

import (
	"context"

	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/queue"
	auditsvc "github.com/wannanbigpig/gin-layout/internal/service/audit"
	"go.uber.org/zap"
)

// RegisterAll 注册当前版本的全部异步任务。
func RegisterAll(registry queue.Registry) {
	RegisterAllWithDeps(registry, AuditLogHandlerDeps{})
}

// NewRegistry 创建并注册当前版本的全部任务处理器。
func NewRegistry() queue.Registry {
	registry := queue.NewRegistry()
	RegisterAll(registry)
	return registry
}

// RegisterAllWithDeps 注册全部异步任务并支持依赖注入。
func RegisterAllWithDeps(registry queue.Registry, deps AuditLogHandlerDeps) {
	if registry == nil {
		return
	}
	registerAuditLogJob(registry, deps)
	queue.RegisterJSON(registry, TypeExportRequestLog, handleExportRequestLog)
	queue.RegisterJSON(registry, TypeExportFileList, handleExportFileList)
}

func registerAuditLogJob(registry queue.Registry, deps AuditLogHandlerDeps) {
	persistFn := deps.Persist
	// 生产默认走 auditsvc.PersistAuditLog，仅测试或特殊场景显式注入时才覆盖。
	if persistFn == nil {
		persistFn = auditsvc.PersistAuditLog
	}
	queue.RegisterJSON(registry, AuditLogTaskType, func(ctx context.Context, payload AuditLogPayload) error {
		_ = ctx
		if err := persistFn(payload.Snapshot); err != nil {
			log.Error("Persist audit log failed",
				zap.String("request_id", payload.Snapshot.RequestID),
				zap.Error(err))
			return err
		}
		return nil
	})
}
