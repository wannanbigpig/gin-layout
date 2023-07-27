package admin_auth

import (
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	"strings"
)

// PermissionService 登录授权服务
type PermissionService struct {
	service.Base
}

func NewPermissionService() *PermissionService {
	return &PermissionService{}
}

func (s *PermissionService) Edit(params *form.EditPermission) error {
	permissionModel := model.NewPermission()
	data := map[string]any{
		"name":    params.Name,
		"desc":    params.Desc,
		"is_auth": params.IsAuth,
		"sort":    params.Sort,
	}
	if params.Id > 0 {
		return permissionModel.Update(params.Id, data)
	}
	data["func"] = params.Func
	data["func_path"] = params.FuncPath
	data["method"] = params.Method
	data["route"] = params.Route
	count, err := permissionModel.HasRoute(params.Route)
	if err != nil {
		return err
	}
	if count > 0 {
		return e.NewBusinessError(1, "权限路由已存在")
	}
	return permissionModel.Create(data)
}

// ListPage 权限列表
func (s *PermissionService) ListPage(permission *form.ListPermission) *resources.Collection {
	var condition strings.Builder
	var args []any
	if permission.Name != "" {
		condition.WriteString("name like ? AND ")
		args = append(args, "%"+permission.Name+"%")
	}
	if permission.Method != "" {
		condition.WriteString("method = ? AND ")
		args = append(args, permission.Method)
	}
	if permission.Route != "" {
		condition.WriteString("route like ? AND ")
		args = append(args, "%"+permission.Route+"%")
	}
	if permission.IsAuth > 0 {
		condition.WriteString("is_auth = ? AND ")
		args = append(args, permission.IsAuth)
	}
	conditionStr := condition.String()
	if conditionStr != "" {
		conditionStr = strings.TrimSuffix(condition.String(), "AND ")
	}

	collection := model.NewPermission().ListPage(permission.Page, permission.PerPage, conditionStr, args)
	return collection.ToCollection()
}
