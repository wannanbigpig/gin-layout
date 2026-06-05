package middleware

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mssola/useragent"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/response"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/sensitive"
	accesssvc "github.com/wannanbigpig/gin-layout/internal/service/access"
	auditsvc "github.com/wannanbigpig/gin-layout/internal/service/audit"
	"github.com/wannanbigpig/gin-layout/internal/service/auth"
)

func buildRequestAuditLogSnapshot(c *gin.Context, recorder *responseRecorder, resp *response.Result) *auditsvc.AuditLogSnapshot {
	if c == nil {
		return nil
	}
	return buildAuditLogSnapshot(c, recorder, operationStatusFromResponse(recorder, resp), buildMaskedResponseBody(recorder), sensitive.GetMaskedResponseHeaders(recorder.Header()))
}

func buildPanicAuditLogSnapshot(c *gin.Context, panicMessage string) *auditsvc.AuditLogSnapshot {
	if c == nil {
		return nil
	}

	responseBody, err := json.Marshal(response.Result{
		Code:      http.StatusInternalServerError,
		Msg:       panicMessage,
		Data:      map[string]any{},
		RequestId: c.GetString(global.ContextKeyRequestID),
	})
	if err != nil {
		responseBody = []byte{}
	}

	return buildAuditLogSnapshot(c, nil, http.StatusInternalServerError, string(responseBody), "")
}

type auditRequestMeta struct {
	requestID       string
	method          string
	path            string
	ip              string
	userAgent       string
	os              string
	browser         string
	requestKind     string
	operationName   string
	requestHeaders  string
	requestQuery    string
	requestBody     string
	executionTimeUs int64
	executionScope  string
}

type auditOperatorMeta struct {
	operatorID      uint
	jwtID           string
	operatorAccount string
	operatorName    string
}

const requestLogExecutionTimeMaxUs int64 = 99999999999999

// buildAuditLogSnapshot 组装审计快照（仅负责编排，不包含具体字段提取细节）。
func buildAuditLogSnapshot(c *gin.Context, recorder *responseRecorder, operationStatus int, responseBody string, responseHeader string) *auditsvc.AuditLogSnapshot {
	requestMeta := collectAuditRequestMeta(c)
	if requestMeta == nil {
		return nil
	}

	operatorMeta := collectAuditOperatorMeta(c)
	isHighRisk := resolveAuditHighRisk(c, requestMeta.method)
	changeDiff := resolveAuditChangeDiff(c, isHighRisk, requestMeta.requestBody, responseBody)

	return &auditsvc.AuditLogSnapshot{
		RequestID:          requestMeta.requestID,
		JwtID:              operatorMeta.jwtID,
		OperatorID:         operatorMeta.operatorID,
		OperatorAccount:    operatorMeta.operatorAccount,
		OperatorName:       operatorMeta.operatorName,
		IP:                 requestMeta.ip,
		UserAgent:          requestMeta.userAgent,
		OS:                 requestMeta.os,
		Browser:            requestMeta.browser,
		RequestKind:        requestMeta.requestKind,
		Method:             requestMeta.method,
		BaseURL:            requestMeta.path,
		OperationName:      requestMeta.operationName,
		OperationStatus:    operationStatus,
		IsHighRisk:         isHighRisk,
		RequestHeaders:     requestMeta.requestHeaders,
		RequestQuery:       requestMeta.requestQuery,
		RequestBody:        requestMeta.requestBody,
		ChangeDiff:         changeDiff,
		ResponseStatus:     resolveAuditResponseStatus(recorder),
		ResponseBody:       responseBody,
		ResponseHeader:     responseHeader,
		ExecutionTimeUs:    requestMeta.executionTimeUs,
		ExecutionTimeScope: requestMeta.executionScope,
	}
}

// collectAuditRequestMeta 提取请求侧审计信息（请求标识、UA、请求体、耗时等）。
func collectAuditRequestMeta(c *gin.Context) *auditRequestMeta {
	requestID := c.GetString(global.ContextKeyRequestID)
	if requestID == "" {
		return nil
	}

	method := c.Request.Method
	path := c.Request.URL.Path
	userAgentStr := c.Request.UserAgent()
	ua := useragent.New(userAgentStr)
	browser, _ := ua.Browser()
	requestKind := resolveAuditRequestKind(c)
	executionTimeUs, executionScope := calculateExecutionTimeMeta(c, requestKind)

	return &auditRequestMeta{
		requestID:       requestID,
		method:          method,
		path:            path,
		ip:              c.ClientIP(),
		userAgent:       userAgentStr,
		os:              ua.OS(),
		browser:         browser,
		requestKind:     requestKind,
		operationName:   getOperationName(path, method, c.GetHeader("X-Operation-Name")),
		requestHeaders:  sensitive.GetMaskedRequestHeaders(c.Request.Header),
		requestQuery:    sensitive.MaskQueryString(c.Request.URL.RawQuery),
		requestBody:     resolveAuditRequestBody(c),
		executionTimeUs: executionTimeUs,
		executionScope:  executionScope,
	}
}

func resolveAuditRequestKind(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return model.RequestKindHTTP
	}
	path := c.Request.URL.Path
	if _, ok := websocketLoggingPaths[path]; ok {
		return model.RequestKindWebSocket
	}
	return model.RequestKindHTTP
}

// collectAuditOperatorMeta 提取操作者信息（优先 principal，回退到上下文 uid）。
func collectAuditOperatorMeta(c *gin.Context) auditOperatorMeta {
	if principal := auth.GetAuthPrincipal(c); principal != nil {
		return auditOperatorMeta{
			operatorID:      principal.UserID,
			jwtID:           principal.JWTID,
			operatorAccount: principal.Username,
			operatorName:    principal.Nickname,
		}
	}

	return auditOperatorMeta{
		operatorID: c.GetUint(global.ContextKeyUID),
	}
}

func calculateExecutionTimeMeta(c *gin.Context, requestKind string) (int64, string) {
	scope := model.ExecutionTimeScopeRequest
	switch requestKind {
	case model.RequestKindWebSocket:
		scope = model.ExecutionTimeScopeConnection
	}
	if c == nil {
		return 0, model.ExecutionTimeScopeNone
	}
	startTime := c.GetTime(global.ContextKeyRequestStartTime)
	if startTime.IsZero() {
		return 0, model.ExecutionTimeScopeNone
	}
	duration := time.Since(startTime)
	executionTimeUs := duration.Microseconds()
	if executionTimeUs < 0 {
		return 0, model.ExecutionTimeScopeNone
	}
	if executionTimeUs > requestLogExecutionTimeMaxUs {
		return requestLogExecutionTimeMaxUs, scope
	}
	return executionTimeUs, scope
}

func resolveAuditResponseStatus(recorder *responseRecorder) int {
	if recorder == nil {
		return http.StatusOK
	}
	return recorder.statusCode
}

func buildMaskedRequestBody(c *gin.Context) string {
	cached := getRequestBodyCache(c)
	if cached == nil {
		bodyBytes := readRequestBody(c)
		if bodyBytes == nil {
			return ""
		}
		cached = getRequestBodyCache(c)
	}
	if cached == nil || len(cached.body) == 0 {
		return ""
	}

	contentType := c.Request.Header.Get("Content-Type")
	if !cached.truncated {
		return sensitive.GetMaskedRequestBody(cached.body, contentType)
	}

	return sensitive.GetMaskedRequestBody(cached.body, contentType) + "...(truncated,total_size=" + strconv.Itoa(cached.totalBytes) + "B)"
}

func resolveAuditRequestBody(c *gin.Context) string {
	if c != nil {
		if raw, exists := c.Get(global.ContextKeyAuditRequestBody); exists {
			if value, ok := raw.(string); ok && strings.TrimSpace(value) != "" {
				return value
			}
		}
	}
	return buildMaskedRequestBody(c)
}

func buildMaskedResponseBody(recorder *responseRecorder) string {
	if recorder == nil {
		return ""
	}
	bodyBytes := recorder.body.Bytes()
	if len(bodyBytes) == 0 {
		return ""
	}

	if !recorder.truncated {
		return sensitive.GetMaskedResponseBody(bodyBytes)
	}

	return sensitive.GetMaskedResponseBody(bodyBytes) + "...(truncated,total_size=" + strconv.Itoa(recorder.responseBytes) + "B)"
}

func resolveAuditHighRisk(c *gin.Context, method string) uint8 {
	if c != nil {
		if raw, exists := c.Get(global.ContextKeyAuditHighRisk); exists {
			switch value := raw.(type) {
			case bool:
				if value {
					return 1
				}
				return 0
			case uint8:
				if value > 0 {
					return 1
				}
				return 0
			case int:
				if value > 0 {
					return 1
				}
				return 0
			case string:
				if strings.EqualFold(value, "1") || strings.EqualFold(value, "true") {
					return 1
				}
				if strings.EqualFold(value, "0") || strings.EqualFold(value, "false") {
					return 0
				}
			}
		}
	}

	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return 0
	default:
		return 1
	}
}

func resolveAuditChangeDiff(c *gin.Context, isHighRisk uint8, requestBody, responseBody string) string {
	if c != nil {
		if raw, exists := c.Get(global.ContextKeyAuditChangeDiff); exists {
			switch value := raw.(type) {
			case string:
				if strings.TrimSpace(value) != "" {
					return value
				}
			default:
				bytes, err := json.Marshal(value)
				if err == nil {
					return string(bytes)
				}
			}
		}
	}

	if isHighRisk == 0 || (requestBody == "" && responseBody == "") {
		return ""
	}

	payload := map[string]any{
		"request_body":  requestBody,
		"response_body": responseBody,
	}
	bytes, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(bytes)
}

func operationStatusFromResponse(recorder *responseRecorder, resp *response.Result) int {
	if resp != nil {
		return resp.Code
	}
	if recorder != nil && recorder.statusCode >= http.StatusBadRequest {
		return recorder.statusCode
	}
	return 0
}

// getOperationName 获取操作名称。
func getOperationName(route string, method string, headerOperationName string) string {
	if operationName := accesssvc.NewApiRouteCacheService().GetApiName(route, method); operationName != "" {
		return operationName
	}
	if headerOperationName != "" {
		return headerOperationName
	}
	return route
}
