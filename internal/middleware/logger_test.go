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
	ctx.Request = httptest.NewRequest(http.MethodGet, "/demo", nil)

	respRecorder := createResponseRecorder(ctx)
	respRecorder.body.WriteString(`{"code":0,"msg":"ok","data":{"name":"demo"}}`)

	resp := parseResponse(ctx, respRecorder)
	if resp == nil {
		t.Fatal("expected parsed response")
	}
	if resp.Code != 0 {
		t.Fatalf("expected code 0, got %d", resp.Code)
	}
	if resp.Data != nil {
		t.Fatal("expected get response data to be cleared")
	}
}

// TestParseResponseForNonJSON 验证非 JSON 响应不会解析成功。
func TestParseResponseForNonJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/demo", nil)

	respRecorder := createResponseRecorder(ctx)
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
