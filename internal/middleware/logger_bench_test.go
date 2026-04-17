package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/pkg/response"
)

func BenchmarkCustomLoggerJSONPost(b *testing.B) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"name":"codex","email":"codex@example.com","password":"secret"}`)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/admin/v1/demo", bytes.NewReader(body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		ctx.Set(global.ContextKeyRequestStartTime, time.Now())
		ctx.Set(global.ContextKeyRequestID, "bench-request")

		cacheRequestBody(ctx)

		respRecorder := createResponseRecorder(ctx)
		respRecorder.Header().Set("Content-Type", "application/json")
		_, _ = respRecorder.Write([]byte(`{"code":0,"msg":"ok","data":{"id":1}}`))

		resp := parseResponse(ctx, respRecorder)
		if resp == nil {
			resp = &response.Result{Code: 0}
		}
		snapshot := buildRequestAuditLogSnapshot(ctx, respRecorder, resp)
		if snapshot == nil {
			b.Fatal("expected snapshot")
		}
	}
}
