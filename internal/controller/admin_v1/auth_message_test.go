package admin_v1

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/global"
)

func TestSysMessageWSRequiresTicket(t *testing.T) {
	initControllerAuthTest(t)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/v1/system/notification/ws", nil)
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())
	ctx.Set(global.ContextKeyRequestID, "ws-missing-token")

	NewSysMessageController().WS(ctx)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected http status 401, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"code":401`) {
		t.Fatalf("expected not login response, got %s", recorder.Body.String())
	}
}

func TestSysMessageWSRejectsQueryTokenFallback(t *testing.T) {
	initControllerAuthTest(t)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/v1/system/notification/ws?access_token=legacy-token", nil)
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())
	ctx.Set(global.ContextKeyRequestID, "ws-query-token")

	NewSysMessageController().WS(ctx)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected http status 401, got %d", recorder.Code)
	}
}

func TestMaskWSSecret(t *testing.T) {
	if got := maskWSSecret("12345678"); got != "***" {
		t.Fatalf("expected short secret mask, got %q", got)
	}
	if got := maskWSSecret("1234567890abcdef"); got != "1234...cdef" {
		t.Fatalf("unexpected masked secret: %q", got)
	}
}
