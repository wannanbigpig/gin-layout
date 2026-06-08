package taskcron

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"go.uber.org/zap"
)

const (
	auditLogBatchSize         = 1000
	auditLogMinRetentionDays  = 7
	auditLogDefaultRetention  = 30
)

var cleanAuditLogsGetDB = model.GetDB

// CleanAuditLogsHandler 清理超过保留天数的 request_logs 和 admin_login_logs 记录。
func CleanAuditLogsHandler(ctx context.Context, _ map[string]any) error {
	retentionDays := getAuditLogRetentionDays()
	log.Logger.Info("开始执行清理审计日志任务", zap.Int("retention_days", retentionDays))

	db, err := cleanAuditLogsGetDB()
	if err != nil {
		return err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	db = db.WithContext(ctx)

	cutoffExpr := buildCutoffExpr(db, retentionDays)

	requestLogDeleted, err := batchDeleteByCutoff(db, &model.RequestLogs{}, "request_logs", cutoffExpr, ctx, false)
	if err != nil {
		log.Logger.Error("清理 request_logs 失败", zap.Error(err))
		return err
	}

	loginLogDeleted, err := batchDeleteByCutoff(db, &model.AdminLoginLogs{}, "admin_login_logs", cutoffExpr, ctx, true)
	if err != nil {
		log.Logger.Error("清理 admin_login_logs 失败", zap.Error(err))
		return err
	}

	if requestLogDeleted == 0 && loginLogDeleted == 0 {
		log.Logger.Info("没有需要清理的过期审计日志")
		return nil
	}

	log.Logger.Info("清理审计日志完成",
		zap.Int64("request_logs_deleted", requestLogDeleted),
		zap.Int64("admin_login_logs_deleted", loginLogDeleted),
		zap.Int("retention_days", retentionDays),
	)
	return nil
}

// getAuditLogRetentionDays 从配置读取保留天数，带最小值保护。
func getAuditLogRetentionDays() int {
	cfg := config.GetConfig()
	days := cfg.Queue.AuditLogRetentionDays
	if days < auditLogMinRetentionDays {
		if days > 0 {
			log.Logger.Warn("审计日志保留天数低于最小值，使用最小值",
				zap.Int("configured", days), zap.Int("minimum", auditLogMinRetentionDays))
			return auditLogMinRetentionDays
		}
		return auditLogDefaultRetention
	}
	return days
}

// buildCutoffExpr 根据数据库方言构建截止时间表达式。
func buildCutoffExpr(db *gorm.DB, retentionDays int) interface{} {
	switch db.Dialector.Name() {
	case "sqlite":
		return gorm.Expr("datetime('now', '-' || ? || ' days')", retentionDays)
	default:
		// mysql
		return gorm.Expr("DATE_SUB(NOW(), INTERVAL ? DAY)", retentionDays)
	}
}

// batchDeleteByCutoff 分批删除指定表中 created_at 早于 cutoff 的记录。
// 先查询 ID 列表再按 ID 删除，避免 DELETE ... LIMIT 在 SQLite 下不兼容的问题。
func batchDeleteByCutoff(db *gorm.DB, modelPtr interface{}, tableName string, cutoff interface{}, ctx context.Context, physicalDelete bool) (int64, error) {
	var totalDeleted int64

	for {
		if ctxErr := ctx.Err(); ctxErr != nil {
			log.Logger.Warn("清理审计日志任务被取消", zap.String("table", tableName), zap.Error(ctxErr))
			return totalDeleted, ctxErr
		}

		var ids []uint
		queryDB := db.Model(modelPtr)
		if physicalDelete {
			queryDB = queryDB.Unscoped()
		}
		err := queryDB.
			Where("created_at < ?", cutoff).
			Limit(auditLogBatchSize).
			Pluck("id", &ids).Error
		if err != nil {
			return totalDeleted, fmt.Errorf("query %s ids: %w", tableName, err)
		}

		if len(ids) == 0 {
			break
		}

		deleteDB := db.Where("id IN ?", ids)
		if physicalDelete {
			deleteDB = deleteDB.Unscoped()
		}
		result := deleteDB.Delete(modelPtr)
		if result.Error != nil {
			return totalDeleted, fmt.Errorf("delete %s: %w", tableName, result.Error)
		}
		totalDeleted += result.RowsAffected

		if len(ids) < auditLogBatchSize {
			break
		}

		// 短暂让出 CPU，避免长时间占据连接
		time.Sleep(100 * time.Millisecond)
	}

	return totalDeleted, nil
}
