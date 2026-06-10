package resources

import (
	"strings"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

// ExportRecordBaseResources 导出记录公共字段。
type ExportRecordBaseResources struct {
	ID               uint              `json:"id"`
	ExportName       string            `json:"export_name"`
	Scene            string            `json:"scene"`
	SceneName        string            `json:"scene_name"`
	TaskCode         string            `json:"task_code"`
	TaskRunID        uint              `json:"task_run_id"`
	TriggerUserID    uint              `json:"trigger_user_id"`
	Status           string            `json:"status"`
	StatusName       string            `json:"status_name"`
	Stage            string            `json:"stage"`
	StageName        string            `json:"stage_name"`
	Queue            string            `json:"queue"`
	Progress         uint8             `json:"progress"`
	TotalCount       uint64            `json:"total_count"`
	ProcessedCount   uint64            `json:"processed_count"`
	FileID           uint              `json:"file_id"`
	FileUUID         string            `json:"file_uuid"`
	FileName         string            `json:"file_name"`
	FileSize         uint64            `json:"file_size"`
	FileURL          string            `json:"file_url"`
	DownloadURL      string            `json:"download_url"`
	HasFile          bool              `json:"has_file"`
	DownloadReady    bool              `json:"download_ready"`
	Format           string            `json:"format"`
	ErrorMessage     string            `json:"error_message"`
	FailReason       string            `json:"fail_reason"`
	CanRetry         bool              `json:"can_retry"`
	RetryBlockReason string            `json:"retry_block_reason"`
	StartedAt        *utils.FormatDate `json:"started_at"`
	FinishedAt       *utils.FormatDate `json:"finished_at"`
	CreatedAt        utils.FormatDate  `json:"created_at"`
}

// ExportRecordListResources 导出记录列表响应。
type ExportRecordListResources struct {
	ExportRecordBaseResources
}

// ExportRecordResources 导出记录详情响应。
type ExportRecordResources struct {
	ExportRecordBaseResources
	UpdatedAt utils.FormatDate `json:"updated_at"`
}

// ExportRecordTransformer 导出记录响应转换器。
type ExportRecordTransformer struct {
	BaseResources[*model.SysExportRecord, *ExportRecordResources]
}

func NewExportRecordTransformer() ExportRecordTransformer {
	return ExportRecordTransformer{
		BaseResources: BaseResources[*model.SysExportRecord, *ExportRecordResources]{
			NewResource: func() *ExportRecordResources {
				return &ExportRecordResources{}
			},
		},
	}
}

func buildExportRecordBaseResources(data *model.SysExportRecord) ExportRecordBaseResources {
	sceneName := exportSceneDisplayName(data.Scene)
	taskCode := exportSceneTaskCode(data.Scene)
	statusName := exportStatusDisplayName(data.Status)
	hasFile := exportRecordHasFile(data)
	stage, stageName := exportRecordStage(data.Status, hasFile)
	downloadURL := exportRecordDownloadURL(data)
	return ExportRecordBaseResources{
		ID:               data.ID,
		ExportName:       sceneName,
		Scene:            data.Scene,
		SceneName:        sceneName,
		TaskCode:         taskCode,
		TaskRunID:        data.TaskRunID,
		TriggerUserID:    data.TriggerUserID,
		Status:           data.Status,
		StatusName:       statusName,
		Stage:            stage,
		StageName:        stageName,
		Queue:            data.Queue,
		Progress:         data.Progress,
		TotalCount:       data.TotalCount,
		ProcessedCount:   data.ProcessedCount,
		FileID:           data.FileID,
		FileUUID:         data.FileUUID,
		FileName:         data.FileName,
		FileSize:         data.FileSize,
		FileURL:          downloadURL,
		DownloadURL:      downloadURL,
		HasFile:          hasFile,
		DownloadReady:    hasFile,
		Format:           data.Format,
		ErrorMessage:     data.ErrorMessage,
		FailReason:       strings.TrimSpace(data.ErrorMessage),
		CanRetry:         data.CanRetry,
		RetryBlockReason: data.RetryBlockReason,
		StartedAt:        data.StartedAt,
		FinishedAt:       data.FinishedAt,
		CreatedAt:        data.CreatedAt,
	}
}

// exportSceneDisplayName 将场景代码映射为中文显示名称，便于前端直接渲染。
func exportSceneDisplayName(scene string) string {
	switch scene {
	case model.ExportSceneRequestLog:
		return "请求日志"
	case model.ExportSceneFileList:
		return "文件资源"
	default:
		return ""
	}
}

func exportSceneTaskCode(scene string) string {
	switch scene {
	case model.ExportSceneRequestLog:
		return "export:request-log"
	case model.ExportSceneFileList:
		return "export:file-list"
	default:
		return ""
	}
}

func exportStatusDisplayName(status string) string {
	switch strings.TrimSpace(status) {
	case model.TaskRunStatusPending:
		return "等待执行"
	case model.TaskRunStatusRunning:
		return "执行中"
	case model.TaskRunStatusSuccess:
		return "导出成功"
	case model.TaskRunStatusFailed:
		return "导出失败"
	case model.TaskRunStatusCanceled:
		return "已取消"
	case model.TaskRunStatusRetrying:
		return "重试中"
	default:
		return strings.TrimSpace(status)
	}
}

func exportRecordStage(status string, hasFile bool) (string, string) {
	switch strings.TrimSpace(status) {
	case model.TaskRunStatusPending:
		return "queued", "排队中"
	case model.TaskRunStatusRetrying:
		return "queued", "重新排队中"
	case model.TaskRunStatusRunning:
		return "processing", "生成文件中"
	case model.TaskRunStatusSuccess:
		if hasFile {
			return "download_ready", "可下载"
		}
		return "completed", "已完成"
	case model.TaskRunStatusFailed:
		return "failed", "失败"
	case model.TaskRunStatusCanceled:
		return "canceled", "已取消"
	default:
		return "", ""
	}
}

func exportRecordHasFile(data *model.SysExportRecord) bool {
	if data == nil {
		return false
	}
	return data.FileID > 0 || strings.TrimSpace(data.FileUUID) != ""
}

func exportRecordDownloadURL(data *model.SysExportRecord) string {
	if data == nil {
		return ""
	}
	fileUUID := strings.TrimSpace(data.FileUUID)
	if fileUUID == "" {
		return ""
	}
	return buildFileResourceURL(fileUUID)
}

func (r ExportRecordTransformer) ToStruct(data *model.SysExportRecord) *ExportRecordResources {
	base := buildExportRecordBaseResources(data)
	return &ExportRecordResources{
		ExportRecordBaseResources: base,
		UpdatedAt:                 data.UpdatedAt,
	}
}

func (r ExportRecordTransformer) ToCollection(page, perPage int, total int64, data []*model.SysExportRecord) *Collection {
	response := make([]any, 0, len(data))
	for _, item := range data {
		response = append(response, &ExportRecordListResources{
			ExportRecordBaseResources: buildExportRecordBaseResources(item),
		})
	}
	return NewCollection().SetPaginate(page, perPage, total).ToCollection(response)
}
