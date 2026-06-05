package audit

import (
	"context"

	"github.com/wannanbigpig/gin-layout/internal/model"
)

// AuditLogSnapshot 表示请求结束时提取出的审计日志快照。
type AuditLogSnapshot struct {
	RequestID          string `json:"request_id"`
	JwtID              string `json:"jwt_id"`
	OperatorID         uint   `json:"operator_id"`
	OperatorAccount    string `json:"operator_account"`
	OperatorName       string `json:"operator_name"`
	IP                 string `json:"ip"`
	UserAgent          string `json:"user_agent"`
	OS                 string `json:"os"`
	Browser            string `json:"browser"`
	RequestKind        string `json:"request_kind"`
	Method             string `json:"method"`
	BaseURL            string `json:"base_url"`
	OperationName      string `json:"operation_name"`
	OperationStatus    int    `json:"operation_status"`
	IsHighRisk         uint8  `json:"is_high_risk"`
	RequestHeaders     string `json:"request_headers"`
	RequestQuery       string `json:"request_query"`
	RequestBody        string `json:"request_body"`
	ChangeDiff         string `json:"change_diff"`
	ResponseStatus     int    `json:"response_status"`
	ResponseBody       string `json:"response_body"`
	ResponseHeader     string `json:"response_header"`
	ExecutionTimeUs    int64  `json:"execution_time_us"`
	ExecutionTimeScope string `json:"execution_time_scope"`
}

// PersistAuditLog 将请求审计日志快照写入数据库。
func PersistAuditLog(snapshot *AuditLogSnapshot) error {
	if snapshot == nil || snapshot.RequestID == "" {
		return nil
	}

	requestLog := snapshotToRequestLog(snapshot)
	return requestLog.Create()
}

// PersistAuditLogsBatch 将一批请求审计日志快照批量写入数据库。
//
// 主要用于队列降级路径中聚合后的批量落库，避免每请求一次同步 INSERT 带来的写放大。
// - 自动过滤 nil / 缺少 RequestID 的快照。
// - 全部条目无效或 logs 为空时直接返回 nil。
// - 使用 GORM CreateInBatches 单批 100 条提交，DB 错误透传由调用方决定是否兜底。
func PersistAuditLogsBatch(ctx context.Context, snapshots []*AuditLogSnapshot) error {
	if len(snapshots) == 0 {
		return nil
	}

	rows := make([]*model.RequestLogs, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot == nil || snapshot.RequestID == "" {
			continue
		}
		rows = append(rows, snapshotToRequestLog(snapshot))
	}
	if len(rows) == 0 {
		return nil
	}

	db, err := model.GetDB()
	if err != nil {
		return err
	}
	if ctx != nil {
		db = db.WithContext(ctx)
	}
	return db.CreateInBatches(rows, 100).Error
}

// snapshotToRequestLog 将审计快照映射为持久化模型。
func snapshotToRequestLog(snapshot *AuditLogSnapshot) *model.RequestLogs {
	requestLog := model.NewRequestLogs()
	requestLog.RequestID = snapshot.RequestID
	requestLog.JwtID = snapshot.JwtID
	requestLog.OperatorID = snapshot.OperatorID
	requestLog.IP = snapshot.IP
	requestLog.UserAgent = snapshot.UserAgent
	requestLog.OS = snapshot.OS
	requestLog.Browser = snapshot.Browser
	requestLog.RequestKind = snapshot.RequestKind
	requestLog.Method = snapshot.Method
	requestLog.BaseURL = snapshot.BaseURL
	requestLog.OperationName = snapshot.OperationName
	requestLog.OperationStatus = snapshot.OperationStatus
	requestLog.IsHighRisk = snapshot.IsHighRisk
	requestLog.OperatorAccount = snapshot.OperatorAccount
	requestLog.OperatorName = snapshot.OperatorName
	requestLog.RequestHeaders = snapshot.RequestHeaders
	requestLog.RequestQuery = snapshot.RequestQuery
	requestLog.RequestBody = snapshot.RequestBody
	requestLog.ChangeDiff = snapshot.ChangeDiff
	requestLog.ResponseStatus = snapshot.ResponseStatus
	requestLog.ResponseBody = snapshot.ResponseBody
	requestLog.ResponseHeader = snapshot.ResponseHeader
	requestLog.ExecutionTimeUs = snapshot.ExecutionTimeUs
	requestLog.ExecutionTimeScope = snapshot.ExecutionTimeScope
	return requestLog
}
