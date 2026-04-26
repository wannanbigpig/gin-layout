package form

// TaskDefinitionList 任务定义列表查询参数。
type TaskDefinitionList struct {
	Paginate
	Code        string `form:"code" json:"code" binding:"omitempty"`
	Name        string `form:"name" json:"name" binding:"omitempty"`
	Kind        string `form:"kind" json:"kind" binding:"omitempty,oneof=async cron"`
	Status      *uint8 `form:"status" json:"status" binding:"omitempty,oneof=0 1"`
	AllowManual *uint8 `form:"allow_manual" json:"allow_manual" binding:"omitempty,oneof=0 1"`
	AllowRetry  *uint8 `form:"allow_retry" json:"allow_retry" binding:"omitempty,oneof=0 1"`
	IsHighRisk  *uint8 `form:"is_high_risk" json:"is_high_risk" binding:"omitempty,oneof=0 1"`
}

func NewTaskDefinitionListQuery() *TaskDefinitionList {
	return &TaskDefinitionList{}
}

// TaskRunList 任务执行记录列表查询参数。
type TaskRunList struct {
	Paginate
	TaskCode  string `form:"task_code" json:"task_code" binding:"omitempty"`
	Kind      string `form:"kind" json:"kind" binding:"omitempty,oneof=async cron"`
	Source    string `form:"source" json:"source" binding:"omitempty,oneof=queue cron manual"`
	SourceID  string `form:"source_id" json:"source_id" binding:"omitempty"`
	Status    string `form:"status" json:"status" binding:"omitempty"`
	StartTime string `form:"start_time" json:"start_time" binding:"omitempty"`
	EndTime   string `form:"end_time" json:"end_time" binding:"omitempty"`
}

func NewTaskRunListQuery() *TaskRunList {
	return &TaskRunList{}
}

// CronTaskStateList 定时任务状态列表查询参数。
type CronTaskStateList struct {
	Paginate
	TaskCode   string `form:"task_code" json:"task_code" binding:"omitempty"`
	LastStatus string `form:"last_status" json:"last_status" binding:"omitempty"`
}

func NewCronTaskStateListQuery() *CronTaskStateList {
	return &CronTaskStateList{}
}

// TaskTriggerForm 手动触发任务参数。
type TaskTriggerForm struct {
	TaskCode string         `form:"task_code" json:"task_code" binding:"required"`
	Queue    string         `form:"queue" json:"queue" binding:"omitempty"`
	TaskID   string         `form:"task_id" json:"task_id" binding:"omitempty"`
	Payload  map[string]any `form:"payload" json:"payload" binding:"omitempty"`
}

func NewTaskTriggerForm() *TaskTriggerForm {
	return &TaskTriggerForm{}
}

// TaskRetryForm 重试任务参数。
type TaskRetryForm struct {
	RunID uint `form:"run_id" json:"run_id" binding:"required,gt=0"`
}

func NewTaskRetryForm() *TaskRetryForm {
	return &TaskRetryForm{}
}

// TaskCancelForm 取消任务参数。
type TaskCancelForm struct {
	RunID uint `form:"run_id" json:"run_id" binding:"required,gt=0"`
}

func NewTaskCancelForm() *TaskCancelForm {
	return &TaskCancelForm{}
}
