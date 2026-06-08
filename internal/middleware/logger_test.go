package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/jobs"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/response"
	auditsvc "github.com/wannanbigpig/gin-layout/internal/service/audit"
	"github.com/wannanbigpig/gin-layout/internal/service/auth"
)

// TestCacheRequestBody 验证请求体缓存逻辑。
func TestCacheRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/demo", bytes.NewBufferString(`{"name":"codex"}`))

	cacheRequestBody(ctx)
	body := readRequestBody(ctx)
	if string(body) != `{"name":"codex"}` {
		t.Fatalf("unexpected cached body: %s", string(body))
	}
}

// TestCacheRequestBodySkipsGet 验证 GET 请求不会缓存请求体。
func TestCacheRequestBodySkipsGet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/demo", nil)

	cacheRequestBody(ctx)
	if body := readRequestBody(ctx); body != nil {
		t.Fatalf("expected nil body for get request, got %q", string(body))
	}
}

func TestCacheRequestBodySkipsMultipartRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/upload", bytes.NewBufferString("file-content"))
	ctx.Request.Header.Set("Content-Type", "multipart/form-data; boundary=demo")

	cacheRequestBody(ctx)
	if body := readRequestBody(ctx); body != nil {
		t.Fatalf("expected multipart body to be skipped, got %q", string(body))
	}
}

// TestParseResponse 验证 JSON 响应解析逻辑。
func TestParseResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/demo", nil)

	respRecorder := createResponseRecorder(ctx)
	respRecorder.Header().Set("Content-Type", "application/json")
	respRecorder.body.WriteString(`{"code":0,"msg":"ok","data":{"name":"demo"}}`)

	resp := parseResponse(ctx, respRecorder)
	if resp == nil {
		t.Fatal("expected parsed response")
	}
	if resp.Code != 0 {
		t.Fatalf("expected code 0, got %d", resp.Code)
	}
}

// TestParseResponseForNonJSON 验证非 JSON 响应不会解析成功。
func TestParseResponseForNonJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/demo", nil)

	respRecorder := createResponseRecorder(ctx)
	respRecorder.Header().Set("Content-Type", "text/plain")
	respRecorder.body.WriteString("pong")
	if resp := parseResponse(ctx, respRecorder); resp != nil {
		t.Fatal("expected nil response for non-json body")
	}
}

// TestBuildMaskedBodies 验证请求体和响应体截断逻辑。
func TestBuildMaskedBodies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/demo", bytes.NewBuffer(bytes.Repeat([]byte("a"), maxRequestBodySize+10)))
	ctx.Request.Header.Set("Content-Type", "application/json")
	cacheRequestBody(ctx)

	requestBody := buildMaskedRequestBody(ctx)
	if len(requestBody) == 0 {
		t.Fatal("expected masked request body")
	}
	cached := getRequestBodyCache(ctx)
	if cached == nil {
		t.Fatal("expected cached request body")
	}
	if !cached.truncated {
		t.Fatal("expected request body cache to be marked truncated")
	}
	if len(cached.body) != maxRequestBodySize {
		t.Fatalf("expected cached request body length %d, got %d", maxRequestBodySize, len(cached.body))
	}
	if !bytes.Contains([]byte(requestBody), []byte("truncated")) {
		t.Fatalf("expected truncated marker in request body, got %s", requestBody)
	}

	respRecorder := createResponseRecorder(ctx)
	_, _ = respRecorder.Write(bytes.Repeat([]byte("b"), maxResponseBodySize+10))
	responseBody := buildMaskedResponseBody(respRecorder)
	if len(responseBody) == 0 {
		t.Fatal("expected masked response body")
	}
	if !respRecorder.truncated {
		t.Fatal("expected response recorder to mark body as truncated")
	}
	if respRecorder.body.Len() != maxResponseBodySize {
		t.Fatalf("expected cached response body length %d, got %d", maxResponseBodySize, respRecorder.body.Len())
	}
	if !bytes.Contains([]byte(responseBody), []byte("truncated")) {
		t.Fatalf("expected truncated marker in response body, got %s", responseBody)
	}
}

func TestReadRequestBodyPreservesLargeRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	originalBody := bytes.Repeat([]byte("x"), maxRequestBodySize+128)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/demo", bytes.NewReader(originalBody))
	ctx.Request.Header.Set("Content-Type", "application/json")

	cacheRequestBody(ctx)

	cached := readRequestBody(ctx)
	if len(cached) != maxRequestBodySize {
		t.Fatalf("expected cached body length %d, got %d", maxRequestBodySize, len(cached))
	}

	remaining, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}
	if !bytes.Equal(remaining, originalBody) {
		t.Fatal("expected request body to remain readable after snapshot")
	}
}

// TestOperationStatusFromResponse 验证操作状态选择逻辑。
func TestOperationStatusFromResponse(t *testing.T) {
	recorder := &responseRecorder{statusCode: http.StatusBadRequest}
	if got := operationStatusFromResponse(recorder, &response.Result{Code: 10000}); got != 10000 {
		t.Fatalf("expected business code, got %d", got)
	}
	if got := operationStatusFromResponse(recorder, nil); got != http.StatusBadRequest {
		t.Fatalf("expected http status, got %d", got)
	}

	recorder.statusCode = http.StatusOK
	if got := operationStatusFromResponse(recorder, nil); got != 0 {
		t.Fatalf("expected default status 0, got %d", got)
	}
}

// TestLogRequestSkipsPing 验证 ping 请求不会触发后续日志处理。
func TestLogRequestSkipsPing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/ping", nil)
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())

	respRecorder := createResponseRecorder(ctx)
	logRequest(ctx, respRecorder)
}

func TestBuildRequestAuditLogSnapshotUsesPrincipal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/demo", bytes.NewBufferString(`{"name":"codex"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())
	ctx.Set(global.ContextKeyRequestID, "req-1")
	cacheRequestBody(ctx)

	auth.StoreAuthPrincipal(ctx, &auth.AuthPrincipal{
		UserID:   12,
		JWTID:    "jwt-1",
		Username: "tester",
		Nickname: "Tester",
	})

	respRecorder := createResponseRecorder(ctx)
	respRecorder.body.WriteString(`{"code":0,"msg":"ok","data":{"name":"demo"}}`)
	snapshot := buildRequestAuditLogSnapshot(ctx, respRecorder, &response.Result{Code: 0})
	if snapshot == nil {
		t.Fatal("expected audit snapshot")
	}
	if snapshot.OperatorID != 12 || snapshot.JwtID != "jwt-1" {
		t.Fatalf("unexpected operator fields: %#v", snapshot)
	}
	if snapshot.OperatorAccount != "tester" || snapshot.OperatorName != "Tester" {
		t.Fatalf("unexpected operator names: %#v", snapshot)
	}
	if snapshot.RequestKind != "http" {
		t.Fatalf("expected request kind http, got %q", snapshot.RequestKind)
	}
}

func TestBuildRequestAuditLogSnapshotMarksNotificationWSAsWebSocket(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/v1/common/ws?access_token=demo", nil)
	ctx.Set(global.ContextKeyRequestStartTime, time.Now().Add(-2*time.Second))
	ctx.Set(global.ContextKeyRequestID, "req-ws")

	respRecorder := createResponseRecorder(ctx)
	snapshot := buildRequestAuditLogSnapshot(ctx, respRecorder, &response.Result{Code: 0})
	if snapshot == nil {
		t.Fatal("expected audit snapshot")
	}
	if snapshot.RequestKind != "websocket" {
		t.Fatalf("expected request kind websocket, got %q", snapshot.RequestKind)
	}
	if snapshot.ExecutionTimeUs <= 0 {
		t.Fatalf("expected websocket execution_time_us > 0, got %d", snapshot.ExecutionTimeUs)
	}
	if snapshot.ExecutionTimeScope != "connection" {
		t.Fatalf("expected websocket execution_time_scope connection, got %q", snapshot.ExecutionTimeScope)
	}
}

func TestCalculateExecutionTimeMsClampsLargeHTTPDuration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set(global.ContextKeyRequestStartTime, time.Now().Add(-8*24*time.Hour))

	got, scope := calculateExecutionTimeMeta(ctx, "http")
	if got != requestLogExecutionTimeMaxUs {
		t.Fatalf("expected clamped execution_time_us %d, got %d", requestLogExecutionTimeMaxUs, got)
	}
	if scope != "request" {
		t.Fatalf("expected request scope, got %q", scope)
	}
}

func TestBuildRequestAuditLogSnapshotUsesAuditRequestBodyOverride(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/admin/v1/system/config/create", bytes.NewBufferString(`{"config_value":"plain-secret"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())
	ctx.Set(global.ContextKeyRequestID, "req-override")
	cacheRequestBody(ctx)
	SetAuditRequestBodyRaw(ctx, `{"config_value":"******"}`)

	respRecorder := createResponseRecorder(ctx)
	respRecorder.body.WriteString(`{"code":0,"msg":"ok","data":{}}`)
	snapshot := buildRequestAuditLogSnapshot(ctx, respRecorder, &response.Result{Code: 0})
	if snapshot == nil {
		t.Fatal("expected audit snapshot")
	}
	if snapshot.RequestBody != `{"config_value":"******"}` {
		t.Fatalf("unexpected request body override: %s", snapshot.RequestBody)
	}
}

func TestBuildRequestAuditLogSnapshotMarksHighRiskAndDiff(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/demo", bytes.NewBufferString(`{"name":"after"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())
	ctx.Set(global.ContextKeyRequestID, "req-2")
	cacheRequestBody(ctx)

	SetAuditHighRisk(ctx, true)
	SetAuditChangeDiff(ctx, map[string]any{"name": "before"}, map[string]any{"name": "after"})

	respRecorder := createResponseRecorder(ctx)
	respRecorder.body.WriteString(`{"code":0,"msg":"ok"}`)

	snapshot := buildRequestAuditLogSnapshot(ctx, respRecorder, &response.Result{Code: 0})
	if snapshot == nil {
		t.Fatal("expected audit snapshot")
	}
	if snapshot.IsHighRisk != 1 {
		t.Fatalf("expected high risk audit snapshot, got %#v", snapshot.IsHighRisk)
	}

	diff := map[string]any{}
	if err := json.Unmarshal([]byte(snapshot.ChangeDiff), &diff); err != nil {
		t.Fatalf("expected valid change diff json, got err=%v raw=%s", err, snapshot.ChangeDiff)
	}
	if _, ok := diff["before"]; !ok {
		t.Fatalf("expected before section in change diff, got %#v", diff)
	}
	if _, ok := diff["after"]; !ok {
		t.Fatalf("expected after section in change diff, got %#v", diff)
	}
}

func TestBuildRequestAuditLogSnapshotGetRequestIsNotHighRiskByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/demo", nil)
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())
	ctx.Set(global.ContextKeyRequestID, "req-3")

	respRecorder := createResponseRecorder(ctx)
	respRecorder.body.WriteString(`{"code":0,"msg":"ok"}`)

	snapshot := buildRequestAuditLogSnapshot(ctx, respRecorder, &response.Result{Code: 0})
	if snapshot == nil {
		t.Fatal("expected audit snapshot")
	}
	if snapshot.IsHighRisk != 0 {
		t.Fatalf("expected get request not high risk, got %#v", snapshot.IsHighRisk)
	}
	if snapshot.ChangeDiff != "" {
		t.Fatalf("expected empty change diff for get request, got %s", snapshot.ChangeDiff)
	}
}

// TestLogWebSocketConnectionEnqueuesWebSocketSnapshot 验证 WS 建连日志被记录，
// 且标记为 websocket、状态为 101，即使在会丢弃普通低风险 GET 的高采样率下也不被采样丢弃。
func TestLogWebSocketConnectionEnqueuesWebSocketSnapshot(t *testing.T) {
	var captured *auditsvc.AuditLogSnapshot
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		Enqueue: func(_ context.Context, kind string, snapshot *auditsvc.AuditLogSnapshot) error {
			if kind == jobs.AuditLogKindRequest {
				captured = snapshot
			}
			return nil
		},
	})
	defer restoreDeps()

	// keepEvery=2：普通低风险 GET 会被 1/2 采样，但 WS 应凭类型放行绕过采样。
	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Queue.Enable = true
		cfg.Queue.AuditSampleGetKeepEvery = 2
	})
	defer restoreConfig()

	restoreLogger := log.ReplaceLoggerForTesting(zap.NewNop())
	defer restoreLogger()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/v1/common/ws", nil)
	ctx.Set(global.ContextKeyRequestID, "ws-req-1")
	// 模拟连接已存活一段时间：记录点在断开后，执行时长应为请求开始至断开的连接全程时长。
	ctx.Set(global.ContextKeyRequestStartTime, time.Now().Add(-50*time.Millisecond))

	// 连续多次调用，确认每次都被记录（绕过采样的稳定性，而非偶发命中保留计数）。
	for i := 0; i < 5; i++ {
		captured = nil
		LogWebSocketConnection(ctx)
		if captured == nil {
			t.Fatalf("attempt %d: expected websocket audit log to be enqueued", i)
		}
		if captured.RequestKind != model.RequestKindWebSocket {
			t.Fatalf("attempt %d: expected request kind %q, got %q", i, model.RequestKindWebSocket, captured.RequestKind)
		}
		if captured.ResponseStatus != http.StatusSwitchingProtocols {
			t.Fatalf("attempt %d: expected response status 101, got %d", i, captured.ResponseStatus)
		}
		if captured.OperationStatus != 0 {
			t.Fatalf("attempt %d: expected operation status 0 (success), got %d", i, captured.OperationStatus)
		}
		if captured.BaseURL != "/admin/v1/common/ws" {
			t.Fatalf("attempt %d: unexpected base url %q", i, captured.BaseURL)
		}
		// 记录点在断开后：执行时长应为连接全程时长，按 WebSocket 类型计为 connection scope（微秒）。
		if captured.ExecutionTimeScope != model.ExecutionTimeScopeConnection {
			t.Fatalf("attempt %d: expected connection scope, got %q", i, captured.ExecutionTimeScope)
		}
		if captured.ExecutionTimeUs <= 0 {
			t.Fatalf("attempt %d: expected positive connection duration, got %v", i, captured.ExecutionTimeUs)
		}
	}
}

// TestFinalizeRequestLoggingSkipsWebSocketPath 验证中间件收尾路径对 WS 路径跳过记录，
// 避免与 handler 主动记录的连接日志重复。
func TestFinalizeRequestLoggingSkipsWebSocketPath(t *testing.T) {
	called := false
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		Enqueue: func(_ context.Context, _ string, _ *auditsvc.AuditLogSnapshot) error {
			called = true
			return nil
		},
	})
	defer restoreDeps()

	restoreLogger := log.ReplaceLoggerForTesting(zap.NewNop())
	defer restoreLogger()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/v1/common/ws", nil)
	ctx.Set(global.ContextKeyRequestID, "ws-req-skip")
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())

	respRecorder := createResponseRecorder(ctx)
	logRequest(ctx, respRecorder)
	if called {
		t.Fatal("expected middleware finalize to skip websocket path (handler records it)")
	}
}

// TestLogWebSocketConnectionNilContextIsNoop 验证空上下文不触发投递、不 panic。
func TestLogWebSocketConnectionNilContextIsNoop(t *testing.T) {
	called := false
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		Enqueue: func(_ context.Context, _ string, _ *auditsvc.AuditLogSnapshot) error {
			called = true
			return nil
		},
	})
	defer restoreDeps()

	LogWebSocketConnection(nil)
	if called {
		t.Fatal("expected no enqueue for nil context")
	}
}

func TestLogWebSocketConnectionCanRecordUpgradeFailure(t *testing.T) {
	var captured *auditsvc.AuditLogSnapshot
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		Enqueue: func(_ context.Context, kind string, snapshot *auditsvc.AuditLogSnapshot) error {
			if kind == jobs.AuditLogKindRequest {
				captured = snapshot
			}
			return nil
		},
	})
	defer restoreDeps()

	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Queue.Enable = true
	})
	defer restoreConfig()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/v1/common/ws", nil)
	ctx.Set(global.ContextKeyRequestID, "ws-req-failed")
	ctx.Set(global.ContextKeyRequestStartTime, time.Now().Add(-10*time.Millisecond))

	LogWebSocketConnection(ctx, WebSocketAuditOptions{
		OperationStatus: e.InvalidParameter,
		ResponseStatus:  http.StatusBadRequest,
		ResponseBody:    "upgrade failed",
	})

	if captured == nil {
		t.Fatal("expected websocket failure audit log to be enqueued")
	}
	if captured.OperationStatus != e.InvalidParameter {
		t.Fatalf("expected operation status InvalidParameter, got %d", captured.OperationStatus)
	}
	if captured.ResponseStatus != http.StatusBadRequest {
		t.Fatalf("expected response status 400, got %d", captured.ResponseStatus)
	}
	if captured.ResponseBody != "upgrade failed" {
		t.Fatalf("unexpected response body: %q", captured.ResponseBody)
	}
}
