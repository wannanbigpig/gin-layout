package admin_v1

import (
	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/middleware"
	"github.com/wannanbigpig/gin-layout/internal/service/taskcenter"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// TaskCenterController 任务中心控制器。
type TaskCenterController struct {
	controller.Api
}

func NewTaskCenterController() *TaskCenterController {
	return &TaskCenterController{}
}

// TaskList 分页查询任务定义列表。
func (api TaskCenterController) TaskList(c *gin.Context) {
	params := form.NewTaskDefinitionListQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}

	result, err := taskcenter.NewTaskCenterService().ListTaskDefinitions(params)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

// RunList 分页查询任务执行记录列表。
func (api TaskCenterController) RunList(c *gin.Context) {
	params := form.NewTaskRunListQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}

	result, err := taskcenter.NewTaskCenterService().ListTaskRuns(params)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

// RunStats 查询任务执行全量统计汇总。
func (api TaskCenterController) RunStats(c *gin.Context) {
	params := form.NewTaskRunStatsQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}

	result, err := taskcenter.NewTaskCenterService().TaskRunStats(params)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

// RunStatsTrend 查询任务执行统计趋势。
func (api TaskCenterController) RunStatsTrend(c *gin.Context) {
	params := form.NewTaskRunStatsQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}

	result, err := taskcenter.NewTaskCenterService().TaskRunStatsTrend(params)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

// RunDetail 查询任务执行记录详情。
func (api TaskCenterController) RunDetail(c *gin.Context) {
	query := form.NewIdForm()
	if err := validator.CheckQueryParams(c, &query); err != nil {
		return
	}

	detail, err := taskcenter.NewTaskCenterService().TaskRunDetail(query.ID)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, detail)
}

// RunEvents 查询任务执行事件列表。
func (api TaskCenterController) RunEvents(c *gin.Context) {
	query := form.NewTaskRunEventsQuery()
	if err := validator.CheckQueryParams(c, &query); err != nil {
		return
	}

	events, err := taskcenter.NewTaskCenterService().TaskRunEvents(query.RunID)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, events)
}

// CronStateList 分页查询定时任务最近状态列表。
func (api TaskCenterController) CronStateList(c *gin.Context) {
	params := form.NewCronTaskStateListQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}

	result, err := taskcenter.NewTaskCenterService().ListCronTaskStates(params)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

// RecordPolicy 更新任务执行明细记录策略。
func (api TaskCenterController) RecordPolicy(c *gin.Context) {
	params := form.NewTaskRecordPolicyForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	result, err := taskcenter.NewTaskCenterService().UpdateTaskRecordPolicy(c.Request.Context(), params)
	if err != nil {
		api.Err(c, err)
		return
	}
	middleware.SetAuditChangeDiffRaw(c, taskcenter.BuildRecordPolicyAuditDiff(params))
	api.Success(c, result)
}

// OperationConfig 更新任务运营配置。
func (api TaskCenterController) OperationConfig(c *gin.Context) {
	params := form.NewTaskOperationConfigForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	result, err := taskcenter.NewTaskCenterService().UpdateTaskOperationConfig(c.Request.Context(), params)
	if err != nil {
		api.Err(c, err)
		return
	}
	middleware.SetAuditChangeDiffRaw(c, taskcenter.BuildOperationConfigAuditDiff(params))
	api.Success(c, result)
}

// Trigger 手动触发任务。
func (api TaskCenterController) Trigger(c *gin.Context) {
	params := form.NewTaskTriggerForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	uid := api.GetCurrentUserID(c)
	account := ""
	if user := api.GetCurrentAdminUserSnapshot(c); user != nil {
		account = user.Username
	}

	service := taskcenter.NewTaskCenterService()
	result, err := service.TriggerTask(c.Request.Context(), params, uid, account)
	if err != nil {
		api.Err(c, err)
		return
	}
	middleware.SetAuditChangeDiffRaw(c, taskcenter.BuildTriggerAuditDiff(params, result))
	api.Success(c, result)
}

// Retry 重试失败任务。
func (api TaskCenterController) Retry(c *gin.Context) {
	params := form.NewTaskRetryForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	uid := api.GetCurrentUserID(c)
	account := ""
	if user := api.GetCurrentAdminUserSnapshot(c); user != nil {
		account = user.Username
	}

	service := taskcenter.NewTaskCenterService()
	before, _ := service.TaskRunAuditSnapshot(params.RunID)
	result, err := service.RetryTask(c.Request.Context(), params.RunID, uid, account)
	if err != nil {
		api.Err(c, err)
		return
	}
	middleware.SetAuditChangeDiffRaw(c, taskcenter.BuildRetryAuditDiff(before, result))
	api.Success(c, result)
}

// Cancel 取消任务。
func (api TaskCenterController) Cancel(c *gin.Context) {
	params := form.NewTaskCancelForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}

	uid := api.GetCurrentUserID(c)
	account := ""
	if user := api.GetCurrentAdminUserSnapshot(c); user != nil {
		account = user.Username
	}

	service := taskcenter.NewTaskCenterService()
	before, _ := service.TaskRunAuditSnapshot(params.RunID)
	result, err := service.CancelTask(c.Request.Context(), params.RunID, uid, account, params.Reason)
	if err != nil {
		api.Err(c, err)
		return
	}
	middleware.SetAuditChangeDiffRaw(c, taskcenter.BuildCancelAuditDiff(before, result))
	api.Success(c, result)
}
