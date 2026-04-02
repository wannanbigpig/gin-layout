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
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
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
