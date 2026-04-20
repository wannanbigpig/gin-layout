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

const (
	auditEnqueueTimeout  = 2 * time.Second
	auditLocalBufferSize = 256
)

type auditPersistTask struct {
	kind     string
	snapshot *audit.AuditLogSnapshot
}

type auditQueueDispatcherDeps struct {
	Enqueue      func(ctx context.Context, kind string, snapshot *audit.AuditLogSnapshot) error
	EnqueueLocal func(kind string, snapshot *audit.AuditLogSnapshot)
	Persist      func(snapshot *audit.AuditLogSnapshot) error
}

type auditQueueDispatcher struct {
	enqueueFn      func(ctx context.Context, kind string, snapshot *audit.AuditLogSnapshot) error
	enqueueLocalFn func(kind string, snapshot *audit.AuditLogSnapshot)
	persistFn      func(snapshot *audit.AuditLogSnapshot) error

	queueUnavailableLogged atomic.Bool
	storageUnavailable     atomic.Bool
	localBufferFullLogged  atomic.Bool
	localWriterOnce        sync.Once
	localWriterChan        chan auditPersistTask
}

func newAuditQueueDispatcher(deps auditQueueDispatcherDeps) *auditQueueDispatcher {
	d := &auditQueueDispatcher{
		enqueueFn:      deps.Enqueue,
		enqueueLocalFn: deps.EnqueueLocal,
		persistFn:      deps.Persist,
	}
	d.ensureRuntimeDeps()
	return d
}

func (d *auditQueueDispatcher) ensureRuntimeDeps() {
	if d.enqueueFn == nil {
		d.enqueueFn = jobs.EnqueueAuditLog
	}
	if d.persistFn == nil {
		d.persistFn = audit.PersistAuditLog
	}
	if d.enqueueLocalFn == nil {
		d.enqueueLocalFn = d.enqueueLocal
	}
}

var (
	defaultAuditQueueDispatcherOnce sync.Once
	defaultAuditQueueDispatcherVal  atomic.Pointer[auditQueueDispatcher]
)

func currentAuditQueueDispatcher() *auditQueueDispatcher {
	defaultAuditQueueDispatcherOnce.Do(func() {
		defaultAuditQueueDispatcherVal.Store(newAuditQueueDispatcher(auditQueueDispatcherDeps{}))
	})

	dispatcher := defaultAuditQueueDispatcherVal.Load()
	if dispatcher != nil {
		return dispatcher
	}

	// 防御性兜底：确保任何情况下都能返回可用 dispatcher。
	dispatcher = newAuditQueueDispatcher(auditQueueDispatcherDeps{})
	defaultAuditQueueDispatcherVal.Store(dispatcher)
	return dispatcher
}

func replaceDefaultAuditQueueDispatcherForTesting(dispatcher *auditQueueDispatcher) func() {
	if dispatcher == nil {
		dispatcher = newAuditQueueDispatcher(auditQueueDispatcherDeps{})
	}
	previous := currentAuditQueueDispatcher()
	defaultAuditQueueDispatcherVal.Store(dispatcher)
	return func() {
		defaultAuditQueueDispatcherVal.Store(previous)
	}
}

func enqueueAuditLog(c *gin.Context, kind string, snapshot *audit.AuditLogSnapshot) {
	currentAuditQueueDispatcher().enqueue(c, kind, snapshot)
}

func (d *auditQueueDispatcher) enqueue(c *gin.Context, kind string, snapshot *audit.AuditLogSnapshot) {
	d.ensureRuntimeDeps()
	if snapshot == nil {
		return
	}

	cfg := config.GetConfig()
	if cfg == nil || !cfg.Queue.Enable {
		d.enqueueLocalFn(kind, snapshot)
		return
	}

	ctx := context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
	}
	ctx, cancel := context.WithTimeout(ctx, auditEnqueueTimeout)
	defer cancel()

	if err := d.enqueueFn(ctx, kind, snapshot); err != nil {
		if errors.Is(err, queue.ErrPublisherUnavailable) {
			if d.queueUnavailableLogged.CompareAndSwap(false, true) {
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
	d.queueUnavailableLogged.Store(false)
}

func (d *auditQueueDispatcher) enqueueLocal(kind string, snapshot *audit.AuditLogSnapshot) {
	d.localWriterOnce.Do(func() {
		d.localWriterChan = make(chan auditPersistTask, auditLocalBufferSize)
		go d.runLocalWriter(d.localWriterChan)
	})

	task := auditPersistTask{kind: kind, snapshot: snapshot}
	select {
	case d.localWriterChan <- task:
		d.localBufferFullLogged.Store(false)
	default:
		if d.localBufferFullLogged.CompareAndSwap(false, true) {
			log.Warn("Audit log local buffer is full, drop persistence",
				zap.String("kind", kind),
				zap.String("request_id", snapshot.RequestID))
		}
	}
}

func (d *auditQueueDispatcher) runLocalWriter(tasks <-chan auditPersistTask) {
	for task := range tasks {
		d.reportPersistenceResult(task.kind, task.snapshot, "local_async")
	}
}

func reportAuditPersistenceResult(kind string, snapshot *audit.AuditLogSnapshot, mode string) {
	currentAuditQueueDispatcher().reportPersistenceResult(kind, snapshot, mode)
}

func (d *auditQueueDispatcher) reportPersistenceResult(kind string, snapshot *audit.AuditLogSnapshot, mode string) {
	d.ensureRuntimeDeps()
	if snapshot == nil {
		return
	}

	if err := d.persistFn(snapshot); err != nil {
		if perrors.IsDependencyNotReady(err) {
			if d.storageUnavailable.CompareAndSwap(false, true) {
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

	d.storageUnavailable.Store(false)
}
