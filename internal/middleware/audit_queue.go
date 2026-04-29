package middleware

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/jobs"
	perrors "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/queue"
	audit "github.com/wannanbigpig/gin-layout/internal/service/audit"
)

const auditEnqueueTimeout = 2 * time.Second

var (
	enqueueAuditTaskFn = jobs.EnqueueAuditLog
	persistAuditLogFn  = audit.PersistAuditLog

	queueUnavailableLogged atomic.Bool
	storageUnavailable     atomic.Bool
)

type auditQueueDeps struct {
	Enqueue func(ctx context.Context, kind string, snapshot *audit.AuditLogSnapshot) error
	Persist func(snapshot *audit.AuditLogSnapshot) error
}

func setAuditQueueDepsForTesting(deps auditQueueDeps) func() {
	previousEnqueue := enqueueAuditTaskFn
	previousPersist := persistAuditLogFn
	previousQueueUnavailable := queueUnavailableLogged.Load()
	previousStorageUnavailable := storageUnavailable.Load()

	enqueueAuditTaskFn = deps.Enqueue
	if enqueueAuditTaskFn == nil {
		enqueueAuditTaskFn = jobs.EnqueueAuditLog
	}
	persistAuditLogFn = deps.Persist
	if persistAuditLogFn == nil {
		persistAuditLogFn = audit.PersistAuditLog
	}

	queueUnavailableLogged.Store(false)
	storageUnavailable.Store(false)

	return func() {
		enqueueAuditTaskFn = previousEnqueue
		persistAuditLogFn = previousPersist
		queueUnavailableLogged.Store(previousQueueUnavailable)
		storageUnavailable.Store(previousStorageUnavailable)
	}
}

func enqueueAuditLog(c *gin.Context, kind string, snapshot *audit.AuditLogSnapshot) {
	if snapshot == nil {
		return
	}

	cfg := config.GetConfig()
	if cfg == nil || !cfg.Queue.Enable {
		reportAuditPersistenceResult(kind, snapshot, "sync_direct")
		return
	}

	ctx := context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
	}
	ctx, cancel := context.WithTimeout(ctx, auditEnqueueTimeout)
	defer cancel()

	if err := enqueueAuditTaskFn(ctx, kind, snapshot); err != nil {
		if errors.Is(err, queue.ErrPublisherUnavailable) {
			if queueUnavailableLogged.CompareAndSwap(false, true) {
				log.Warn("Audit queue publisher unavailable, fallback to sync persist",
					zap.String("operation", "enqueue_audit_log"),
					zap.String("kind", kind),
					zap.String("request_id", snapshot.RequestID))
			}
		} else {
			log.Warn("Enqueue audit log failed, fallback to sync persist",
				zap.String("operation", "enqueue_audit_log"),
				zap.String("kind", kind),
				zap.String("request_id", snapshot.RequestID),
				zap.Error(err))
		}
		reportAuditPersistenceResult(kind, snapshot, "sync_fallback")
		return
	}

	// 队列恢复后复位告警开关，保证下次不可用时仍能打首条告警。
	queueUnavailableLogged.Store(false)
}

func reportAuditPersistenceResult(kind string, snapshot *audit.AuditLogSnapshot, mode string) {
	if snapshot == nil {
		return
	}

	if err := persistAuditLogFn(snapshot); err != nil {
		if perrors.IsDependencyNotReady(err) {
			if storageUnavailable.CompareAndSwap(false, true) {
				log.Warn("Audit log storage unavailable, skip persistence",
					zap.String("kind", kind),
					zap.String("mode", mode),
					zap.String("request_id", snapshot.RequestID),
					zap.Error(err))
			}
			return
		}
		log.Error("Persist audit log failed",
			zap.String("kind", kind),
			zap.String("mode", mode),
			zap.String("request_id", snapshot.RequestID),
			zap.Error(err))
		return
	}

	storageUnavailable.Store(false)
}
