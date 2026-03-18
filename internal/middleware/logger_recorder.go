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

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil || len(bodyBytes) == 0 {
		return
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	c.Set("requestBody", &requestBodyCache{
		body:       truncateBytes(bodyBytes, maxRequestBodySize),
		totalBytes: len(bodyBytes),
		truncated:  len(bodyBytes) > maxRequestBodySize,
	})
}

// createResponseRecorder 创建响应记录器。
func createResponseRecorder(c *gin.Context) *responseRecorder {
	return &responseRecorder{
		ResponseWriter: c.Writer,
		body:           bytes.NewBuffer(nil),
		statusCode:     defaultStatusCode,
	}
}

func (r *responseRecorder) cacheBody(b []byte) {
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
	var resp response.Result
	if err := json.Unmarshal(recorder.body.Bytes(), &resp); err != nil {
		return nil
	}

	if c.Request.Method == http.MethodGet {
		resp.Data = nil
	}
	return &resp
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

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil || len(bodyBytes) == 0 {
		return nil
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	cached := &requestBodyCache{
		body:       truncateBytes(bodyBytes, maxRequestBodySize),
		totalBytes: len(bodyBytes),
		truncated:  len(bodyBytes) > maxRequestBodySize,
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
