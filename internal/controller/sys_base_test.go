package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	pkgerrors "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func TestApiErrMapsDBUninitializedToDependencyNotReady(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/v1/demo", nil)
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())
	ctx.Set(global.ContextKeyRequestID, "test-request-id")

	Api{}.Err(ctx, model.ErrDBUninitialized)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected http status 503, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"code":10003`) {
		t.Fatalf("expected dependency not ready response, got %s", recorder.Body.String())
	}
}

func TestApiErrDoesNotExposeRawTechnicalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/v1/demo", nil)
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())

	Api{}.Err(ctx, errors.New("driver: bad connection password=should-not-leak"))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected http status 500, got %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"code":500`) {
		t.Fatalf("expected server error response, got %s", body)
	}
	if strings.Contains(body, "should-not-leak") || strings.Contains(body, "bad connection") {
		t.Fatalf("raw technical error leaked in response: %s", body)
	}
}

func TestApiErrDoesNotExposeBusinessErrorContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/v1/demo", nil)
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())

	businessErr := pkgerrors.NewBusinessError(pkgerrors.InvalidParameter, "可安全展示的参数错误")
	businessErr.AppendContextErr(errors.New("sql: secret=should-not-leak"))
	Api{}.Err(ctx, businessErr)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected http status 400, got %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "可安全展示的参数错误") {
		t.Fatalf("expected safe business message, got %s", body)
	}
	if strings.Contains(body, "should-not-leak") || strings.Contains(body, "sql:") {
		t.Fatalf("business context error leaked in response: %s", body)
	}
}
