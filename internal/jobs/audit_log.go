package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/wannanbigpig/gin-layout/config"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/queue"
	auditsvc "github.com/wannanbigpig/gin-layout/internal/service/audit"
	"go.uber.org/zap"
)

const (
	AuditLogTaskType = "audit:request_log.write"
	AuditQueueName   = "audit"

	AuditLogKindRequest = "request"
	AuditLogKindPanic   = "panic"
)

// AuditLogHandlerDeps 描述审计日志任务处理器可注入依赖。
type AuditLogHandlerDeps struct {
	Persist func(snapshot *auditsvc.AuditLogSnapshot) error
}

// AuditLogPayload 表示异步审计日志任务 payload。
type AuditLogPayload struct {
	Kind     string                     `json:"kind"`
	Snapshot *auditsvc.AuditLogSnapshot `json:"snapshot"`
}

// NewAuditLogPayload 创建审计日志 payload。
func NewAuditLogPayload(kind string, snapshot *auditsvc.AuditLogSnapshot) (AuditLogPayload, error) {
	payload := AuditLogPayload{
		Kind:     kind,
		Snapshot: snapshot,
	}
	if err := payload.Validate(); err != nil {
		return AuditLogPayload{}, err
	}
	return payload, nil
}

// Validate 校验 payload 是否满足最小要求。
func (p AuditLogPayload) Validate() error {
	if p.Kind != AuditLogKindRequest && p.Kind != AuditLogKindPanic {
		return fmt.Errorf("invalid audit log kind %q", p.Kind)
	}
	if p.Snapshot == nil || p.Snapshot.RequestID == "" {
		return fmt.Errorf("audit log snapshot is invalid")
	}
	return nil
}

// EnqueueAuditLog 发布异步审计日志任务。
func EnqueueAuditLog(ctx context.Context, kind string, snapshot *auditsvc.AuditLogSnapshot) error {
	payload, err := NewAuditLogPayload(kind, snapshot)
	if err != nil {
		return err
	}
	_, err = queue.PublishJSON(ctx, AuditLogTaskType, AuditQueueName, payload, auditLogOptions()...)
	return err
}

func auditLogOptions() []queue.JobOption {
	cfg := config.GetConfig()
	maxRetry := 3
	timeout := 10 * time.Second
	if cfg != nil {
		if cfg.Queue.AuditMaxRetry > 0 {
			maxRetry = cfg.Queue.AuditMaxRetry
		}
		if cfg.Queue.AuditTimeoutSeconds > 0 {
			timeout = time.Duration(cfg.Queue.AuditTimeoutSeconds) * time.Second
		}
	}
	return []queue.JobOption{
		queue.WithMaxRetry(maxRetry),
		queue.WithTimeout(timeout),
	}
}

// RegisterAll 注册当前版本的全部异步任务。
func RegisterAll(registry queue.Registry) {
	RegisterAllWithDeps(registry, AuditLogHandlerDeps{})
}

// RegisterAllWithDeps 注册全部异步任务并支持依赖注入。
func RegisterAllWithDeps(registry queue.Registry, deps AuditLogHandlerDeps) {
	if registry == nil {
		return
	}
	persistFn := deps.Persist
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
