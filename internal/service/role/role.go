package role

import (
	"strconv"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/query_builder"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

const (
	maxRoleLevel      = 2
	maxChildrenPerTop = 5
)

// RoleService 角色服务。
type RoleService struct {
	service.Base
}

// NewRoleService 创建角色服务实例。
func NewRoleService() *RoleService {
	return &RoleService{}
}

// List 分页查询角色列表。
func (s *RoleService) List(params *form.RoleList) interface{} {
	condition, args := s.buildListCondition(params)

	roleModel := model.NewRole()
	total, collection, err := model.ListPageE(
		roleModel,
		params.Page,
		params.PerPage,
		condition,
		args,
		model.ListOptionalParams{OrderBy: "sort desc, id desc"},
	)
	if err != nil {
		return resources.ToRawCollection(params.Page, params.PerPage, 0, make([]*model.Role, 0))
	}

	return resources.ToRawCollection(params.Page, params.PerPage, total, collection)
}

func (s *RoleService) buildListCondition(params *form.RoleList) (string, []any) {
	return query_builder.New().
		AddLike("name", params.Name).
		AddEq("status", params.Status).
		AddEq("pid", params.Pid).
		Build()
}

// Create 新增角色。
func (s *RoleService) Create(params *form.CreateRole) error {
	return s.applyRoleMutation(&roleMutation{
		Code:        params.Code,
		Name:        params.Name,
		Description: params.Description,
		Status:      params.Status,
		Pid:         params.Pid,
		Sort:        params.Sort,
		MenuList:    params.MenuList,
	})
}

// Update 更新角色。
func (s *RoleService) Update(params *form.UpdateRole) error {
	return s.applyRoleMutation(&roleMutation{
		Id:          params.Id,
		Name:        params.Name,
		Description: params.Description,
		Status:      params.Status,
		Pid:         params.Pid,
		Sort:        params.Sort,
		MenuList:    params.MenuList,
	})
}

// Delete 删除角色。
func (s *RoleService) Delete(id uint) error {
	role := model.NewRole()
	if err := role.GetById(id); err != nil || role.ID == 0 {
		return e.NewBusinessError(e.RoleNotFound)
	}
	if access.NewSystemDefaultsService().IsProtectedRole(role) {
		return e.NewBusinessError(e.SuperAdminCannotDelete)
	}
	if role.ChildrenNum > 0 {
		return e.NewBusinessError(e.RoleHasChildren)
	}

	return s.executeDeleteTransaction(role, id)
}

// Detail 获取角色详情。
func (s *RoleService) Detail(id uint) (any, error) {
	role := model.NewRole()
	if err := role.GetAllById(id); err != nil || role.ID == 0 {
		return nil, e.NewBusinessError(e.RoleNotFound)
	}
	return resources.NewRoleTransformer().ToStruct(role), nil
}

// GetRoleMenus 获取角色的所有菜单标识列表。
// 调用跨包方法 access.UserPermissionSyncService.RoleMenuIDs 获取菜单 ID，再转换为字符串列表。
func (s *RoleService) GetRoleMenus(roleId uint) ([]string, error) {
	role := model.NewRole()
	if err := role.GetById(roleId); err != nil || role.ID == 0 {
		return nil, e.NewBusinessError(e.RoleNotFound)
	}

	menuIDs, err := access.NewUserPermissionSyncService().RoleMenuIDs([]uint{roleId})
	if err != nil {
		return nil, e.NewBusinessError(e.FAILURE)
	}

	result := make([]string, 0, len(menuIDs))
	for _, menuID := range menuIDs {
		result = append(result, strconv.FormatUint(uint64(menuID), 10))
	}
	return result, nil
}
