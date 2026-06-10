package resources

import (
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/cronexpr"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

// TaskDefinitionResources 任务定义响应结构。
type TaskDefinitionResources struct {
	ID                           uint             `json:"id"`
	Code                         string           `json:"code"`
	Name                         string           `json:"name"`
	Kind                         string           `json:"kind"`
	Queue                        string           `json:"queue"`
	CronSpec                     string           `json:"cron_spec"`
	CronSpecDescription          string           `json:"cron_spec_description"`
	Handler                      string           `json:"handler"`
	Status                       uint8            `json:"status"`
	AllowManual                  uint8            `json:"allow_manual"`
	AllowRetry                   uint8            `json:"allow_retry"`
	IsHighRisk                   uint8            `json:"is_high_risk"`
	RecordSuccessMode            string           `json:"record_success_mode"`
	RecordSuccessRate            uint8            `json:"record_success_rate"`
	RecordSuccessIntervalSeconds int              `json:"record_success_interval_seconds"`
	RecordDetailOnManual         uint8            `json:"record_detail_on_manual"`
	RecordDetailOnFailure        uint8            `json:"record_detail_on_failure"`
	Remark                       string           `json:"remark"`
	CreatedAt                    utils.FormatDate `json:"created_at"`
	UpdatedAt                    utils.FormatDate `json:"updated_at"`
}

func (r *TaskDefinitionResources) SetCustomFields(data *model.TaskDefinition) {
	if data == nil {
		return
	}
	r.CronSpecDescription = cronexpr.Describe(data.CronSpec)
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
	ID                uint              `json:"id"`
	TaskCode          string            `json:"task_code"`
	Kind              string            `json:"kind"`
	Source            string            `json:"source"`
	SourceID          string            `json:"source_id"`
	Queue             string            `json:"queue"`
	Status            string            `json:"status"`
	Attempt           int               `json:"attempt"`
	MaxRetry          int               `json:"max_retry"`
	RetryOfRunID      uint              `json:"retry_of_run_id"`
	RetryRootRunID    uint              `json:"retry_root_run_id"`
	RetrySeq          int               `json:"retry_seq"`
	ErrorMessage      string            `json:"error_message"`
	DurationMS        float64           `json:"duration_ms"`
	StartedAt         *utils.FormatDate `json:"started_at"`
	FinishedAt        *utils.FormatDate `json:"finished_at"`
	CreatedAt         utils.FormatDate  `json:"created_at"`
	TriggerUserID     uint              `json:"trigger_user_id"`
	TriggerAccount    string            `json:"trigger_account"`
	DetailRecordMode  string            `json:"detail_record_mode"`
	CanRetry          bool              `json:"can_retry"`
	RetryBlockReason  string            `json:"retry_block_reason"`
	RetryCount        int               `json:"retry_count"`
	LatestRetryRunID  uint              `json:"latest_retry_run_id"`
	LatestRetryStatus string            `json:"latest_retry_status"`
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

// TaskRunEventResources 任务执行事件响应。
type TaskRunEventResources struct {
	ID        uint             `json:"id"`
	RunID     uint             `json:"run_id"`
	EventType string           `json:"event_type"`
	Message   string           `json:"message"`
	Meta      string           `json:"meta"`
	CreatedAt utils.FormatDate `json:"created_at"`
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
		ID:                data.ID,
		TaskCode:          data.TaskCode,
		Kind:              data.Kind,
		Source:            data.Source,
		SourceID:          data.SourceID,
		Queue:             data.Queue,
		Status:            data.Status,
		Attempt:           data.Attempt,
		MaxRetry:          data.MaxRetry,
		RetryOfRunID:      data.RetryOfRunID,
		RetryRootRunID:    data.RetryRootRunID,
		RetrySeq:          data.RetrySeq,
		ErrorMessage:      data.ErrorMessage,
		DurationMS:        data.DurationMS,
		StartedAt:         data.StartedAt,
		FinishedAt:        data.FinishedAt,
		CreatedAt:         data.CreatedAt,
		TriggerUserID:     data.TriggerUserID,
		TriggerAccount:    data.TriggerAccount,
		DetailRecordMode:  data.DetailRecordMode,
		CanRetry:          data.CanRetry,
		RetryBlockReason:  data.RetryBlockReason,
		RetryCount:        data.RetryCount,
		LatestRetryRunID:  data.LatestRetryRunID,
		LatestRetryStatus: data.LatestRetryStatus,
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

// TaskRunEventTransformer 任务执行事件转换器。
type TaskRunEventTransformer struct {
	BaseResources[*model.TaskRunEvent, *TaskRunEventResources]
}

func NewTaskRunEventTransformer() TaskRunEventTransformer {
	return TaskRunEventTransformer{
		BaseResources: BaseResources[*model.TaskRunEvent, *TaskRunEventResources]{
			NewResource: func() *TaskRunEventResources {
				return &TaskRunEventResources{}
			},
		},
	}
}

// CronTaskStateResources 定时任务最近状态响应结构。
type CronTaskStateResources struct {
	ID                   uint              `json:"id"`
	TaskCode             string            `json:"task_code"`
	CronSpec             string            `json:"cron_spec"`
	CronSpecDescription  string            `json:"cron_spec_description"`
	LastRunID            uint              `json:"last_run_id"`
	LastStatus           string            `json:"last_status"`
	LastStartedAt        *utils.FormatDate `json:"last_started_at"`
	LastFinishedAt       *utils.FormatDate `json:"last_finished_at"`
	NextRunAt            *utils.FormatDate `json:"next_run_at"`
	LastSuccessAt        *utils.FormatDate `json:"last_success_at"`
	LastFailedAt         *utils.FormatDate `json:"last_failed_at"`
	LastStatsWindowStart *utils.FormatDate `json:"last_stats_window_start"`
	LastError            string            `json:"last_error"`
	UpdatedAt            utils.FormatDate  `json:"updated_at"`
}

func (r *CronTaskStateResources) SetCustomFields(data *model.CronTaskState) {
	if data == nil {
		return
	}
	r.CronSpecDescription = cronexpr.Describe(data.CronSpec)
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

// TaskRunStatsResources 任务执行全量统计响应结构。
type TaskRunStatsResources struct {
	TaskCode            string  `json:"task_code"`
	Kind                string  `json:"kind"`
	Source              string  `json:"source"`
	TotalCount          uint64  `json:"total_count"`
	SuccessCount        uint64  `json:"success_count"`
	FailedCount         uint64  `json:"failed_count"`
	CanceledCount       uint64  `json:"canceled_count"`
	TimeoutCount        uint64  `json:"timeout_count"`
	InterruptedCount    uint64  `json:"interrupted_count"`
	SampledSuccessCount uint64  `json:"sampled_success_count"`
	DurationTotalMS     float64 `json:"duration_total_ms"`
	DurationMaxMS       float64 `json:"duration_max_ms"`
	DurationAvgMS       float64 `json:"duration_avg_ms"`
	SuccessRate         float64 `json:"success_rate"`
}

// TaskRunTrendPointResources 任务执行统计趋势单点。
type TaskRunTrendPointResources struct {
	WindowStart         utils.FormatDate `json:"window_start"`
	WindowSize          string           `json:"window_size"`
	TotalCount          uint64           `json:"total_count"`
	SuccessCount        uint64           `json:"success_count"`
	FailedCount         uint64           `json:"failed_count"`
	CanceledCount       uint64           `json:"canceled_count"`
	TimeoutCount        uint64           `json:"timeout_count"`
	InterruptedCount    uint64           `json:"interrupted_count"`
	SampledSuccessCount uint64           `json:"sampled_success_count"`
	DurationTotalMS     float64          `json:"duration_total_ms"`
	DurationMaxMS       float64          `json:"duration_max_ms"`
}
