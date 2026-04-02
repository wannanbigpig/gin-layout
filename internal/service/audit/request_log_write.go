package audit

import (
	"github.com/wannanbigpig/gin-layout/internal/model"
)

// AuditLogSnapshot 表示请求结束时提取出的审计日志快照。
type AuditLogSnapshot struct {
	RequestID       string  `json:"request_id"`
	JwtID           string  `json:"jwt_id"`
	OperatorID      uint    `json:"operator_id"`
	OperatorAccount string  `json:"operator_account"`
	OperatorName    string  `json:"operator_name"`
	IP              string  `json:"ip"`
	UserAgent       string  `json:"user_agent"`
	OS              string  `json:"os"`
	Browser         string  `json:"browser"`
	Method          string  `json:"method"`
	BaseURL         string  `json:"base_url"`
	OperationName   string  `json:"operation_name"`
	OperationStatus int     `json:"operation_status"`
	RequestHeaders  string  `json:"request_headers"`
	RequestQuery    string  `json:"request_query"`
	RequestBody     string  `json:"request_body"`
	ResponseStatus  int     `json:"response_status"`
	ResponseBody    string  `json:"response_body"`
	ResponseHeader  string  `json:"response_header"`
	ExecutionTime   float64 `json:"execution_time"`
}

// PersistAuditLog 将请求审计日志快照写入数据库。
func PersistAuditLog(snapshot *AuditLogSnapshot) error {
	if snapshot == nil || snapshot.RequestID == "" {
		return nil
	}

	requestLog := model.NewRequestLogs()
	requestLog.RequestID = snapshot.RequestID
	requestLog.JwtID = snapshot.JwtID
	requestLog.OperatorID = snapshot.OperatorID
	requestLog.IP = snapshot.IP
	requestLog.UserAgent = snapshot.UserAgent
	requestLog.OS = snapshot.OS
	requestLog.Browser = snapshot.Browser
	requestLog.Method = snapshot.Method
	requestLog.BaseURL = snapshot.BaseURL
	requestLog.OperationName = snapshot.OperationName
	requestLog.OperationStatus = snapshot.OperationStatus
	requestLog.OperatorAccount = snapshot.OperatorAccount
	requestLog.OperatorName = snapshot.OperatorName
	requestLog.RequestHeaders = snapshot.RequestHeaders
	requestLog.RequestQuery = snapshot.RequestQuery
	requestLog.RequestBody = snapshot.RequestBody
	requestLog.ResponseStatus = snapshot.ResponseStatus
	requestLog.ResponseBody = snapshot.ResponseBody
	requestLog.ResponseHeader = snapshot.ResponseHeader
	requestLog.ExecutionTime = snapshot.ExecutionTime

	db, err := requestLog.GetDB()
	if err != nil {
		return err
	}
	return db.Create(requestLog).Error
}
