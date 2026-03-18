package middleware

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mssola/useragent"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/response"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/sensitive"
	accesssvc "github.com/wannanbigpig/gin-layout/internal/service/access"
)

// saveRequestLogToDB 保存请求日志到数据库。
func saveRequestLogToDB(c *gin.Context, recorder *responseRecorder, resp *response.Result) {
	defer func() {
		if r := recover(); r != nil {
			log.Error("保存请求日志到数据库时发生panic", zap.Any("error", r))
		}
	}()

	duration := time.Since(c.GetTime(global.ContextKeyRequestStartTime))
	executionTime := float64(duration.Nanoseconds()) / 1000000.0
	executionTime = float64(int(executionTime*10000+0.5)) / 10000.0

	requestID := c.GetString(global.ContextKeyRequestID)
	if requestID == "" {
		return
	}

	operatorID := c.GetUint(global.ContextKeyUID)
	jwtID := c.GetString("jwt_id")
	operatorAccount := c.GetString("username")
	operatorName := c.GetString("nickname")

	userAgentStr := c.Request.UserAgent()
	ua := useragent.New(userAgentStr)
	os := ua.OS()
	browser, _ := ua.Browser()

	requestHeaders := sensitive.GetMaskedRequestHeaders(c.Request.Header)
	requestQuery := sensitive.MaskQueryString(c.Request.URL.RawQuery)
	requestBody := buildMaskedRequestBody(c)
	responseBody := buildMaskedResponseBody(recorder)
	responseHeader := sensitive.GetMaskedResponseHeaders(recorder.Header())

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
	requestLog.OperationStatus = operationStatusFromResponse(recorder, resp)
	requestLog.OperatorAccount = operatorAccount
	requestLog.OperatorName = operatorName
	requestLog.RequestHeaders = requestHeaders
	requestLog.RequestQuery = requestQuery
	requestLog.RequestBody = requestBody
	requestLog.ResponseStatus = recorder.statusCode
	requestLog.ResponseBody = responseBody
	requestLog.ResponseHeader = responseHeader
	requestLog.ExecutionTime = executionTime

	db, err := requestLog.GetDB()
	if err != nil {
		log.Error("保存请求日志到数据库失败", zap.Error(err), zap.String("request_id", requestID))
		return
	}
	if err := db.Create(requestLog).Error; err != nil {
		log.Error("保存请求日志到数据库失败", zap.Error(err), zap.String("request_id", requestID))
	}
}

// savePanicRequestLogToDB 为 panic 请求补充数据库审计日志。
func savePanicRequestLogToDB(c *gin.Context, panicMessage string) {
	defer func() {
		if r := recover(); r != nil {
			log.Error("保存 panic 请求日志到数据库时发生panic", zap.Any("error", r))
		}
	}()

	requestID := c.GetString(global.ContextKeyRequestID)
	if requestID == "" {
		return
	}

	duration := time.Since(c.GetTime(global.ContextKeyRequestStartTime))
	executionTime := float64(duration.Nanoseconds()) / 1000000.0
	executionTime = float64(int(executionTime*10000+0.5)) / 10000.0

	userAgentStr := c.Request.UserAgent()
	ua := useragent.New(userAgentStr)
	os := ua.OS()
	browser, _ := ua.Browser()

	responseBody, err := json.Marshal(response.Result{
		Code:      http.StatusInternalServerError,
		Msg:       panicMessage,
		Data:      map[string]any{},
		RequestId: requestID,
	})
	if err != nil {
		responseBody = []byte{}
	}

	requestLog := model.NewRequestLogs()
	requestLog.RequestID = requestID
	requestLog.JwtID = c.GetString("jwt_id")
	requestLog.OperatorID = c.GetUint(global.ContextKeyUID)
	requestLog.IP = c.ClientIP()
	requestLog.UserAgent = userAgentStr
	requestLog.OS = os
	requestLog.Browser = browser
	requestLog.Method = c.Request.Method
	requestLog.BaseURL = c.Request.URL.Path
	requestLog.OperationName = getOperationName(c)
	requestLog.OperationStatus = http.StatusInternalServerError
	requestLog.OperatorAccount = c.GetString("username")
	requestLog.OperatorName = c.GetString("nickname")
	requestLog.RequestHeaders = sensitive.GetMaskedRequestHeaders(c.Request.Header)
	requestLog.RequestQuery = sensitive.MaskQueryString(c.Request.URL.RawQuery)
	requestLog.RequestBody = buildMaskedRequestBody(c)
	requestLog.ResponseStatus = http.StatusInternalServerError
	requestLog.ResponseBody = string(responseBody)
	requestLog.ExecutionTime = executionTime

	db, dbErr := requestLog.GetDB()
	if dbErr != nil {
		log.Error("保存 panic 请求日志到数据库失败", zap.Error(dbErr), zap.String("request_id", requestID))
		return
	}
	if err := db.Create(requestLog).Error; err != nil {
		log.Error("保存 panic 请求日志到数据库失败", zap.Error(err), zap.String("request_id", requestID))
	}
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

func buildMaskedResponseBody(recorder *responseRecorder) string {
	bodyBytes := recorder.body.Bytes()
	if len(bodyBytes) == 0 {
		return ""
	}

	if !recorder.truncated {
		return sensitive.GetMaskedResponseBody(bodyBytes)
	}

	return sensitive.GetMaskedResponseBody(bodyBytes) + "...(truncated,total_size=" + strconv.Itoa(recorder.responseBytes) + "B)"
}

func operationStatusFromResponse(recorder *responseRecorder, resp *response.Result) int {
	if resp != nil {
		return resp.Code
	}
	if recorder.statusCode >= http.StatusBadRequest {
		return recorder.statusCode
	}
	return 0
}

// getOperationName 获取操作名称。
func getOperationName(c *gin.Context) string {
	route := c.Request.URL.Path
	method := c.Request.Method

	if operationName := accesssvc.NewApiRouteCacheService().GetApiName(route, method); operationName != "" {
		return operationName
	}
	if operationName := c.GetHeader("X-Operation-Name"); operationName != "" {
		return operationName
	}
	return route
}
