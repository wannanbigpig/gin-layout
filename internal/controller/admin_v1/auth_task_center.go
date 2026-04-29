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

	result := taskcenter.NewTaskCenterService().ListTaskDefinitions(params)
	api.Success(c, result)
}

// RunList 分页查询任务执行记录列表。
func (api TaskCenterController) RunList(c *gin.Context) {
	params := form.NewTaskRunListQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}

	result := taskcenter.NewTaskCenterService().ListTaskRuns(params)
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

// CronStateList 分页查询定时任务最近状态列表。
func (api TaskCenterController) CronStateList(c *gin.Context) {
	params := form.NewCronTaskStateListQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}

	result := taskcenter.NewTaskCenterService().ListCronTaskStates(params)
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
