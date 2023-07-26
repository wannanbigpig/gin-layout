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
	permissionModel *model.Permission
}

func NewPermissionService() *PermissionService {
	return &PermissionService{}
}

func (p *PermissionService) Edit(params *form.EditPermission) error {
	data := map[string]any{
		"name":    params.Name,
		"desc":    params.Desc,
		"is_auth": params.IsAuth,
		"sort":    params.Sort,
	}
	if params.Id > 0 {
		return p.permissionModel.Update(params.Id, data)
	}
	data["func"] = params.Func
	data["func_path"] = params.FuncPath
	data["method"] = params.Method
	data["route"] = params.Route
	count, err := p.permissionModel.HasRoute(params.Route)
	if err != nil {
		return err
	}
	if count > 0 {
		return e.NewBusinessError(1, "权限路由已存在")
	}
	return p.permissionModel.Create(data)
}

// ListPage 权限列表
func (p *PermissionService) ListPage(permission *form.ListPermission) *resources.Collection {
	permissionModel := model.NewPermission()
	var condition string
	var args []any

	if permission.Name != "" {
		condition = "name like ? AND"
		args = append(args, "%"+permission.Name+"%")
	}
	if permission.Method != "" {
		condition = "method = ? AND "
		args = append(args, permission.Method)
	}
	if permission.Route != "" {
		condition = "route like ? AND "
		args = append(args, "%"+permission.Route+"%")
	}
	if permission.IsAuth > 0 {
		condition = "is_auth = ? AND "
		args = append(args, permission.IsAuth)
	}

	if condition != "" {
		condition = strings.TrimSuffix(condition, "AND ")
	}

	collection := permissionModel.ListPage(permission.Page, permission.PerPage, condition, args)
	return collection.ToCollection()
}
