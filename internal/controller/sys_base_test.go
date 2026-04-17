package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
)

func TestApiErrMapsDBUninitializedToDependencyNotReady(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/v1/demo", nil)
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())
	ctx.Set(global.ContextKeyRequestID, "test-request-id")

	Api{}.Err(ctx, model.ErrDBUninitialized)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected http status 200, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"code":10003`) {
		t.Fatalf("expected dependency not ready response, got %s", recorder.Body.String())
	}
}
