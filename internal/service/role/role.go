package role

import (
	"errors"
	"strconv"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/query_builder"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// RoleService 角色服务。
type RoleService struct {
	service.Base
}

// NewRoleService 创建角色服务实例。
func NewRoleService() *RoleService {
	return &RoleService{}
}

// roleOptionsLimit 选择器返回的最大条数。
const roleOptionsLimit = 200

// Options 角色选择器：按名称/编码模糊搜索，供前端下拉选择器使用。
func (s *RoleService) Options(keyword string) ([]resources.RoleOptionResources, error) {
	conditionStr, args := query_builder.New().
		AddKeywordLike(keyword, "name", "code").
		Build()
	_, collection, err := model.ListPageE(model.NewRole(), 1, roleOptionsLimit, conditionStr, args, model.ListOptionalParams{
		SelectFields: []string{"id", "name", "code"},
		OrderBy:      "sort desc, id desc",
	})
	if err != nil {
		return nil, err
	}
	return resources.ToRoleOptions(collection), nil
}

// List 分页查询角色列表。
func (s *RoleService) List(params *form.RoleList) (interface{}, error) {
	condition, args := query_builder.New().
		AddLike("name", params.Name).
		AddLike("code", params.Code).
		AddEq("status", params.Status).
		Build()

	roleModel := model.NewRole()
	total, collection, err := model.ListPageE(
		roleModel,
		params.Page,
		params.PerPage,
		condition,
		args,
		model.ListOptionalParams{OrderBy: query_builder.ResolveListOrder(params.SortBy, params.SortOrder, "sort desc, id desc", "id", "name", "code", "sort", "created_at", "updated_at")},
	)
	if err != nil {
		return nil, err
	}

	return resources.ToRawCollection(params.Page, params.PerPage, total, collection), nil
}

// Create 新增角色。
func (s *RoleService) Create(params *form.CreateRole) error {
	return s.applyRoleMutation(&roleMutation{
		Code:        params.Code,
		Name:        params.Name,
		Description: params.Description,
		Status:      params.Status,
		Sort:        params.Sort,
		MenuList:    params.MenuList,
		DataScope:   params.DataScope,
		DeptIds:     params.DeptIds,
	})
}

// Update 更新角色。
func (s *RoleService) Update(params *form.UpdateRole) error {
	if params.Code != nil {
		return e.NewBusinessError(e.InvalidParameter)
	}
	return s.applyRoleMutation(&roleMutation{
		Id:          params.Id,
		Name:        params.Name,
		Description: params.Description,
		Status:      params.Status,
		Sort:        params.Sort,
		MenuList:    params.MenuList,
		DataScope:   params.DataScope,
		DeptIds:     params.DeptIds,
	})
}

// Delete 删除角色。
func (s *RoleService) Delete(id uint) error {
	role := model.NewRole()
	if err := role.GetById(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return e.NewBusinessError(e.RoleNotFound)
		}
		return err
	}
	if role.ID == 0 {
		return e.NewBusinessError(e.RoleNotFound)
	}
	if access.NewSystemDefaultsService().IsProtectedRole(role) {
		return e.NewBusinessError(e.SuperAdminCannotDelete)
	}

	return s.executeDeleteTransaction(role, id)
}

// Detail 获取角色详情。
func (s *RoleService) Detail(id uint) (any, error) {
	role := model.NewRole()
	if err := role.GetAllById(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, e.NewBusinessError(e.RoleNotFound)
		}
		return nil, err
	}
	if role.ID == 0 {
		return nil, e.NewBusinessError(e.RoleNotFound)
	}
	resource := resources.NewRoleTransformer().ToStruct(role)
	menuIDs, err := access.NewRoleMenuAssignmentService().NormalizeMenuIDs(resource.MenuList)
	if err != nil {
		return nil, err
	}
	resource.MenuList = menuIDs
	return resource, nil
}

// GetRoleMenus 获取角色的所有菜单标识列表。
// 调用跨包方法 access.UserPermissionSyncService.RoleMenuIDs 获取菜单 ID，再转换为字符串列表。
func (s *RoleService) GetRoleMenus(roleId uint) ([]string, error) {
	role := model.NewRole()
	if err := role.GetById(roleId); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, e.NewBusinessError(e.RoleNotFound)
		}
		return nil, err
	}
	if role.ID == 0 {
		return nil, e.NewBusinessError(e.RoleNotFound)
	}

	menuIDs, err := access.NewUserPermissionSyncService().RoleMenuIDs([]uint{roleId})
	if err != nil {
		return nil, err
	}
	menuIDs, err = access.NewRoleMenuAssignmentService().NormalizeMenuIDs(menuIDs)
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(menuIDs))
	for _, menuID := range menuIDs {
		result = append(result, strconv.FormatUint(uint64(menuID), 10))
	}
	return result, nil
}
