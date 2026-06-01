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
	auditsvc "github.com/wannanbigpig/gin-layout/internal/service/audit"
	"github.com/wannanbigpig/gin-layout/internal/service/notification"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	TypeExportRequestLog       = "export:request-log"
	ExportRequestLogQueue      = ExportQueueName
	exportRequestLogResultType = "result"
)

// ExportRequestLogPayload 表示异步导出请求日志任务 payload。
type ExportRequestLogPayload struct {
	TaskRunID          uint   `json:"task_run_id"`
	TriggerUserID      uint   `json:"trigger_user_id"`
	RequestID          string `json:"request_id"`
	OperatorID         uint   `json:"operator_id"`
	OperatorAccount    string `json:"operator_account"`
	OperationStatus    *int   `json:"operation_status"`
	IsHighRisk         *uint8 `json:"is_high_risk"`
	RequestKind        string `json:"request_kind"`
	ExecutionTimeScope string `json:"execution_time_scope"`
	Method             string `json:"method"`
	BaseURL            string `json:"base_url"`
	OperationName      string `json:"operation_name"`
	IP                 string `json:"ip"`
	StartTime          string `json:"start_time"`
	EndTime            string `json:"end_time"`
	Limit              int    `json:"limit"`
}

// handleExportRequestLog 处理异步导出请求日志任务。
// 任务状态生命周期由 worker 中间件管理（pending -> running -> success/failed），
// handler 只负责业务逻辑和写入结果。
func handleExportRequestLog(ctx context.Context, payload ExportRequestLogPayload) error {
	exportService := service.NewExportCenterService()

	params := &form.RequestLogExport{
		RequestID:          payload.RequestID,
		OperatorID:         payload.OperatorID,
		OperatorAccount:    payload.OperatorAccount,
		OperationStatus:    payload.OperationStatus,
		IsHighRisk:         payload.IsHighRisk,
		RequestKind:        payload.RequestKind,
		ExecutionTimeScope: payload.ExecutionTimeScope,
		Method:             payload.Method,
		BaseURL:            payload.BaseURL,
		OperationName:      payload.OperationName,
		IP:                 payload.IP,
		StartTime:          payload.StartTime,
		EndTime:            payload.EndTime,
		Limit:              payload.Limit,
	}

	// 使用 io.Pipe 将 StreamExportCSV 的 writer 端与 CreateFromReader 的 reader 端串接，
	// 避免在内存中一次性持有完整 CSV：内存峰值 ≈ batch × 单行大小，而不是导出行数 × 单行大小。
	pipeReader, pipeWriter := io.Pipe()
	progressStarted := false
	// fileName 在主 goroutine 内完成生成，作为 OriginName 传给 CreateFromReader，避免
	// 与 StreamExportCSV 返回值产生跨 goroutine 共享写。
	fileName := resolveRequestLogExportFileName()

	// writer 协程：流式生成 CSV，结束时通过 CloseWithError 将错误传递给 reader 端，
	// 这样下游 CreateFromReader 在读取过程中能立刻拿到原始错误而不是 io.EOF。
	streamErrCh := make(chan error, 1)
	go func() {
		_, err := auditsvc.StreamExportCSV(ctx, params, pipeWriter, func(processed, total int) error {
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

	fileService := service.NewFileResourceService()
	uploadFile, err := fileService.CreateFromReader(ctx, service.ServerGeneratedFileInput{
		UID:         payload.TriggerUserID,
		OriginName:  fileName,
		MimeType:    "text/csv; charset=utf-8",
		IsPublic:    0,
		UploadScene: "export",
		Reader:      pipeReader,
	})
	// reader 端关闭：成功路径下 io.Copy 已读到 EOF，Close 是幂等收尾；
	// 失败路径下 CloseWithError 会让仍在写入的 writer 收到 ErrClosedPipe 并退出，
	// 避免 writer 协程永久阻塞在 Write 上。
	if err != nil {
		_ = pipeReader.CloseWithError(err)
	} else {
		_ = pipeReader.Close()
	}
	// 等待 writer 协程结束并采集其错误：writer 端错误优先级最高，因为 reader 看到的
	// 可能是被 CloseWithError 注入的同一错误，也可能是 EOF。
	streamErr := <-streamErrCh
	if streamErr != nil {
		_ = exportService.MarkFailed(ctx, payload.TaskRunID, streamErr)
		notifyExportRequestLogMessage(ctx, payload, model.TaskRunStatusFailed, nil, "", streamErr)
		return streamErr
	}
	if err != nil {
		_ = exportService.MarkFailed(ctx, payload.TaskRunID, err)
		notifyExportRequestLogMessage(ctx, payload, model.TaskRunStatusFailed, nil, "", err)
		return err
	}

	if err := exportService.MarkSuccess(ctx, payload.TaskRunID, service.ExportRecordSuccessInput{
		File:     uploadFile,
		FileName: fileName,
	}); err != nil {
		_ = exportService.MarkFailed(ctx, payload.TaskRunID, err)
		notifyExportRequestLogMessage(ctx, payload, model.TaskRunStatusFailed, nil, fileName, err)
		return err
	}

	recordExportRequestLogResult(ctx, payload.TaskRunID, uploadFile, fileName)
	notifyExportRequestLogMessage(ctx, payload, model.TaskRunStatusSuccess, uploadFile, fileName, nil)

	return nil
}

func notifyExportRequestLogMessage(ctx context.Context, payload ExportRequestLogPayload, status string, uploadFile *model.UploadFiles, fileName string, taskErr error) {
	input := notification.ExportResultInput{
		UserID:       payload.TriggerUserID,
		Scene:        model.ExportSceneRequestLog,
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
		log.Error("Export request log: notify result failed",
			zap.Uint("task_run_id", payload.TaskRunID),
			zap.Error(err))
	}
}

func recordExportRequestLogResult(ctx context.Context, taskRunID uint, uploadFile *model.UploadFiles, fileName string) {
	if taskRunID == 0 || uploadFile == nil {
		return
	}
	db, err := model.GetDB()
	if err != nil {
		log.Error("Export request log: get db for result event failed",
			zap.Uint("task_run_id", taskRunID),
			zap.Error(err))
		return
	}
	if err := recordExportRequestLogResultWithDB(ctx, db, taskRunID, uploadFile, fileName); err != nil {
		log.Error("Export request log: write result event failed",
			zap.Uint("task_run_id", taskRunID),
			zap.Error(err))
	}
}

func recordExportRequestLogResultWithDB(ctx context.Context, db *gorm.DB, taskRunID uint, uploadFile *model.UploadFiles, fileName string) error {
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
		EventType: exportRequestLogResultType,
		Message:   "export file generated",
		Meta:      string(meta),
	}).Error
}

// resolveRequestLogExportFileName 在主 goroutine 中生成最终文件名。
// 与 StreamExportCSV 内部生成的命名规则保持一致，但在主 goroutine 内决定，避免
// 跨 goroutine 共享 fileName 字段引入数据竞争。
func resolveRequestLogExportFileName() string {
	return "request_logs_" + time.Now().Format("20060102150405") + ".csv"
}
