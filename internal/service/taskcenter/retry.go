package taskcenter

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
)

type retryChainSummary struct {
	MaxSeq            int
	SuccessCount      int64
	ActiveCount       int64
	LatestRetryRunID  uint
	LatestRetryStatus string
}

var loadTaskRetryChainSummary = loadTaskRetryChainSummaryFromDB

const exportManualRetryLimit = 3

func isTaskRunRetryableStatus(status string) bool {
	switch status {
	case model.TaskRunStatusFailed, model.TaskRunStatusTimeout, model.TaskRunStatusInterrupted:
		return true
	default:
		return false
	}
}

func manualRetryLimit(run *model.TaskRun) int {
	if run != nil && isExportTaskCode(run.TaskCode) {
		return exportManualRetryLimit
	}
	if run != nil && run.MaxRetry > 0 {
		return run.MaxRetry
	}
	return 1
}

func retryRootRunID(run *model.TaskRun) uint {
	if run == nil {
		return 0
	}
	if run.RetryRootRunID > 0 {
		return run.RetryRootRunID
	}
	return run.ID
}

func evaluateTaskRunRetryState(run *model.TaskRun, definition *model.TaskDefinition, summary retryChainSummary) (bool, string) {
	if run == nil || run.ID == 0 {
		return false, "执行记录不存在"
	}
	if !isTaskRunRetryableStatus(run.Status) {
		return false, "仅失败、超时或中断任务允许重试"
	}
	if definition == nil {
		return false, "任务定义不存在"
	}
	if definition.Status != model.TaskStatusEnabled {
		return false, "任务已停用，无法重试"
	}
	if definition.AllowRetry != model.TaskRetryAllowed {
		return false, "任务不允许重试"
	}
	if summary.ActiveCount > 0 {
		return false, "该任务已有重试记录正在执行或排队"
	}
	if summary.SuccessCount > 0 {
		return false, "该任务重试链路已成功，不允许继续重试"
	}
	limit := manualRetryLimit(run)
	if summary.MaxSeq >= limit {
		return false, fmt.Sprintf("已达到最大手动重试次数 %d", limit)
	}
	return true, ""
}

func loadTaskRetryChainSummaryFromDB(ctx context.Context, db *gorm.DB, rootRunID uint) (retryChainSummary, error) {
	if rootRunID == 0 {
		return retryChainSummary{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if db == nil {
		var err error
		db, err = model.GetDB()
		if err != nil {
			return retryChainSummary{}, err
		}
	}

	var summary retryChainSummary
	err := db.WithContext(ctx).
		Model(&model.TaskRun{}).
		Select(
			`COALESCE(MAX(retry_seq), 0) AS max_seq,
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS success_count,
			COALESCE(SUM(CASE WHEN status IN (?, ?, ?) THEN 1 ELSE 0 END), 0) AS active_count`,
			model.TaskRunStatusSuccess,
			model.TaskRunStatusPending,
			model.TaskRunStatusRunning,
			model.TaskRunStatusRetrying,
		).
		Where("retry_root_run_id = ?", rootRunID).
		Scan(&summary).Error
	return summary, err
}

func loadTaskRetryChainSummaries(ctx context.Context, db *gorm.DB, rootRunIDs []uint) (map[uint]retryChainSummary, error) {
	result := make(map[uint]retryChainSummary, len(rootRunIDs))
	if len(rootRunIDs) == 0 {
		return result, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if db == nil {
		var err error
		db, err = model.GetDB()
		if err != nil {
			return result, err
		}
	}

	type row struct {
		RetryRootRunID    uint
		MaxSeq            int
		SuccessCount      int64
		ActiveCount       int64
		LatestRetryRunID  uint
		LatestRetryStatus string
	}
	rows := make([]row, 0, len(rootRunIDs))
	err := db.WithContext(ctx).
		Model(&model.TaskRun{}).
		Select(
			`retry_root_run_id,
			COALESCE(MAX(retry_seq), 0) AS max_seq,
			COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS success_count,
			COALESCE(SUM(CASE WHEN status IN (?, ?, ?) THEN 1 ELSE 0 END), 0) AS active_count`,
			model.TaskRunStatusSuccess,
			model.TaskRunStatusPending,
			model.TaskRunStatusRunning,
			model.TaskRunStatusRetrying,
		).
		Where("retry_root_run_id IN ?", rootRunIDs).
		Group("retry_root_run_id").
		Scan(&rows).Error
	if err != nil {
		return result, err
	}
	for i := range rows {
		if rows[i].MaxSeq <= 0 {
			continue
		}
		var latest model.TaskRun
		if err := db.WithContext(ctx).
			Select("id", "status").
			Where("retry_root_run_id = ? AND retry_seq = ?", rows[i].RetryRootRunID, rows[i].MaxSeq).
			Order("id DESC").
			First(&latest).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				continue
			}
			return result, err
		}
		rows[i].LatestRetryRunID = latest.ID
		rows[i].LatestRetryStatus = latest.Status
	}
	for _, item := range rows {
		result[item.RetryRootRunID] = retryChainSummary{
			MaxSeq:            item.MaxSeq,
			SuccessCount:      item.SuccessCount,
			ActiveCount:       item.ActiveCount,
			LatestRetryRunID:  item.LatestRetryRunID,
			LatestRetryStatus: item.LatestRetryStatus,
		}
	}
	return result, nil
}
