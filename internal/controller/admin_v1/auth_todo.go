package admin_v1

import (
	"context"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service/todo"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

type userTodoService interface {
	List(context.Context, uint, *form.UserTodoList) (*resources.Collection, error)
	Summary(context.Context, uint) (*resources.UserTodoSummaryResources, error)
	SummaryInGroup(context.Context, uint, *uint) (*resources.UserTodoSummaryResources, error)
	CreateManual(context.Context, uint, *form.UserTodoCreate) (*model.UserTodo, error)
	UpdateManual(context.Context, uint, *form.UserTodoUpdate) (*model.UserTodo, error)
	UpdateStatus(context.Context, uint, uint, string) (*model.UserTodo, error)
	DeleteManual(context.Context, uint, uint) error
	ListGroups(context.Context, uint) ([]*model.UserTodoGroup, error)
	CreateGroup(context.Context, uint, string) (*model.UserTodoGroup, error)
	RenameGroup(context.Context, uint, uint, string) (*model.UserTodoGroup, error)
	DeleteGroup(context.Context, uint, uint) error
	MoveToGroup(context.Context, uint, uint, uint) (*model.UserTodo, error)
	GetReminderPreference(context.Context, uint) (*todo.ReminderPreference, error)
	UpdateReminderPreference(context.Context, uint, bool, bool, *int) (*todo.ReminderPreference, error)
}

var newUserTodoService = func() userTodoService {
	return todo.NewService()
}

type UserTodoController struct {
	controller.Api
}

func NewUserTodoController() *UserTodoController {
	return &UserTodoController{}
}

func (api UserTodoController) List(c *gin.Context) {
	params := form.NewUserTodoListQuery()
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}
	svc := newUserTodoService()
	userID := api.GetCurrentUserID(c)
	var result *resources.Collection
	var summary *resources.UserTodoSummaryResources
	var summaryErr error
	var group errgroup.Group
	group.Go(func() error {
		var err error
		result, err = svc.List(c.Request.Context(), userID, params)
		return err
	})
	group.Go(func() error {
		summary, summaryErr = svc.SummaryInGroup(c.Request.Context(), userID, params.GroupID)
		return nil
	})
	if err := group.Wait(); err != nil {
		api.Err(c, err)
		return
	}
	if summaryErr != nil {
		log.Logger.Error("todo list summary failed", zap.Uint("user_id", userID), zap.Error(summaryErr))
	}
	api.Success(c, &resources.UserTodoListResources{Collection: result, Summary: summary})
}

func (api UserTodoController) Summary(c *gin.Context) {
	params := &form.UserTodoSummaryQuery{}
	if err := validator.CheckQueryParams(c, &params); err != nil {
		return
	}
	result, err := newUserTodoService().SummaryInGroup(c.Request.Context(), api.GetCurrentUserID(c), params.GroupID)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

func (api UserTodoController) Create(c *gin.Context) {
	params := form.NewUserTodoCreateForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	item, err := newUserTodoService().CreateManual(c.Request.Context(), api.GetCurrentUserID(c), params)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, resources.NewUserTodoTransformer().ToStruct(item))
}

func (api UserTodoController) Update(c *gin.Context) {
	params := form.NewUserTodoUpdateForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	item, err := newUserTodoService().UpdateManual(c.Request.Context(), api.GetCurrentUserID(c), params)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, resources.NewUserTodoTransformer().ToStruct(item))
}

func (api UserTodoController) Status(c *gin.Context) {
	params := form.NewUserTodoStatusForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	item, err := newUserTodoService().UpdateStatus(c.Request.Context(), api.GetCurrentUserID(c), params.ID, params.Status)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, resources.NewUserTodoTransformer().ToStruct(item))
}

func (api UserTodoController) Delete(c *gin.Context) {
	params := form.NewUserTodoDeleteForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	if err := newUserTodoService().DeleteManual(c.Request.Context(), api.GetCurrentUserID(c), params.ID); err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, nil)
}

func (api UserTodoController) Groups(c *gin.Context) {
	groups, err := newUserTodoService().ListGroups(c.Request.Context(), api.GetCurrentUserID(c))
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, map[string]any{"groups": resources.NewUserTodoGroupCollection(groups)})
}

func (api UserTodoController) CreateGroup(c *gin.Context) {
	params := &form.UserTodoGroupCreate{}
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	group, err := newUserTodoService().CreateGroup(c.Request.Context(), api.GetCurrentUserID(c), params.Name)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, resources.NewUserTodoGroupResource(group))
}

func (api UserTodoController) RenameGroup(c *gin.Context) {
	params := &form.UserTodoGroupRename{}
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	group, err := newUserTodoService().RenameGroup(c.Request.Context(), api.GetCurrentUserID(c), params.ID, params.Name)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, resources.NewUserTodoGroupResource(group))
}

func (api UserTodoController) DeleteGroup(c *gin.Context) {
	params := &form.UserTodoGroupID{}
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	if err := newUserTodoService().DeleteGroup(c.Request.Context(), api.GetCurrentUserID(c), params.ID); err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, nil)
}

func (api UserTodoController) MoveGroup(c *gin.Context) {
	params := &form.UserTodoMoveGroup{}
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	item, err := newUserTodoService().MoveToGroup(c.Request.Context(), api.GetCurrentUserID(c), params.ID, params.GroupID)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, resources.NewUserTodoTransformer().ToStruct(item))
}

func (api UserTodoController) ReminderPreference(c *gin.Context) {
	result, err := newUserTodoService().GetReminderPreference(c.Request.Context(), api.GetCurrentUserID(c))
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

func (api UserTodoController) UpdateReminderPreference(c *gin.Context) {
	params := form.NewUserTodoReminderPreferenceForm()
	if err := validator.CheckPostParams(c, &params); err != nil {
		return
	}
	result, err := newUserTodoService().UpdateReminderPreference(
		c.Request.Context(), api.GetCurrentUserID(c), *params.InAppEnabled, *params.SMSEnabled, params.LeadMinutes,
	)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}
