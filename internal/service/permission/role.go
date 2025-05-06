package permission

import (
	"strings"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// RoleService 角色服务
type RoleService struct {
	service.Base
}

func (s RoleService) List(params *form.RoleList) interface{} {
	var condition strings.Builder
	var args []any
	if params.Name != "" {
		condition.WriteString("name like ? AND ")
		args = append(args, "%"+params.Name+"%")
	}

	if params.Status != nil {
		condition.WriteString("status = ? AND ")
		args = append(args, params.Status)
	}

	conditionStr := condition.String()
	if conditionStr != "" {
		conditionStr = strings.TrimSuffix(condition.String(), "AND ")
	}

	RoleModel := model.NewRole()
	ListOptionalParams := model.ListOptionalParams{
		OrderBy: "sort desc, id desc",
	}

	total, collection := model.ListPage[model.Role](RoleModel, params.Page, params.PerPage, conditionStr, args, ListOptionalParams)
	return resources.ToRawCollection(params.Page, params.PerPage, total, collection)
}

func NewRoleService() *RoleService {
	return &RoleService{}
}
