package jobs

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/service/notification"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	TypeExportFileList       = "export:file-list"
	ExportFileListQueue      = ExportQueueName
	exportFileListResultType = "result"

	// exportFileListMaxLimit 上限 50000，避免任务跑飞。
	exportFileListMaxLimit = 50000
)

// ExportFileListPayload 表示异步导出文件资源任务 payload。
// 字段来自 form.FileResourceList 的过滤条件。
type ExportFileListPayload struct {
	TaskRunID        uint   `json:"task_run_id"`
	TriggerUserID    uint   `json:"trigger_user_id"`
	OriginName       string `json:"origin_name"`
	UUID             string `json:"uuid"`
	MimeType         string `json:"mime_type"`
	FileType         string `json:"file_type"`
	IsPublic         *uint8 `json:"is_public"`
	FolderID         *uint  `json:"folder_id"`
	IncludeSubfolder uint8  `json:"include_subfolder"`
	IncludeHidden    *uint8 `json:"include_hidden"`
	StorageDriver    string `json:"storage_driver"`
	StorageStatus    string `json:"storage_status"`
	IsReferenced     *uint8 `json:"is_referenced"`
	IsDeleted        *uint8 `json:"is_deleted"`
	UID              uint   `json:"uid"`
	StartTime        string `json:"start_time"`
	EndTime          string `json:"end_time"`
	Limit            int    `json:"limit"`
}

// handleExportFileList 处理异步导出文件资源任务。
// 任务状态生命周期由 worker 中间件管理（pending -> running -> success/failed），
// handler 只负责业务逻辑和写入结果。
func handleExportFileList(ctx context.Context, payload ExportFileListPayload) error {
	exportService := service.NewExportCenterService()
	if completed, err := exportService.CompletedRecordForTaskRun(ctx, payload.TaskRunID); err != nil {
		return err
	} else if completed != nil {
		notifyExportFileListMessage(ctx, payload, model.TaskRunStatusSuccess, exportRecordUploadFile(completed), completed.FileName, nil)
		return nil
	}

	limit := payload.Limit
	if limit > exportFileListMaxLimit {
		limit = exportFileListMaxLimit
	}

	params := &form.FileResourceList{
		OriginName:       payload.OriginName,
		UUID:             payload.UUID,
		MimeType:         payload.MimeType,
		FileType:         payload.FileType,
		IsPublic:         payload.IsPublic,
		FolderID:         payload.FolderID,
		IncludeSubfolder: payload.IncludeSubfolder,
		IncludeHidden:    payload.IncludeHidden,
		StorageDriver:    payload.StorageDriver,
		StorageStatus:    payload.StorageStatus,
		IsReferenced:     payload.IsReferenced,
		IsDeleted:        payload.IsDeleted,
		UID:              payload.UID,
		StartTime:        payload.StartTime,
		EndTime:          payload.EndTime,
		Limit:            limit,
	}

	fileService := service.NewFileResourceService()

	pipeReader, pipeWriter := io.Pipe()
	fileName := "file_resources_" + time.Now().Format("20060102150405") + ".csv"
	progressStarted := false
	streamErrCh := make(chan error, 1)
	go func() {
		err := fileService.StreamFileListExportCSV(ctx, params, pipeWriter, func(processed, total int) error {
			if !progressStarted {
				progressStarted = true
				return exportService.MarkRunningWithTotal(ctx, payload.TaskRunID, uint64(total))
			}
			return exportService.UpdateProgress(ctx, payload.TaskRunID, uint64(processed), uint64(total))
		})
		if err != nil {
			_ = pipeWriter.CloseWithError(err)
			streamErrCh <- err
			return
		}
		_ = pipeWriter.Close()
		streamErrCh <- nil
	}()
	uploadFile, err := fileService.CreateFromReader(ctx, service.ServerGeneratedFileInput{
		UID:         payload.TriggerUserID,
		OriginName:  fileName,
		MimeType:    "text/csv; charset=utf-8",
		IsPublic:    0,
		UploadScene: "export",
		Reader:      pipeReader,
	})
	if err != nil {
		_ = pipeReader.CloseWithError(err)
	} else {
		_ = pipeReader.Close()
	}
	streamErr := <-streamErrCh
	if streamErr != nil {
		if shouldFinalizeExportFailure(ctx) {
			_ = exportService.MarkFailed(ctx, payload.TaskRunID, streamErr)
			notifyExportFileListMessage(ctx, payload, model.TaskRunStatusFailed, nil, "", streamErr)
		}
		return streamErr
	}
	if err != nil {
		if shouldFinalizeExportFailure(ctx) {
			_ = exportService.MarkFailed(ctx, payload.TaskRunID, err)
			notifyExportFileListMessage(ctx, payload, model.TaskRunStatusFailed, nil, "", err)
		}
		return err
	}

	if err := exportService.MarkSuccess(ctx, payload.TaskRunID, service.ExportRecordSuccessInput{
		File:     uploadFile,
		FileName: fileName,
	}); err != nil {
		if shouldFinalizeExportFailure(ctx) {
			_ = exportService.MarkFailed(ctx, payload.TaskRunID, err)
			notifyExportFileListMessage(ctx, payload, model.TaskRunStatusFailed, nil, fileName, err)
		}
		return err
	}

	recordExportFileListResult(ctx, payload.TaskRunID, uploadFile, fileName)
	notifyExportFileListMessage(ctx, payload, model.TaskRunStatusSuccess, uploadFile, fileName, nil)

	return nil
}

func notifyExportFileListMessage(ctx context.Context, payload ExportFileListPayload, status string, uploadFile *model.UploadFiles, fileName string, taskErr error) {
	input := notification.ExportResultInput{
		UserID:       payload.TriggerUserID,
		Scene:        model.ExportSceneFileList,
		Status:       status,
		TaskRunID:    payload.TaskRunID,
		FileName:     fileName,
		ErrorMessage: "",
	}
	if uploadFile != nil {
		input.FileID = uploadFile.ID
		input.FileUUID = uploadFile.UUID
		if strings.TrimSpace(input.FileName) == "" {
			input.FileName = uploadFile.DisplayName
		}
	}
	if taskErr != nil {
		input.ErrorMessage = taskErr.Error()
	}
	if _, err := notification.NewService().NotifyExportResult(ctx, input); err != nil {
		log.Error("Export file list: notify result failed",
			zap.Uint("task_run_id", payload.TaskRunID),
			zap.Error(err))
	}
}

func recordExportFileListResult(ctx context.Context, taskRunID uint, uploadFile *model.UploadFiles, fileName string) {
	if taskRunID == 0 || uploadFile == nil {
		return
	}
	db, err := model.GetDB()
	if err != nil {
		log.Error("Export file list: get db for result event failed",
			zap.Uint("task_run_id", taskRunID),
			zap.Error(err))
		return
	}
	if err := recordExportFileListResultWithDB(ctx, db, taskRunID, uploadFile, fileName); err != nil {
		log.Error("Export file list: write result event failed",
			zap.Uint("task_run_id", taskRunID),
			zap.Error(err))
	}
}

func recordExportFileListResultWithDB(ctx context.Context, db *gorm.DB, taskRunID uint, uploadFile *model.UploadFiles, fileName string) error {
	if taskRunID == 0 || uploadFile == nil || db == nil {
		return nil
	}
	meta, err := json.Marshal(map[string]any{
		"file_id":   uploadFile.ID,
		"file_name": fileName,
		"file_size": uploadFile.Size,
		"file_uuid": uploadFile.UUID,
	})
	if err != nil {
		return err
	}
	return db.WithContext(ctx).Create(&model.TaskRunEvent{
		RunID:     taskRunID,
		EventType: exportFileListResultType,
		Message:   "export file generated",
		Meta:      string(meta),
	}).Error
}
