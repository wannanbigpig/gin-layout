package middleware

import (
	"context"
	"errors"
	"sync"
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

var enqueueAuditLogFunc = jobs.EnqueueAuditLog
var enqueueLocalAuditLogFunc = enqueueLocalAuditLog
var persistAuditLogFunc = audit.PersistAuditLog
var auditQueueUnavailableLogged atomic.Bool
var auditStorageUnavailableLogged atomic.Bool
var auditLocalBufferFullLogged atomic.Bool
var auditLocalWriterOnce sync.Once
var auditLocalWriterChan chan auditPersistTask

const (
	auditEnqueueTimeout  = 2 * time.Second
	auditLocalBufferSize = 256
)

type auditPersistTask struct {
	kind     string
	snapshot *audit.AuditLogSnapshot
}

func enqueueAuditLog(c *gin.Context, kind string, snapshot *audit.AuditLogSnapshot) {
	if snapshot == nil {
		return
	}

	cfg := config.GetConfig()
	if cfg == nil || !cfg.Queue.Enable {
		enqueueLocalAuditLogFunc(kind, snapshot)
		return
	}

	ctx := context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
	}
	ctx, cancel := context.WithTimeout(ctx, auditEnqueueTimeout)
	defer cancel()

	if err := enqueueAuditLogFunc(ctx, kind, snapshot); err != nil {
		if errors.Is(err, queue.ErrPublisherUnavailable) {
			if auditQueueUnavailableLogged.CompareAndSwap(false, true) {
				log.Warn("Audit queue publisher unavailable, skip enqueue",
					zap.String("operation", "enqueue_audit_log"),
					zap.String("kind", kind),
					zap.String("request_id", snapshot.RequestID))
			}
			return
		}
		log.Warn("Enqueue audit log failed",
			zap.String("operation", "enqueue_audit_log"),
			zap.String("kind", kind),
			zap.String("request_id", snapshot.RequestID),
			zap.Error(err))
		return
	}

	// 队列恢复后复位告警开关，保证下次不可用时仍能打首条告警。
	auditQueueUnavailableLogged.Store(false)
}

func enqueueLocalAuditLog(kind string, snapshot *audit.AuditLogSnapshot) {
	auditLocalWriterOnce.Do(func() {
		auditLocalWriterChan = make(chan auditPersistTask, auditLocalBufferSize)
		go runAuditLocalWriter(auditLocalWriterChan)
	})

	task := auditPersistTask{kind: kind, snapshot: snapshot}
	select {
	case auditLocalWriterChan <- task:
		auditLocalBufferFullLogged.Store(false)
	default:
		if auditLocalBufferFullLogged.CompareAndSwap(false, true) {
			log.Warn("Audit log local buffer is full, drop persistence",
				zap.String("kind", kind),
				zap.String("request_id", snapshot.RequestID))
		}
	}
}

func runAuditLocalWriter(tasks <-chan auditPersistTask) {
	for task := range tasks {
		reportAuditPersistenceResult(task.kind, task.snapshot, "local_async")
	}
}

func reportAuditPersistenceResult(kind string, snapshot *audit.AuditLogSnapshot, mode string) {
	if err := persistAuditLogFunc(snapshot); err != nil {
		if perrors.IsDependencyNotReady(err) {
			if auditStorageUnavailableLogged.CompareAndSwap(false, true) {
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

	auditStorageUnavailableLogged.Store(false)
}
