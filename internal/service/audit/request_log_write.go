package audit

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	requestLogTextMaxBytes = 65535
	truncatedAuditSuffix   = "...(truncated)"
)

// AuditLogSnapshot 表示请求结束时提取出的审计日志快照。
type AuditLogSnapshot struct {
	OccurredAt         time.Time `json:"occurred_at"`
	RequestID          string    `json:"request_id"`
	JwtID              string    `json:"jwt_id"`
	OperatorID         uint      `json:"operator_id"`
	OperatorAccount    string    `json:"operator_account"`
	OperatorName       string    `json:"operator_name"`
	IP                 string    `json:"ip"`
	UserAgent          string    `json:"user_agent"`
	OS                 string    `json:"os"`
	Browser            string    `json:"browser"`
	RequestKind        string    `json:"request_kind"`
	Method             string    `json:"method"`
	BaseURL            string    `json:"base_url"`
	OperationName      string    `json:"operation_name"`
	OperationStatus    int       `json:"operation_status"`
	IsHighRisk         uint8     `json:"is_high_risk"`
	RequestHeaders     string    `json:"request_headers"`
	RequestQuery       string    `json:"request_query"`
	RequestBody        string    `json:"request_body"`
	ChangeDiff         string    `json:"change_diff"`
	ResponseStatus     int       `json:"response_status"`
	ResponseBody       string    `json:"response_body"`
	ResponseHeader     string    `json:"response_header"`
	ExecutionTimeUs    int64     `json:"execution_time_us"`
	ExecutionTimeScope string    `json:"execution_time_scope"`
}

// PersistAuditLog 将请求审计日志快照写入数据库。
func PersistAuditLog(snapshot *AuditLogSnapshot) error {
	return PersistAuditLogContext(context.Background(), snapshot)
}

// PersistAuditLogContext 在调用方给定的时间预算内写入单条请求审计日志。
func PersistAuditLogContext(ctx context.Context, snapshot *AuditLogSnapshot) error {
	if snapshot == nil || snapshot.RequestID == "" {
		return nil
	}

	db, err := model.GetDB()
	if err != nil {
		return err
	}
	return persistAuditLogWithDB(ctx, db, snapshot)
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
	return persistAuditLogsBatchWithDB(ctx, db, rows)
}

func persistAuditLogWithDB(ctx context.Context, db *gorm.DB, snapshot *AuditLogSnapshot) error {
	if db == nil || snapshot == nil || snapshot.RequestID == "" {
		return nil
	}
	if ctx != nil {
		db = db.WithContext(ctx)
	}
	return db.Clauses(requestLogIdempotentUpsert()).Create(snapshotToRequestLog(snapshot)).Error
}

func persistAuditLogsBatchWithDB(ctx context.Context, db *gorm.DB, rows []*model.RequestLogs) error {
	if db == nil || len(rows) == 0 {
		return nil
	}
	if ctx != nil {
		db = db.WithContext(ctx)
	}
	// 项目全局关闭了 GORM 默认写事务。CreateInBatches 超过 100 条时会分批执行，
	// 必须在此显式包装事务，避免前一子批已提交、后一子批失败后整批重试导致重复审计。
	return db.Transaction(func(tx *gorm.DB) error {
		return tx.Clauses(requestLogIdempotentUpsert()).CreateInBatches(rows, 100).Error
	})
}

// requestLogIdempotentUpsert 以请求 ID 作为审计事实唯一键。
// 队列在“数据库提交成功、ACK 前进程退出”时可能重投同一任务；此处更新同一行，
// 避免多 worker/重试把一次 HTTP 请求写成多条个人或系统日志。
func requestLogIdempotentUpsert() clause.OnConflict {
	return clause.OnConflict{
		Columns: []clause.Column{{Name: "request_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"jwt_id", "operator_id", "ip", "user_agent", "os", "browser",
			"request_kind", "method", "base_url", "operation_name", "operation_status",
			"is_high_risk", "operator_account", "operator_name", "request_headers",
			"request_query", "request_body", "change_diff", "response_status",
			"response_body", "response_header", "execution_time_us",
			"execution_time_scope", "created_at", "updated_at",
		}),
	}
}

// snapshotToRequestLog 将审计快照映射为持久化模型。
func snapshotToRequestLog(snapshot *AuditLogSnapshot) *model.RequestLogs {
	requestLog := model.NewRequestLogs()
	if !snapshot.OccurredAt.IsZero() {
		requestLog.CreatedAt = utils.FormatDate{Time: snapshot.OccurredAt.In(time.Local)}
	}
	requestLog.RequestID = truncateAuditRunes(snapshot.RequestID, 64)
	requestLog.JwtID = truncateAuditRunes(snapshot.JwtID, 36)
	requestLog.OperatorID = snapshot.OperatorID
	requestLog.IP = truncateAuditRunes(snapshot.IP, 45)
	requestLog.UserAgent = truncateAuditRunes(snapshot.UserAgent, 1024)
	requestLog.OS = truncateAuditRunes(snapshot.OS, 50)
	requestLog.Browser = truncateAuditRunes(snapshot.Browser, 50)
	requestLog.RequestKind = truncateAuditRunes(snapshot.RequestKind, 16)
	requestLog.Method = truncateAuditRunes(snapshot.Method, 10)
	requestLog.BaseURL = truncateAuditRunes(snapshot.BaseURL, 160)
	requestLog.OperationName = truncateAuditRunes(snapshot.OperationName, 255)
	requestLog.OperationStatus = snapshot.OperationStatus
	requestLog.IsHighRisk = snapshot.IsHighRisk
	requestLog.OperatorAccount = truncateAuditRunes(snapshot.OperatorAccount, 50)
	requestLog.OperatorName = truncateAuditRunes(snapshot.OperatorName, 50)
	requestLog.RequestHeaders = truncateAuditBytes(snapshot.RequestHeaders, requestLogTextMaxBytes)
	requestLog.RequestQuery = truncateAuditBytes(snapshot.RequestQuery, requestLogTextMaxBytes)
	requestLog.RequestBody = truncateAuditBytes(snapshot.RequestBody, requestLogTextMaxBytes)
	requestLog.ChangeDiff = snapshot.ChangeDiff
	requestLog.ResponseStatus = snapshot.ResponseStatus
	requestLog.ResponseBody = truncateAuditBytes(snapshot.ResponseBody, requestLogTextMaxBytes)
	requestLog.ResponseHeader = truncateAuditBytes(snapshot.ResponseHeader, requestLogTextMaxBytes)
	requestLog.ExecutionTimeUs = snapshot.ExecutionTimeUs
	requestLog.ExecutionTimeScope = truncateAuditRunes(snapshot.ExecutionTimeScope, 16)
	return requestLog
}

// truncateAuditRunes 按 MySQL varchar 的字符上限裁剪，并把非法 UTF-8 归一化为替代字符。
func truncateAuditRunes(value string, maxRunes int) string {
	if maxRunes <= 0 || value == "" {
		return ""
	}
	value = strings.ToValidUTF8(value, "�")
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	return string(runes[:maxRunes])
}

// truncateAuditBytes 按 MySQL TEXT 的字节上限裁剪，保留合法 UTF-8 并附加截断标记。
func truncateAuditBytes(value string, maxBytes int) string {
	if maxBytes <= 0 || value == "" {
		return ""
	}
	value = strings.ToValidUTF8(value, "�")
	if len(value) <= maxBytes {
		return value
	}
	limit := maxBytes - len(truncatedAuditSuffix)
	if limit <= 0 {
		return truncateAuditRunes(truncatedAuditSuffix, maxBytes)
	}
	for limit > 0 && !utf8.ValidString(value[:limit]) {
		limit--
	}
	return value[:limit] + truncatedAuditSuffix
}
