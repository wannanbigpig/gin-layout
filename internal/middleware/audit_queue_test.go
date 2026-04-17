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
	original := enqueueAuditLogFunc
	defer func() {
		enqueueAuditLogFunc = original
	}()

	called := false
	enqueueAuditLogFunc = func(ctx context.Context, kind string, snapshot *auditsvc.AuditLogSnapshot) error {
		called = true
		if kind != "request" {
			t.Fatalf("unexpected kind: %s", kind)
		}
		if snapshot == nil || snapshot.RequestID != "req-1" {
			t.Fatalf("unexpected snapshot: %#v", snapshot)
		}
		return nil
	}

	originalQueueEnable := config.Config.Queue.Enable
	config.Config.Queue.Enable = true
	defer func() {
		config.Config.Queue.Enable = originalQueueEnable
	}()

	log.Logger = zap.NewNop()
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
		t.Fatal("expected enqueueAuditLogFunc to be called")
	}
}

func TestEnqueueAuditLogFailureDoesNotPanic(t *testing.T) {
	original := enqueueAuditLogFunc
	defer func() {
		enqueueAuditLogFunc = original
	}()

	enqueueAuditLogFunc = func(ctx context.Context, kind string, snapshot *auditsvc.AuditLogSnapshot) error {
		return errors.New("enqueue failed")
	}

	enqueueAuditLog(nil, "request", &auditsvc.AuditLogSnapshot{RequestID: "req-2"})
}

func TestEnqueueAuditLogResetsUnavailableFlagAfterSuccess(t *testing.T) {
	original := enqueueAuditLogFunc
	defer func() {
		enqueueAuditLogFunc = original
	}()

	enqueueAuditLogFunc = func(ctx context.Context, kind string, snapshot *auditsvc.AuditLogSnapshot) error {
		return nil
	}

	originalQueueEnable := config.Config.Queue.Enable
	config.Config.Queue.Enable = true
	defer func() {
		config.Config.Queue.Enable = originalQueueEnable
	}()

	auditQueueUnavailableLogged.Store(true)
	enqueueAuditLog(nil, "request", &auditsvc.AuditLogSnapshot{RequestID: "req-3"})

	if auditQueueUnavailableLogged.Load() {
		t.Fatal("expected unavailable flag to be reset after successful enqueue")
	}
}

func TestEnqueueAuditLogMarksUnavailableWhenPublisherUnavailable(t *testing.T) {
	original := enqueueAuditLogFunc
	defer func() {
		enqueueAuditLogFunc = original
	}()

	enqueueAuditLogFunc = func(ctx context.Context, kind string, snapshot *auditsvc.AuditLogSnapshot) error {
		return queue.ErrPublisherUnavailable
	}

	originalQueueEnable := config.Config.Queue.Enable
	config.Config.Queue.Enable = true
	defer func() {
		config.Config.Queue.Enable = originalQueueEnable
	}()

	auditQueueUnavailableLogged.Store(false)
	enqueueAuditLog(nil, "request", &auditsvc.AuditLogSnapshot{RequestID: "req-4"})

	if !auditQueueUnavailableLogged.Load() {
		t.Fatal("expected unavailable flag to be set when publisher unavailable")
	}
}

func TestEnqueueAuditLogUsesLocalAsyncWriterWhenQueueDisabled(t *testing.T) {
	originalLocalEnqueue := enqueueLocalAuditLogFunc
	defer func() {
		enqueueLocalAuditLogFunc = originalLocalEnqueue
	}()

	called := false
	enqueueLocalAuditLogFunc = func(kind string, snapshot *auditsvc.AuditLogSnapshot) {
		called = true
		if kind != "request" {
			t.Fatalf("unexpected kind: %s", kind)
		}
		if snapshot == nil || snapshot.RequestID != "req-db-unavailable" {
			t.Fatalf("unexpected snapshot: %#v", snapshot)
		}
	}

	originalQueueEnable := config.Config.Queue.Enable
	config.Config.Queue.Enable = false
	defer func() {
		config.Config.Queue.Enable = originalQueueEnable
	}()

	auditStorageUnavailableLogged.Store(false)
	enqueueAuditLog(nil, "request", &auditsvc.AuditLogSnapshot{RequestID: "req-db-unavailable"})

	if !called {
		t.Fatal("expected local async enqueue to be used when queue is disabled")
	}
}

func TestReportAuditPersistenceResultMarksStorageUnavailable(t *testing.T) {
	originalPersist := persistAuditLogFunc
	defer func() {
		persistAuditLogFunc = originalPersist
	}()

	persistAuditLogFunc = func(snapshot *auditsvc.AuditLogSnapshot) error {
		return model.ErrDBUninitialized
	}

	auditStorageUnavailableLogged.Store(false)
	reportAuditPersistenceResult("request", &auditsvc.AuditLogSnapshot{RequestID: "req-db-unavailable"}, "local_async")

	if !auditStorageUnavailableLogged.Load() {
		t.Fatal("expected storage unavailable flag to be set when db is unavailable")
	}
}

func TestReportAuditPersistenceResultResetsStorageUnavailableAfterSuccess(t *testing.T) {
	originalPersist := persistAuditLogFunc
	defer func() {
		persistAuditLogFunc = originalPersist
	}()

	persistAuditLogFunc = func(snapshot *auditsvc.AuditLogSnapshot) error {
		return nil
	}

	auditStorageUnavailableLogged.Store(true)
	reportAuditPersistenceResult("request", &auditsvc.AuditLogSnapshot{RequestID: "req-db-ok"}, "local_async")

	if auditStorageUnavailableLogged.Load() {
		t.Fatal("expected storage unavailable flag to be reset after successful persistence")
	}
}
