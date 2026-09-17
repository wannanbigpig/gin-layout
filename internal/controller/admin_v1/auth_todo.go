package admin_v1

import (
	"context"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/controller"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service/todo"
	"github.com/wannanbigpig/gin-layout/internal/validator"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

type userTodoService interface {
	List(context.Context, uint, *form.UserTodoList) (*resources.Collection, error)
	Summary(context.Context, uint) (*resources.UserTodoSummaryResources, error)
	CreateManual(context.Context, uint, *form.UserTodoCreate) (*model.UserTodo, error)
	UpdateManual(context.Context, uint, *form.UserTodoUpdate) (*model.UserTodo, error)
	UpdateStatus(context.Context, uint, uint, string) (*model.UserTodo, error)
	DeleteManual(context.Context, uint, uint) error
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
	result, err := newUserTodoService().List(c.Request.Context(), api.GetCurrentUserID(c), params)
	if err != nil {
		api.Err(c, err)
		return
	}
	api.Success(c, result)
}

func (api UserTodoController) Summary(c *gin.Context) {
	result, err := newUserTodoService().Summary(c.Request.Context(), api.GetCurrentUserID(c))
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
