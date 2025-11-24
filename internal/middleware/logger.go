package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mssola/useragent"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/response"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/sensitive"
)

const (
	defaultStatusCode   = http.StatusOK
	maxRequestBodySize  = 50 * 1024 // 请求体最大记录大小：50KB
	maxResponseBodySize = 50 * 1024 // 响应体最大记录大小：50KB
)

// responseRecorder 响应记录器，用于记录响应内容
type responseRecorder struct {
	gin.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

// Write 写入响应数据
func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// WriteString 写入字符串响应
func (r *responseRecorder) WriteString(s string) (int, error) {
	r.body.WriteString(s)
	return r.ResponseWriter.WriteString(s)
}

// WriteHeader 写入HTTP状态码
func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

// CustomLogger 自定义日志中间件
func CustomLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 在请求处理前读取并缓存请求体（避免后续处理消耗后无法读取）
		cacheRequestBody(c)

		// 创建响应记录器并替换 c.Writer（这样才能捕获响应）
		recorder := createResponseRecorder(c)
		c.Writer = recorder

		// 处理请求
		c.Next()

		// 记录请求日志
		logRequest(c, recorder)
	}
}

// cacheRequestBody 缓存请求体到上下文（在请求处理前调用）
func cacheRequestBody(c *gin.Context) {
	// 只处理有请求体的方法
	if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
		return
	}

	// 如果已经缓存过，直接返回
	if _, exists := c.Get("requestBody"); exists {
		return
	}

	// 读取请求体
	if c.Request.Body != nil {
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err == nil && len(bodyBytes) > 0 {
			// 重新设置请求体，供后续处理使用
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			// 缓存到上下文，供日志记录使用
			c.Set("requestBody", bodyBytes)
		}
	}
}

// createResponseRecorder 创建响应记录器
func createResponseRecorder(c *gin.Context) *responseRecorder {
	return &responseRecorder{
		ResponseWriter: c.Writer,
		body:           bytes.NewBuffer(nil),
		statusCode:     defaultStatusCode,
	}
}

// logRequest 记录请求日志
func logRequest(c *gin.Context, recorder *responseRecorder) {
	// ping 请求不记录日志
	if c.Request.URL.Path == "/ping" {
		return
	}

	// 解析响应数据
	resp := parseResponse(c, recorder)

	// 记录精简日志到文件（用于快速查看和调试）
	logRequestToFile(c, recorder)

	// 保存详细日志到数据库（异步执行，避免影响响应速度）
	go saveRequestLogToDB(c, recorder, resp)
}

// logRequestToFile 记录精简日志到文件（用于快速查看和调试）
func logRequestToFile(c *gin.Context, recorder *responseRecorder) {
	requestID := c.GetString("requestId")
	if requestID == "" {
		return // 如果没有请求ID，不记录日志
	}

	cost := time.Since(c.GetTime("requestStartTime"))
	uid := c.GetUint("uid")

	logFields := []zap.Field{
		zap.String("requestId", requestID), // 关联字段，用于关联数据库日志
		zap.Uint("uid", uid),
		zap.Int("status", recorder.statusCode),
		zap.String("method", c.Request.Method),
		zap.String("path", c.Request.URL.Path),
		zap.String("ip", c.ClientIP()),
		zap.Duration("cost", cost),
	}

	// 如果有错误，记录错误信息
	if errors := c.Errors.ByType(gin.ErrorTypePrivate).String(); errors != "" {
		logFields = append(logFields, zap.String("errors", errors))
	}

	// 记录日志到文件
	log.Info(c.Request.URL.Path, logFields...)
}

// parseResponse 解析响应数据
func parseResponse(c *gin.Context, recorder *responseRecorder) *response.Result {
	var resp response.Result
	bodyBytes := recorder.body.Bytes()

	// 尝试解析JSON响应
	if err := json.Unmarshal(bodyBytes, &resp); err != nil {
		return nil
	}

	// GET请求不记录数据部分，减少日志大小
	if c.Request.Method == http.MethodGet {
		resp.Data = nil
	}

	return &resp
}

// readRequestBody 读取请求体（不影响后续处理）
// 注意：此函数在请求处理之后调用，如果请求体已被消耗则无法读取
func readRequestBody(c *gin.Context) []byte {
	// 优先从上下文获取（如果之前已经读取过）
	if body, exists := c.Get("requestBody"); exists {
		if bodyBytes, ok := body.([]byte); ok && len(bodyBytes) > 0 {
			return bodyBytes
		}
	}

	// 尝试从请求中读取（仅当请求体未被消耗时）
	if c.Request.Body != nil {
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err == nil && len(bodyBytes) > 0 {
			// 重新设置请求体（虽然此时已经处理完毕，但保持一致性）
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			// 缓存到上下文，避免重复读取
			c.Set("requestBody", bodyBytes)
			return bodyBytes
		}
	}

	return nil
}

// saveRequestLogToDB 保存请求日志到数据库
func saveRequestLogToDB(c *gin.Context, recorder *responseRecorder, resp *response.Result) {
	defer func() {
		if r := recover(); r != nil {
			log.Error("保存请求日志到数据库时发生panic", zap.Any("error", r))
		}
	}()

	// 计算执行时间
	executionTime := int(time.Since(c.GetTime("requestStartTime")).Milliseconds())

	// 获取请求ID
	requestID := c.GetString("requestId")
	if requestID == "" {
		return // 如果没有请求ID，不记录日志
	}

	// 获取用户信息
	operatorID := c.GetUint("uid")
	jwtID := c.GetString("jwt_id")
	operatorAccount := c.GetString("username")
	operatorName := c.GetString("nickname")

	// 解析 user_agent 获取 OS 和 Browser
	userAgentStr := c.Request.UserAgent()
	ua := useragent.New(userAgentStr)
	os := ua.OS()
	browser, _ := ua.Browser()
	fmt.Println(os, browser, ua.Platform())

	// 获取请求头（JSON格式，对敏感信息进行脱敏）
	requestHeaders := sensitive.GetMaskedRequestHeaders(c.Request.Header)

	// 获取请求参数（对敏感参数进行脱敏）
	requestQuery := sensitive.MaskQueryString(c.Request.URL.RawQuery)

	// 获取请求体（对敏感信息进行脱敏）
	requestBody := ""
	if bodyBytes := readRequestBody(c); bodyBytes != nil {
		// 限制请求体大小，避免记录过大的数据
		if len(bodyBytes) <= maxRequestBodySize {
			contentType := c.Request.Header.Get("Content-Type")
			requestBody = sensitive.GetMaskedRequestBody(bodyBytes, contentType)
		} else {
			// 先截断，再脱敏
			truncatedBytes := bodyBytes[:maxRequestBodySize]
			contentType := c.Request.Header.Get("Content-Type")
			requestBody = sensitive.GetMaskedRequestBody(truncatedBytes, contentType) + "...(truncated)"
		}
	}

	// 获取响应体（对敏感信息进行脱敏）
	responseBody := ""
	bodyBytes := recorder.body.Bytes()
	if len(bodyBytes) > 0 {
		// 限制响应体大小
		if len(bodyBytes) <= maxResponseBodySize {
			responseBody = sensitive.GetMaskedResponseBody(bodyBytes)
		} else {
			// 先截断，再脱敏
			truncatedBytes := bodyBytes[:maxResponseBodySize]
			responseBody = sensitive.GetMaskedResponseBody(truncatedBytes) + "...(truncated)"
		}
	}

	// 获取响应头（JSON格式，对敏感信息进行脱敏）
	responseHeader := sensitive.GetMaskedResponseHeaders(recorder.Header())

	// 确定操作状态：直接存储响应返回的code状态码，0=成功，其他=失败
	var operationStatus int
	if resp != nil {
		// 如果响应解析成功，直接使用业务code
		operationStatus = resp.Code
	} else {
		// 如果响应解析失败，使用HTTP状态码（>=400表示失败）
		if recorder.statusCode >= 400 {
			operationStatus = recorder.statusCode
		} else {
			operationStatus = 0 // 默认成功
		}
	}

	// 创建请求日志记录
	requestLog := model.NewRequestLogs()
	requestLog.RequestID = requestID
	requestLog.JwtID = jwtID
	requestLog.OperatorID = operatorID
	requestLog.IP = c.ClientIP()
	requestLog.UserAgent = userAgentStr
	requestLog.OS = os
	requestLog.Browser = browser
	requestLog.Method = c.Request.Method
	requestLog.BaseURL = c.Request.URL.Path
	requestLog.OperationName = getOperationName(c)
	requestLog.OperationStatus = operationStatus
	requestLog.OperatorAccount = operatorAccount
	requestLog.OperatorName = operatorName
	requestLog.RequestHeaders = requestHeaders
	requestLog.RequestQuery = requestQuery
	requestLog.RequestBody = requestBody
	requestLog.ResponseStatus = recorder.statusCode
	requestLog.ResponseBody = responseBody
	requestLog.ResponseHeader = responseHeader
	requestLog.ExecutionTime = executionTime

	// 保存到数据库
	if err := requestLog.DB().Create(requestLog).Error; err != nil {
		log.Error("保存请求日志到数据库失败", zap.Error(err), zap.String("request_id", requestID))
	}
}

// getOperationName 获取操作名称（从 a_api 表中查询，带缓存优化）
func getOperationName(c *gin.Context) string {
	route := c.Request.URL.Path
	method := c.Request.Method

	// 从 a_api 表中查询操作名称（使用缓存优化）
	apiModel := model.NewApi()
	if operationName := apiModel.GetApiName(route, method); operationName != "" {
		return operationName
	}

	// 如果查询不到，优先从请求头获取操作名称
	if operationName := c.GetHeader("X-Operation-Name"); operationName != "" {
		return operationName
	}

	// 最后使用路径作为操作名称
	return route
}
