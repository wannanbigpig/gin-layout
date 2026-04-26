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

	TaskRunStatusPending  = "pending"
	TaskRunStatusRunning  = "running"
	TaskRunStatusSuccess  = "success"
	TaskRunStatusFailed   = "failed"
	TaskRunStatusCanceled = "canceled"
	TaskRunStatusRetrying = "retrying"

	TaskEventEnqueue = "enqueue"
	TaskEventStart   = "start"
	TaskEventRetry   = "retry"
	TaskEventFail    = "fail"
	TaskEventSuccess = "success"
	TaskEventCancel  = "cancel"
)

// TaskDefinition 描述一个可被后台管理识别的任务。
type TaskDefinition struct {
	ContainsDeleteBaseModel
	Code        string `json:"code"`         // 任务唯一编码
	Name        string `json:"name"`         // 任务名称
	Kind        string `json:"kind"`         // async/cron
	Queue       string `json:"queue"`        // 队列名称
	CronSpec    string `json:"cron_spec"`    // Cron 表达式
	Handler     string `json:"handler"`      // 处理器标识
	Status      uint8  `json:"status"`       // 状态 1启用 0停用
	AllowManual uint8  `json:"allow_manual"` // 是否允许手动触发
	AllowRetry  uint8  `json:"allow_retry"`  // 是否允许手动重试
	IsHighRisk  uint8  `json:"is_high_risk"` // 是否高危任务
	Remark      string `json:"remark"`       // 备注
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
	TaskCode       string            `json:"task_code"`       // 任务唯一编码
	Kind           string            `json:"kind"`            // async/cron
	Source         string            `json:"source"`          // queue/cron/manual
	SourceID       string            `json:"source_id"`       // 来源任务ID
	Queue          string            `json:"queue"`           // 队列名称
	TriggerUserID  uint              `json:"trigger_user_id"` // 触发人ID
	TriggerAccount string            `json:"trigger_account"` // 触发人账号
	Status         string            `json:"status"`          // 执行状态
	Attempt        int               `json:"attempt"`         // 当前尝试次数
	MaxRetry       int               `json:"max_retry"`       // 最大重试次数
	Payload        string            `json:"payload"`         // 任务 payload
	ErrorMessage   string            `json:"error_message"`   // 失败原因
	StartedAt      *utils.FormatDate `json:"started_at"`      // 开始时间
	FinishedAt     *utils.FormatDate `json:"finished_at"`     // 结束时间
	DurationMS     float64           `json:"duration_ms"`     // 执行耗时毫秒
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
	TaskCode       string            `json:"task_code"`        // 任务唯一编码
	CronSpec       string            `json:"cron_spec"`        // Cron 表达式
	LastRunID      uint              `json:"last_run_id"`      // 最近执行记录ID
	LastStatus     string            `json:"last_status"`      // 最近执行状态
	LastStartedAt  *utils.FormatDate `json:"last_started_at"`  // 最近开始时间
	LastFinishedAt *utils.FormatDate `json:"last_finished_at"` // 最近结束时间
	NextRunAt      *utils.FormatDate `json:"next_run_at"`      // 下次执行时间
	LastError      string            `json:"last_error"`       // 最近失败原因
}

func NewCronTaskState() *CronTaskState {
	return BindModel(&CronTaskState{})
}

func (m *CronTaskState) TableName() string {
	return "cron_task_states"
}
