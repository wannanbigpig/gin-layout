package taskcenter

import (
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	"go.uber.org/zap"
)

// TaskCenterService 任务中心查询服务。
type TaskCenterService struct {
	service.Base
	db *gorm.DB
}

// TaskCenterServiceDeps 描述 TaskCenterService 可注入依赖。
type TaskCenterServiceDeps struct {
	DB *gorm.DB
}

func NewTaskCenterService() *TaskCenterService {
	return NewTaskCenterServiceWithDeps(TaskCenterServiceDeps{})
}

func NewTaskCenterServiceWithDeps(deps TaskCenterServiceDeps) *TaskCenterService {
	return &TaskCenterService{db: deps.DB}
}

// ListTaskDefinitions 分页查询任务定义列表。
func (s *TaskCenterService) ListTaskDefinitions(params *form.TaskDefinitionList) *resources.Collection {
	query := newListQuery().
		addLike("code", params.Code).
		addLike("name", params.Name).
		addEq("kind", params.Kind).
		addEq("status", params.Status).
		addEq("allow_manual", params.AllowManual).
		addEq("allow_retry", params.AllowRetry).
		addEq("is_high_risk", params.IsHighRisk)
	condition, args := query.Build()

	definitionModel := model.NewTaskDefinition()
	listOptionalParams := model.ListOptionalParams{
		SelectFields: []string{
			"id",
			"code",
			"name",
			"kind",
			"queue",
			"cron_spec",
			"handler",
			"status",
			"allow_manual",
			"allow_retry",
			"is_high_risk",
			"remark",
			"created_at",
			"updated_at",
		},
		OrderBy: "id DESC",
	}

	transformer := resources.NewTaskDefinitionTransformer()
	total, collection, err := model.ListPageE(definitionModel, params.Page, params.PerPage, condition, args, listOptionalParams)
	if err != nil {
		log.Logger.Error("查询任务定义列表失败", zap.Error(err))
		return transformer.ToCollection(params.Page, params.PerPage, 0, nil)
	}
	return transformer.ToCollection(params.Page, params.PerPage, total, collection)
}

// ListTaskRuns 分页查询任务执行记录列表。
func (s *TaskCenterService) ListTaskRuns(params *form.TaskRunList) *resources.Collection {
	query := newListQuery().
		addLike("task_code", params.TaskCode).
		addEq("kind", params.Kind).
		addEq("source", params.Source).
		addLike("source_id", params.SourceID).
		addEq("status", params.Status).
		addCreatedAtRange(params.StartTime, params.EndTime)
	condition, args := query.Build()

	runModel := model.NewTaskRun()
	listOptionalParams := model.ListOptionalParams{
		SelectFields: []string{
			"id",
			"task_code",
			"kind",
			"source",
			"source_id",
			"queue",
			"status",
			"attempt",
			"max_retry",
			"error_message",
			"duration_ms",
			"started_at",
			"finished_at",
			"created_at",
			"trigger_user_id",
			"trigger_account",
		},
		OrderBy: "created_at DESC, id DESC",
	}

	transformer := resources.NewTaskRunTransformer()
	total, collection, err := model.ListPageE(runModel, params.Page, params.PerPage, condition, args, listOptionalParams)
	if err != nil {
		log.Logger.Error("查询任务执行记录列表失败", zap.Error(err))
		return transformer.ToCollection(params.Page, params.PerPage, 0, nil)
	}
	return transformer.ToCollection(params.Page, params.PerPage, total, collection)
}

// TaskRunDetail 获取任务执行记录详情。
func (s *TaskCenterService) TaskRunDetail(id uint) (any, error) {
	taskRun := model.NewTaskRun()
	if s.db != nil {
		taskRun.SetDB(s.db)
	}
	if err := taskRun.GetById(id); err != nil || taskRun.ID == 0 {
		return nil, e.NewBusinessError(e.NotFound)
	}
	return resources.NewTaskRunTransformer().ToStruct(taskRun), nil
}

// TaskRunEvents 查询任务执行事件列表。
func (s *TaskCenterService) TaskRunEvents(runID uint) ([]*resources.TaskRunEventResources, error) {
	eventModel := model.NewTaskRunEvent()
	if s.db != nil {
		var events []*model.TaskRunEvent
		err := s.db.Select([]string{
			"id",
			"run_id",
			"event_type",
			"message",
			"meta",
			"created_at",
		}).Where("run_id = ?", runID).Order("created_at ASC, id ASC").Find(&events).Error
		if err != nil {
			log.Logger.Error("查询任务执行事件失败", zap.Error(err), zap.Uint("run_id", runID))
			return nil, err
		}
		return buildTaskRunEventResources(events), nil
	}
	listOptionalParams := model.ListOptionalParams{
		SelectFields: []string{"id", "run_id", "event_type", "message", "meta", "created_at"},
		OrderBy:      "created_at ASC, id ASC",
	}

	events, err := model.ListE(eventModel, "run_id = ?", []any{runID}, listOptionalParams)
	if err != nil {
		log.Logger.Error("查询任务执行事件失败", zap.Error(err), zap.Uint("run_id", runID))
		return nil, err
	}
	return buildTaskRunEventResources(events), nil
}

func buildTaskRunEventResources(events []*model.TaskRunEvent) []*resources.TaskRunEventResources {
	transformer := resources.NewTaskRunEventTransformer()
	response := make([]*resources.TaskRunEventResources, 0, len(events))
	for _, item := range events {
		response = append(response, transformer.ToStruct(item))
	}
	return response
}

// TaskRunAuditSnapshot 查询任务执行记录的审计快照（用于构建 change_diff）。
func (s *TaskCenterService) TaskRunAuditSnapshot(id uint) (*TaskRunAuditSnapshot, error) {
	taskRun, err := loadTaskRunByID(id)
	if err != nil || taskRun == nil || taskRun.ID == 0 {
		return nil, e.NewBusinessError(e.NotFound)
	}
	return &TaskRunAuditSnapshot{
		RunID:     taskRun.ID,
		TaskCode:  taskRun.TaskCode,
		Status:    taskRun.Status,
		Queue:     taskRun.Queue,
		SourceID:  taskRun.SourceID,
		Kind:      taskRun.Kind,
		MaxRetry:  taskRun.MaxRetry,
		Attempt:   taskRun.Attempt,
		HasRecord: true,
	}, nil
}

// ListCronTaskStates 分页查询定时任务最近状态列表。
func (s *TaskCenterService) ListCronTaskStates(params *form.CronTaskStateList) *resources.Collection {
	query := newListQuery().
		addLike("task_code", params.TaskCode).
		addEq("last_status", params.LastStatus)
	condition, args := query.Build()

	stateModel := model.NewCronTaskState()
	listOptionalParams := model.ListOptionalParams{
		SelectFields: []string{
			"id",
			"task_code",
			"cron_spec",
			"last_run_id",
			"last_status",
			"last_started_at",
			"last_finished_at",
			"next_run_at",
			"last_error",
			"updated_at",
		},
		OrderBy: "updated_at DESC, id DESC",
	}

	transformer := resources.NewCronTaskStateTransformer()
	total, collection, err := model.ListPageE(stateModel, params.Page, params.PerPage, condition, args, listOptionalParams)
	if err != nil {
		log.Logger.Error("查询定时任务最近状态列表失败", zap.Error(err))
		return transformer.ToCollection(params.Page, params.PerPage, 0, nil)
	}
	return transformer.ToCollection(params.Page, params.PerPage, total, collection)
}
