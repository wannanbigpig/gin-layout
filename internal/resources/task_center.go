package resources

import (
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

// TaskDefinitionResources 任务定义响应结构。
type TaskDefinitionResources struct {
	ID          uint             `json:"id"`
	Code        string           `json:"code"`
	Name        string           `json:"name"`
	Kind        string           `json:"kind"`
	Queue       string           `json:"queue"`
	CronSpec    string           `json:"cron_spec"`
	Handler     string           `json:"handler"`
	Status      uint8            `json:"status"`
	AllowManual uint8            `json:"allow_manual"`
	AllowRetry  uint8            `json:"allow_retry"`
	IsHighRisk  uint8            `json:"is_high_risk"`
	Remark      string           `json:"remark"`
	CreatedAt   utils.FormatDate `json:"created_at"`
	UpdatedAt   utils.FormatDate `json:"updated_at"`
}

// TaskDefinitionTransformer 任务定义资源转换器。
type TaskDefinitionTransformer struct {
	BaseResources[*model.TaskDefinition, *TaskDefinitionResources]
}

func NewTaskDefinitionTransformer() TaskDefinitionTransformer {
	return TaskDefinitionTransformer{
		BaseResources: BaseResources[*model.TaskDefinition, *TaskDefinitionResources]{
			NewResource: func() *TaskDefinitionResources {
				return &TaskDefinitionResources{}
			},
		},
	}
}

// TaskRunBaseResources 任务执行记录公共字段。
type TaskRunBaseResources struct {
	ID             uint              `json:"id"`
	TaskCode       string            `json:"task_code"`
	Kind           string            `json:"kind"`
	Source         string            `json:"source"`
	SourceID       string            `json:"source_id"`
	Queue          string            `json:"queue"`
	Status         string            `json:"status"`
	Attempt        int               `json:"attempt"`
	MaxRetry       int               `json:"max_retry"`
	ErrorMessage   string            `json:"error_message"`
	DurationMS     float64           `json:"duration_ms"`
	StartedAt      *utils.FormatDate `json:"started_at"`
	FinishedAt     *utils.FormatDate `json:"finished_at"`
	CreatedAt      utils.FormatDate  `json:"created_at"`
	TriggerUserID  uint              `json:"trigger_user_id"`
	TriggerAccount string            `json:"trigger_account"`
}

// TaskRunListResources 任务执行记录列表响应。
type TaskRunListResources struct {
	TaskRunBaseResources
}

// TaskRunResources 任务执行记录详情响应。
type TaskRunResources struct {
	TaskRunBaseResources
	Payload   string           `json:"payload"`
	UpdatedAt utils.FormatDate `json:"updated_at"`
}

// TaskRunTransformer 任务执行记录资源转换器。
type TaskRunTransformer struct {
	BaseResources[*model.TaskRun, *TaskRunResources]
}

func NewTaskRunTransformer() TaskRunTransformer {
	return TaskRunTransformer{
		BaseResources: BaseResources[*model.TaskRun, *TaskRunResources]{
			NewResource: func() *TaskRunResources {
				return &TaskRunResources{}
			},
		},
	}
}

func buildTaskRunBaseResources(data *model.TaskRun) TaskRunBaseResources {
	return TaskRunBaseResources{
		ID:             data.ID,
		TaskCode:       data.TaskCode,
		Kind:           data.Kind,
		Source:         data.Source,
		SourceID:       data.SourceID,
		Queue:          data.Queue,
		Status:         data.Status,
		Attempt:        data.Attempt,
		MaxRetry:       data.MaxRetry,
		ErrorMessage:   data.ErrorMessage,
		DurationMS:     data.DurationMS,
		StartedAt:      data.StartedAt,
		FinishedAt:     data.FinishedAt,
		CreatedAt:      data.CreatedAt,
		TriggerUserID:  data.TriggerUserID,
		TriggerAccount: data.TriggerAccount,
	}
}

func (r TaskRunTransformer) ToStruct(data *model.TaskRun) *TaskRunResources {
	base := buildTaskRunBaseResources(data)
	return &TaskRunResources{
		TaskRunBaseResources: base,
		Payload:              data.Payload,
		UpdatedAt:            data.UpdatedAt,
	}
}

func (r TaskRunTransformer) ToCollection(page, perPage int, total int64, data []*model.TaskRun) *Collection {
	response := make([]any, 0, len(data))
	for _, v := range data {
		base := buildTaskRunBaseResources(v)
		response = append(response, &TaskRunListResources{
			TaskRunBaseResources: base,
		})
	}
	return NewCollection().SetPaginate(page, perPage, total).ToCollection(response)
}

// CronTaskStateResources 定时任务最近状态响应结构。
type CronTaskStateResources struct {
	ID             uint              `json:"id"`
	TaskCode       string            `json:"task_code"`
	CronSpec       string            `json:"cron_spec"`
	LastRunID      uint              `json:"last_run_id"`
	LastStatus     string            `json:"last_status"`
	LastStartedAt  *utils.FormatDate `json:"last_started_at"`
	LastFinishedAt *utils.FormatDate `json:"last_finished_at"`
	NextRunAt      *utils.FormatDate `json:"next_run_at"`
	LastError      string            `json:"last_error"`
	UpdatedAt      utils.FormatDate  `json:"updated_at"`
}

// CronTaskStateTransformer 定时任务状态资源转换器。
type CronTaskStateTransformer struct {
	BaseResources[*model.CronTaskState, *CronTaskStateResources]
}

func NewCronTaskStateTransformer() CronTaskStateTransformer {
	return CronTaskStateTransformer{
		BaseResources: BaseResources[*model.CronTaskState, *CronTaskStateResources]{
			NewResource: func() *CronTaskStateResources {
				return &CronTaskStateResources{}
			},
		},
	}
}
