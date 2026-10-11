package admin_v1

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wannanbigpig/gin-layout/internal/filestorage"
	"github.com/wannanbigpig/gin-layout/internal/global"
	pkgErrors "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/service"
)

func TestFileResourceController_MultipartInitLocalDriverUnsupported(t *testing.T) {
	initControllerAuthTest(t)
	previous := newFileMultipartInitService
	newFileMultipartInitService = func() *service.FileResourceService {
		return service.NewFileResourceServiceWithDeps(service.FileResourceServiceDeps{
			ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
				cfg := filestorage.DefaultConfig()
				return filestorage.NewLocalDriver(cfg.Local, t.TempDir(), t.TempDir()), cfg, "local", nil
			},
		})
	}
	t.Cleanup(func() { newFileMultipartInitService = previous })
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	// 构造分片上传初始化的参数
	body := map[string]any{
		"hash":        "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"origin_name": "test.txt",
		"size":        1024,
	}

	jsonBytes, _ := json.Marshal(body)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/admin/v1/system/file/upload/multipart/init", bytes.NewBuffer(jsonBytes))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())
	ctx.Set(global.ContextKeyRequestID, "test-multipart-init-request-id")

	// 调用控制器
	FileResourceController{}.MultipartInit(ctx)

	// 业务规则失败（本地驱动不支持分片）走 InferHTTPStatus fallback → 400。
	// 前端 `SILENT_BUSINESS_CODES = [11011]` 会静默该 code 的提示，但 HTTP 状态
	// 仍要落到 4xx，确保网关/审计/Network 能看见这次失败。
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected http status 400, got %d", recorder.Code)
	}

	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// 期望是 LocalStorageMultipartUnsupported (11011) 业务错误码
	if resp.Code != pkgErrors.LocalStorageMultipartUnsupported {
		t.Fatalf("expected code %d, got %d. Response: %s", pkgErrors.LocalStorageMultipartUnsupported, resp.Code, recorder.Body.String())
	}
}
