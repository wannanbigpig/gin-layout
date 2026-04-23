package middleware

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/queue"
	auditsvc "github.com/wannanbigpig/gin-layout/internal/service/audit"
)

func TestEnqueueAuditLogDelegatesToPublisher(t *testing.T) {
	called := false
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		Enqueue: func(ctx context.Context, kind string, snapshot *auditsvc.AuditLogSnapshot) error {
			called = true
			if kind != "request" {
				t.Fatalf("unexpected kind: %s", kind)
			}
			if snapshot == nil || snapshot.RequestID != "req-1" {
				t.Fatalf("unexpected snapshot: %#v", snapshot)
			}
			return nil
		},
	})
	defer restoreDeps()

	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Queue.Enable = true
	})
	defer restoreConfig()

	restoreLogger := log.ReplaceLoggerForTesting(zap.NewNop())
	defer restoreLogger()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/demo", bytes.NewBufferString(`{"name":"codex"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set(global.ContextKeyRequestID, "req-1")
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())
	cacheRequestBody(ctx)

	respRecorder := createResponseRecorder(ctx)
	respRecorder.body.WriteString(`{"code":0,"msg":"ok","data":{}}`)

	logRequest(ctx, respRecorder)
	if !called {
		t.Fatal("expected enqueue to be called")
	}
}

func TestEnqueueAuditLogFailureDoesNotPanic(t *testing.T) {
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		Enqueue: func(ctx context.Context, kind string, snapshot *auditsvc.AuditLogSnapshot) error {
			return errors.New("enqueue failed")
		},
	})
	defer restoreDeps()

	enqueueAuditLog(nil, "request", &auditsvc.AuditLogSnapshot{RequestID: "req-2"})
}

func TestEnqueueAuditLogResetsUnavailableFlagAfterSuccess(t *testing.T) {
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		Enqueue: func(ctx context.Context, kind string, snapshot *auditsvc.AuditLogSnapshot) error {
			return nil
		},
	})
	defer restoreDeps()

	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Queue.Enable = true
	})
	defer restoreConfig()

	queueUnavailableLogged.Store(true)
	enqueueAuditLog(nil, "request", &auditsvc.AuditLogSnapshot{RequestID: "req-3"})

	if queueUnavailableLogged.Load() {
		t.Fatal("expected unavailable flag to be reset after successful enqueue")
	}
}

func TestEnqueueAuditLogMarksUnavailableWhenPublisherUnavailable(t *testing.T) {
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		Enqueue: func(ctx context.Context, kind string, snapshot *auditsvc.AuditLogSnapshot) error {
			return queue.ErrPublisherUnavailable
		},
	})
	defer restoreDeps()

	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Queue.Enable = true
	})
	defer restoreConfig()

	queueUnavailableLogged.Store(false)
	enqueueAuditLog(nil, "request", &auditsvc.AuditLogSnapshot{RequestID: "req-4"})

	if !queueUnavailableLogged.Load() {
		t.Fatal("expected unavailable flag to be set when publisher unavailable")
	}
}

func TestEnqueueAuditLogPersistsSynchronouslyWhenQueueDisabled(t *testing.T) {
	persistCalled := false
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		Enqueue: func(ctx context.Context, kind string, snapshot *auditsvc.AuditLogSnapshot) error {
			t.Fatal("enqueue should not be called when queue is disabled")
			return nil
		},
		Persist: func(snapshot *auditsvc.AuditLogSnapshot) error {
			persistCalled = true
			if snapshot == nil || snapshot.RequestID != "req-db-unavailable" {
				t.Fatalf("unexpected snapshot: %#v", snapshot)
			}
			return nil
		},
	})
	defer restoreDeps()

	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Queue.Enable = false
	})
	defer restoreConfig()

	storageUnavailable.Store(false)
	enqueueAuditLog(nil, "request", &auditsvc.AuditLogSnapshot{RequestID: "req-db-unavailable"})

	if !persistCalled {
		t.Fatal("expected synchronous persistence when queue is disabled")
	}
}

func TestEnqueueAuditLogHandlesStorageUnavailableWhenQueueDisabled(t *testing.T) {
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		Persist: func(snapshot *auditsvc.AuditLogSnapshot) error {
			return model.ErrDBUninitialized
		},
	})
	defer restoreDeps()

	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Queue.Enable = false
	})
	defer restoreConfig()

	storageUnavailable.Store(false)
	enqueueAuditLog(nil, "request", &auditsvc.AuditLogSnapshot{RequestID: "req-db-unavailable"})

	if !storageUnavailable.Load() {
		t.Fatal("expected storage unavailable flag to be set when sync persistence fails")
	}
}

func TestReportAuditPersistenceResultMarksStorageUnavailable(t *testing.T) {
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		Persist: func(snapshot *auditsvc.AuditLogSnapshot) error {
			return model.ErrDBUninitialized
		},
	})
	defer restoreDeps()

	storageUnavailable.Store(false)
	reportAuditPersistenceResult("request", &auditsvc.AuditLogSnapshot{RequestID: "req-db-unavailable"}, "sync_direct")

	if !storageUnavailable.Load() {
		t.Fatal("expected storage unavailable flag to be set when db is unavailable")
	}
}

func TestReportAuditPersistenceResultResetsStorageUnavailableAfterSuccess(t *testing.T) {
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		Persist: func(snapshot *auditsvc.AuditLogSnapshot) error {
			return nil
		},
	})
	defer restoreDeps()

	storageUnavailable.Store(true)
	reportAuditPersistenceResult("request", &auditsvc.AuditLogSnapshot{RequestID: "req-db-ok"}, "sync_direct")

	if storageUnavailable.Load() {
		t.Fatal("expected storage unavailable flag to be reset after successful persistence")
	}
}
