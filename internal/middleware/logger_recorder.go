package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/pkg/response"
)

type requestBodyCache struct {
	body       []byte
	totalBytes int
	truncated  bool
}

// responseRecorder 响应记录器，用于记录响应内容。
type responseRecorder struct {
	gin.ResponseWriter
	captureBody   bool
	body          *bytes.Buffer
	statusCode    int
	responseBytes int
	truncated     bool
}

// Write 写入响应数据。
func (r *responseRecorder) Write(b []byte) (int, error) {
	r.cacheBody(b)
	return r.ResponseWriter.Write(b)
}

// WriteString 写入字符串响应。
func (r *responseRecorder) WriteString(s string) (int, error) {
	r.cacheBody([]byte(s))
	return r.ResponseWriter.WriteString(s)
}

// WriteHeader 写入HTTP状态码。
func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

// cacheRequestBody 缓存请求体到上下文。
func cacheRequestBody(c *gin.Context) {
	if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
		return
	}
	if _, exists := c.Get("requestBody"); exists {
		return
	}
	if c.Request.Body == nil {
		return
	}
	if shouldSkipRequestBodyCache(c) {
		return
	}

	bodyBytes, totalBytes, truncated, err := snapshotRequestBody(c.Request)
	if err != nil || len(bodyBytes) == 0 {
		return
	}
	c.Set("requestBody", &requestBodyCache{
		body:       truncateBytes(bodyBytes, maxRequestBodySize),
		totalBytes: totalBytes,
		truncated:  truncated,
	})
}

// createResponseRecorder 创建响应记录器。
func createResponseRecorder(c *gin.Context) *responseRecorder {
	return &responseRecorder{
		ResponseWriter: c.Writer,
		captureBody:    shouldCaptureResponseBody(c),
		body:           bytes.NewBuffer(nil),
		statusCode:     defaultStatusCode,
	}
}

func (r *responseRecorder) cacheBody(b []byte) {
	if !r.captureBody {
		return
	}
	r.responseBytes += len(b)
	if r.truncated || len(b) == 0 {
		return
	}

	remaining := maxResponseBodySize - r.body.Len()
	if remaining <= 0 {
		r.truncated = true
		return
	}
	if len(b) <= remaining {
		_, _ = r.body.Write(b)
		return
	}

	_, _ = r.body.Write(b[:remaining])
	r.truncated = true
}

// parseResponse 解析响应数据。
func parseResponse(c *gin.Context, recorder *responseRecorder) *response.Result {
	if recorder == nil || !recorder.captureBody || recorder.body.Len() == 0 {
		return nil
	}
	if !strings.Contains(strings.ToLower(recorder.Header().Get("Content-Type")), "json") {
		return nil
	}
	var resp response.Result
	if err := json.Unmarshal(recorder.body.Bytes(), &resp); err != nil {
		return nil
	}

	if c.Request.Method == http.MethodGet {
		resp.Data = nil
	}
	return &resp
}

func shouldCaptureResponseBody(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return true
	}
	if c.Request.URL.Path == "/ping" {
		return false
	}
	return c.Request.Method != http.MethodGet
}

// readRequestBody 读取请求体缓存。
func readRequestBody(c *gin.Context) []byte {
	if body, exists := c.Get("requestBody"); exists {
		if cached, ok := body.(*requestBodyCache); ok && len(cached.body) > 0 {
			return cached.body
		}
	}

	if c.Request.Body == nil {
		return nil
	}
	if shouldSkipRequestBodyCache(c) {
		return nil
	}

	bodyBytes, totalBytes, truncated, err := snapshotRequestBody(c.Request)
	if err != nil || len(bodyBytes) == 0 {
		return nil
	}

	cached := &requestBodyCache{
		body:       truncateBytes(bodyBytes, maxRequestBodySize),
		totalBytes: totalBytes,
		truncated:  truncated,
	}
	c.Set("requestBody", cached)
	return cached.body
}

func getRequestBodyCache(c *gin.Context) *requestBodyCache {
	if body, exists := c.Get("requestBody"); exists {
		if cached, ok := body.(*requestBodyCache); ok {
			return cached
		}
	}
	return nil
}

func truncateBytes(body []byte, maxSize int) []byte {
	if len(body) <= maxSize {
		return body
	}
	return body[:maxSize]
}

func shouldSkipRequestBodyCache(c *gin.Context) bool {
	contentType := strings.ToLower(c.GetHeader("Content-Type"))
	switch {
	case strings.HasPrefix(contentType, "multipart/form-data"):
		return true
	case strings.HasPrefix(contentType, "application/octet-stream"):
		return true
	default:
		return false
	}
}

func snapshotRequestBody(req *http.Request) ([]byte, int, bool, error) {
	if req == nil || req.Body == nil {
		return nil, 0, false, nil
	}

	if req.ContentLength >= 0 && req.ContentLength <= maxRequestBodySize {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil || len(bodyBytes) == 0 {
			return nil, 0, false, err
		}
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		return bodyBytes, len(bodyBytes), false, nil
	}

	peekLimit := int64(maxRequestBodySize + 1)
	bodyBytes, err := io.ReadAll(io.LimitReader(req.Body, peekLimit))
	if err != nil || len(bodyBytes) == 0 {
		return nil, 0, false, err
	}

	req.Body = io.NopCloser(io.MultiReader(bytes.NewReader(bodyBytes), req.Body))

	truncated := len(bodyBytes) > maxRequestBodySize
	totalBytes := len(bodyBytes)
	if req.ContentLength > int64(totalBytes) {
		totalBytes = int(req.ContentLength)
		truncated = true
	}

	return bodyBytes, totalBytes, truncated, nil
}
