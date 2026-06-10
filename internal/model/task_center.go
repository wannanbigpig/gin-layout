package model

import (
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

const (
	TaskKindAsync = "async"
	TaskKindCron  = "cron"

	TaskStatusEnabled  uint8 = 1
	TaskStatusDisabled uint8 = 0

	TaskManualAllowed    uint8 = 1
	TaskManualNotAllowed uint8 = 0

	TaskRetryAllowed    uint8 = 1
	TaskRetryNotAllowed uint8 = 0

	TaskHighRisk    uint8 = 1
	TaskNotHighRisk uint8 = 0

	TaskSourceQueue  = "queue"
	TaskSourceCron   = "cron"
	TaskSourceManual = "manual"

	TaskRunStatusPending     = "pending"
	TaskRunStatusRunning     = "running"
	TaskRunStatusSuccess     = "success"
	TaskRunStatusFailed      = "failed"
	TaskRunStatusCanceled    = "canceled"
	TaskRunStatusRetrying    = "retrying"
	TaskRunStatusTimeout     = "timeout"
	TaskRunStatusInterrupted = "interrupted"

	TaskEventEnqueue     = "enqueue"
	TaskEventStart       = "start"
	TaskEventRetry       = "retry"
	TaskEventFail        = "fail"
	TaskEventSuccess     = "success"
	TaskEventCancel      = "cancel"
	TaskEventTimeout     = "timeout"
	TaskEventInterrupted = "interrupted"

	TaskRecordSuccessAll      = "all"
	TaskRecordSuccessSample   = "sample"
	TaskRecordSuccessInterval = "interval"
	TaskRecordSuccessNone     = "none"

	TaskDetailRecordAll     = "all"
	TaskDetailRecordSampled = "sampled"
	TaskDetailRecordForced  = "forced"
	TaskDetailRecordNone    = "none"

	TaskRunStatsWindowHour = "hour"
	TaskRunStatsWindowDay  = "day"
)

// TaskDefinition 描述一个可被后台管理识别的任务。
type TaskDefinition struct {
	ContainsDeleteBaseModel
	Code                         string `json:"code"`                            // 任务唯一编码
	Name                         string `json:"name"`                            // 任务名称
	Kind                         string `json:"kind"`                            // async/cron
	Queue                        string `json:"queue"`                           // 队列名称
	CronSpec                     string `json:"cron_spec"`                       // Cron 表达式
	Handler                      string `json:"handler"`                         // 处理器标识
	Status                       uint8  `json:"status"`                          // 状态 1启用 0停用
	AllowManual                  uint8  `json:"allow_manual"`                    // 是否允许手动触发
	AllowRetry                   uint8  `json:"allow_retry"`                     // 是否允许手动重试
	IsHighRisk                   uint8  `json:"is_high_risk"`                    // 是否高危任务
	RecordSuccessMode            string `json:"record_success_mode"`             // 成功明细记录策略 all/sample/interval/none
	RecordSuccessRate            uint8  `json:"record_success_rate"`             // 成功采样比例 0-100
	RecordSuccessIntervalSeconds int    `json:"record_success_interval_seconds"` // 成功明细最小记录间隔
	RecordDetailOnManual         uint8  `json:"record_detail_on_manual"`         // 手动触发是否强制记录明细
	RecordDetailOnFailure        uint8  `json:"record_detail_on_failure"`        // 失败类结果是否强制记录明细
	Remark                       string `json:"remark"`                          // 备注
}

func NewTaskDefinition() *TaskDefinition {
	return BindModel(&TaskDefinition{})
}

func (m *TaskDefinition) TableName() string {
	return "task_definitions"
}

// TaskRun 表示一次任务执行记录。
type TaskRun struct {
	BaseModel
	TaskCode          string            `json:"task_code"`          // 任务唯一编码
	Kind              string            `json:"kind"`               // async/cron
	Source            string            `json:"source"`             // queue/cron/manual
	SourceID          string            `json:"source_id"`          // 来源任务ID
	Queue             string            `json:"queue"`              // 队列名称
	TriggerUserID     uint              `json:"trigger_user_id"`    // 触发人ID
	TriggerAccount    string            `json:"trigger_account"`    // 触发人账号
	Status            string            `json:"status"`             // 执行状态
	Attempt           int               `json:"attempt"`            // 当前尝试次数
	MaxRetry          int               `json:"max_retry"`          // 最大重试次数
	RetryOfRunID      uint              `json:"retry_of_run_id"`    // 直接重试来源执行记录ID
	RetryRootRunID    uint              `json:"retry_root_run_id"`  // 重试链路根执行记录ID
	RetrySeq          int               `json:"retry_seq"`          // 重试链路内人工重试序号
	Payload           string            `json:"payload"`            // 任务 payload
	ErrorMessage      string            `json:"error_message"`      // 失败原因
	StartedAt         *utils.FormatDate `json:"started_at"`         // 开始时间
	FinishedAt        *utils.FormatDate `json:"finished_at"`        // 结束时间
	DurationMS        float64           `json:"duration_ms"`        // 执行耗时毫秒
	DetailRecordMode  string            `json:"detail_record_mode"` // 明细记录模式
	CanRetry          bool              `gorm:"-" json:"can_retry"` // 当前执行记录是否允许手动重试
	RetryBlockReason  string            `gorm:"-" json:"retry_block_reason"`
	RetryCount        int               `gorm:"-" json:"retry_count"`         // 重试链路内已人工重试次数
	LatestRetryRunID  uint              `gorm:"-" json:"latest_retry_run_id"` // 最近一次人工重试执行记录ID
	LatestRetryStatus string            `gorm:"-" json:"latest_retry_status"` // 最近一次人工重试状态
}

func NewTaskRun() *TaskRun {
	return BindModel(&TaskRun{})
}

func (m *TaskRun) TableName() string {
	return "task_runs"
}

// TaskRunEvent 表示任务执行过程中的状态事件。
type TaskRunEvent struct {
	BaseModel
	RunID     uint   `json:"run_id"`     // 任务执行记录ID
	EventType string `json:"event_type"` // 事件类型
	Message   string `json:"message"`    // 事件说明
	Meta      string `json:"meta"`       // 事件元数据 JSON
}

func NewTaskRunEvent() *TaskRunEvent {
	return BindModel(&TaskRunEvent{})
}

func (m *TaskRunEvent) TableName() string {
	return "task_run_events"
}

// CronTaskState 保存定时任务最近一次执行状态。
type CronTaskState struct {
	BaseModel
	TaskCode             string            `json:"task_code"`               // 任务唯一编码
	CronSpec             string            `json:"cron_spec"`               // Cron 表达式
	LastRunID            uint              `json:"last_run_id"`             // 最近执行记录ID
	LastStatus           string            `json:"last_status"`             // 最近执行状态
	LastStartedAt        *utils.FormatDate `json:"last_started_at"`         // 最近开始时间
	LastFinishedAt       *utils.FormatDate `json:"last_finished_at"`        // 最近结束时间
	NextRunAt            *utils.FormatDate `json:"next_run_at"`             // 下次执行时间
	LastSuccessAt        *utils.FormatDate `json:"last_success_at"`         // 最近成功时间
	LastFailedAt         *utils.FormatDate `json:"last_failed_at"`          // 最近失败时间
	LastStatsWindowStart *utils.FormatDate `json:"last_stats_window_start"` // 最近统计窗口
	LastError            string            `json:"last_error"`              // 最近失败原因
}

func NewCronTaskState() *CronTaskState {
	return BindModel(&CronTaskState{})
}

func (m *CronTaskState) TableName() string {
	return "cron_task_states"
}

// TaskRunStat 保存任务执行的全量聚合统计。
type TaskRunStat struct {
	BaseModel
	TaskCode            string           `json:"task_code"`             // 任务唯一编码
	Kind                string           `json:"kind"`                  // async/cron
	Source              string           `json:"source"`                // queue/cron/manual
	WindowStart         utils.FormatDate `json:"window_start"`          // 统计窗口开始时间
	WindowSize          string           `json:"window_size"`           // hour/day 等窗口粒度
	TotalCount          uint64           `json:"total_count"`           // 总执行次数
	SuccessCount        uint64           `json:"success_count"`         // 成功次数
	FailedCount         uint64           `json:"failed_count"`          // 失败次数
	CanceledCount       uint64           `json:"canceled_count"`        // 取消次数
	TimeoutCount        uint64           `json:"timeout_count"`         // 超时次数
	InterruptedCount    uint64           `json:"interrupted_count"`     // 中断次数
	SampledSuccessCount uint64           `json:"sampled_success_count"` // 成功明细记录次数
	DurationTotalMS     float64          `json:"duration_total_ms"`     // 总耗时毫秒
	DurationMaxMS       float64          `json:"duration_max_ms"`       // 最大耗时毫秒
}

func NewTaskRunStat() *TaskRunStat {
	return BindModel(&TaskRunStat{})
}

func (m *TaskRunStat) TableName() string {
	return "task_run_stats"
}
