package jobs

import (
	"context"
	"errors"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/service/notification"
	"go.uber.org/zap"
)

const (
	ExportQueueName = "export"
)

var ErrExportExecutionInProgress = errors.New("export execution is already in progress")

type exportRecordLifecycle interface {
	ClaimExecution(context.Context, uint, string, ...string) (*model.SysExportRecord, bool, error)
	CommitSuccess(context.Context, uint, service.ExportRecordSuccessInput) (*model.SysExportRecord, bool, error)
	CompletedRecordForTaskRun(context.Context, uint) (*model.SysExportRecord, error)
	MarkFailed(context.Context, uint, error, ...string) error
	MarkPendingForRetry(context.Context, uint, error, ...string) error
}

type generatedExportFileCleaner interface {
	Destroy(uint) error
}

func exportRecordUploadFile(record *model.SysExportRecord) *model.UploadFiles {
	if record == nil || record.FileID == 0 {
		return nil
	}
	file := model.NewUploadFiles()
	file.ID = record.FileID
	file.UUID = record.FileUUID
	file.OriginName = record.FileName
	file.Size = uint(record.FileSize)
	return file
}

func shouldFinalizeExportFailure(ctx context.Context) bool {
	retryCount, retryOK := asynq.GetRetryCount(ctx)
	maxRetry, maxRetryOK := asynq.GetMaxRetry(ctx)
	return isFinalExportAttempt(retryCount, maxRetry, retryOK && maxRetryOK)
}

func isFinalExportAttempt(retryCount, maxRetry int, hasRetryMetadata bool) bool {
	if !hasRetryMetadata {
		return true
	}
	return retryCount >= maxRetry
}

func beginExportExecution(ctx context.Context, lifecycle exportRecordLifecycle, taskRunID uint) (*model.SysExportRecord, bool, error) {
	executionToken := ExportExecutionToken(ctx, taskRunID)
	var previousTokens []string
	if taskID, ok := asynq.GetTaskID(ctx); ok && taskID != "" {
		if retryCount, ok := asynq.GetRetryCount(ctx); ok && retryCount > 0 {
			// 允许接管同一队列任务的旧尝试；升级前的 token 仅包含 Task ID。
			previousTokens = append(previousTokens, taskID)
			for attempt := 1; attempt <= retryCount; attempt++ {
				previousTokens = append(previousTokens, fmt.Sprintf("%s:attempt:%d", taskID, attempt))
			}
		}
	}
	record, claimed, err := lifecycle.ClaimExecution(ctx, taskRunID, executionToken, previousTokens...)
	if err != nil || claimed {
		return record, claimed, err
	}
	if record != nil && record.Status == model.TaskRunStatusSuccess {
		return record, false, nil
	}
	status := "missing"
	if record != nil {
		status = record.Status
	}
	return record, false, fmt.Errorf("%w: task_run_id=%d status=%s", ErrExportExecutionInProgress, taskRunID, status)
}

// ExportExecutionToken 标识单次尝试，阻止超时执行的迟到收尾覆盖下一次重试。
func ExportExecutionToken(ctx context.Context, taskRunID uint) string {
	taskID, ok := asynq.GetTaskID(ctx)
	if !ok || taskID == "" {
		return fmt.Sprintf("direct:%d", taskRunID)
	}
	retryCount, _ := asynq.GetRetryCount(ctx)
	return fmt.Sprintf("%s:attempt:%d", taskID, retryCount+1)
}

func finishExportAttemptFailure(ctx context.Context, lifecycle exportRecordLifecycle, taskRunID uint, taskErr error) error {
	var stateErr error
	executionToken := ExportExecutionToken(ctx, taskRunID)
	if shouldFinalizeExportFailure(ctx) {
		stateErr = lifecycle.MarkFailed(ctx, taskRunID, taskErr, executionToken)
	} else {
		stateErr = lifecycle.MarkPendingForRetry(ctx, taskRunID, taskErr, executionToken)
	}
	return errors.Join(taskErr, stateErr)
}

func commitGeneratedExport(
	ctx context.Context,
	lifecycle exportRecordLifecycle,
	cleaner generatedExportFileCleaner,
	taskRunID uint,
	input service.ExportRecordSuccessInput,
) (*model.SysExportRecord, bool, error) {
	record, committed, err := lifecycle.CommitSuccess(ctx, taskRunID, input)
	if err != nil {
		if input.File == nil || input.File.ID == 0 {
			return record, false, err
		}
		completed, lookupErr := lifecycle.CompletedRecordForTaskRun(ctx, taskRunID)
		if lookupErr != nil {
			// 提交结果无法确认时保留文件，避免把已写入导出记录的文件误删。
			log.Error("Export commit result lookup failed; generated file cleanup skipped",
				zap.Uint("task_run_id", taskRunID),
				zap.Uint("file_id", input.File.ID),
				zap.Error(lookupErr))
			return record, false, err
		}
		if completed != nil && completed.FileID == input.File.ID {
			return completed, false, err
		}
		cleanupGeneratedExportFile(cleaner, taskRunID, input.File.ID, exportRecordFileID(completed), "commit failure")
		return record, false, err
	}
	if committed || input.File == nil || input.File.ID == 0 || record == nil || record.FileID == input.File.ID {
		return record, committed, nil
	}
	cleanupGeneratedExportFile(cleaner, taskRunID, input.File.ID, record.FileID, "duplicate result")
	return record, false, nil
}

func exportRecordFileID(record *model.SysExportRecord) uint {
	if record == nil {
		return 0
	}
	return record.FileID
}

func updateExportProgress(
	ctx context.Context,
	exportService *service.ExportCenterService,
	userID uint,
	scene string,
	publicID string,
	taskRunID uint,
	processedCount uint64,
	totalCount uint64,
	started bool,
) error {
	var err error
	if !started {
		err = exportService.MarkRunningWithTotal(ctx, taskRunID, totalCount)
	} else {
		err = exportService.UpdateProgress(ctx, taskRunID, processedCount, totalCount)
	}
	if err != nil {
		return err
	}

	normalizedProcessed := processedCount
	if totalCount > 0 && normalizedProcessed > totalCount {
		normalizedProcessed = totalCount
	}
	progress := uint8(0)
	if totalCount > 0 {
		progress = uint8(normalizedProcessed * 100 / totalCount)
	}
	notification.NewService().PublishExportProgress(ctx, notification.ExportProgressInput{
		UserID:         userID,
		Scene:          scene,
		PublicID:       publicID,
		Status:         model.TaskRunStatusRunning,
		Progress:       progress,
		ProcessedCount: normalizedProcessed,
		TotalCount:     totalCount,
	})
	return nil
}

func cleanupGeneratedExportFile(cleaner generatedExportFileCleaner, taskRunID, fileID, winnerFileID uint, reason string) {
	if cleanupErr := cleaner.Destroy(fileID); cleanupErr != nil {
		// 导出文件自带 expires_at；即时清理失败时保留记录，后续过期清理任务会重试。
		log.Error("Export generated file cleanup failed",
			zap.Uint("task_run_id", taskRunID),
			zap.Uint("file_id", fileID),
			zap.Uint("winner_file_id", winnerFileID),
			zap.String("reason", reason),
			zap.Error(cleanupErr))
	}
}
