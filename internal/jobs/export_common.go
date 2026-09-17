package jobs

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/wannanbigpig/gin-layout/internal/model"
)

const (
	ExportQueueName = "export"
)

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
