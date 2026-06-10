package model

import "github.com/wannanbigpig/gin-layout/internal/pkg/utils"

const (
	ExportSceneRequestLog = "request_log"
	ExportSceneFileList   = "file_list"
)

// SysExportRecord 统一导出记录。
type SysExportRecord struct {
	BaseModel
	Scene            string            `json:"scene"`
	TaskRunID        uint              `json:"task_run_id"`
	TriggerUserID    uint              `json:"trigger_user_id"`
	Status           string            `json:"status"`
	Queue            string            `json:"queue"`
	Progress         uint8             `json:"progress"`
	TotalCount       uint64            `json:"total_count"`
	ProcessedCount   uint64            `json:"processed_count"`
	FileID           uint              `json:"file_id"`
	FileUUID         string            `json:"file_uuid"`
	FileName         string            `json:"file_name"`
	FileSize         uint64            `json:"file_size"`
	Format           string            `json:"format"`
	ErrorMessage     string            `json:"error_message"`
	StartedAt        *utils.FormatDate `json:"started_at"`
	FinishedAt       *utils.FormatDate `json:"finished_at"`
	CanRetry         bool              `gorm:"-" json:"can_retry"`
	RetryBlockReason string            `gorm:"-" json:"retry_block_reason"`
}

func NewSysExportRecord() *SysExportRecord {
	return BindModel(&SysExportRecord{})
}

func (m *SysExportRecord) TableName() string {
	return "sys_export_records"
}
