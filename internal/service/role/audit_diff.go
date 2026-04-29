package role

import (
	"sort"
	"strings"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/auditdiff"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

var roleDiffRules = []auditdiff.FieldRule{
	{Field: "id", Label: "角色ID"},
	{Field: "code", Label: "角色编码"},
	{Field: "name", Label: "角色名称"},
	{Field: "description", Label: "描述"},
	{
		Field: "status",
		Label: "状态",
		ValueLabels: map[string]string{
			"0": "禁用",
			"1": "启用",
		},
	},
	{Field: "pid", Label: "上级角色ID"},
	{Field: "pids", Label: "上级路径"},
	{Field: "level", Label: "层级"},
	{Field: "sort", Label: "排序"},
	{Field: "menu_list", Label: "菜单ID列表"},
}

// CreateWithAuditDiff 新增角色并返回精确 change_diff。
func (s *RoleService) CreateWithAuditDiff(params *form.CreateRole) (string, error) {
	if params == nil {
		return "", e.NewBusinessError(e.InvalidParameter)
	}
	payload := *params
	payload.Code = strings.TrimSpace(payload.Code)
	if payload.Code == "" {
		payload.Code = s.generateRoleCode()
	}
	if err := s.Create(&payload); err != nil {
		return "", err
	}
	after, err := s.snapshotRoleByCode(payload.Code)
	if err != nil {
		return auditdiff.Marshal(nil), nil
	}
	return buildRoleDiff(nil, after), nil
}

// UpdateWithAuditDiff 更新角色并返回精确 change_diff。
func (s *RoleService) UpdateWithAuditDiff(params *form.UpdateRole) (string, error) {
	if params == nil {
		return "", e.NewBusinessError(e.InvalidParameter)
	}
	before, err := s.snapshotRoleByID(params.Id)
	if err != nil {
		return "", err
	}
	if err := s.Update(params); err != nil {
		return "", err
	}
	after, err := s.snapshotRoleByID(params.Id)
	if err != nil {
		return auditdiff.Marshal(nil), nil
	}
	return buildRoleDiff(before, after), nil
}

// DeleteWithAuditDiff 删除角色并返回精确 change_diff。
func (s *RoleService) DeleteWithAuditDiff(id uint) (string, error) {
	before, err := s.snapshotRoleByID(id)
	if err != nil {
		return "", err
	}
	if err := s.Delete(id); err != nil {
		return "", err
	}
	return buildRoleDiff(before, nil), nil
}

func (s *RoleService) snapshotRoleByCode(code string) (map[string]any, error) {
	role := model.NewRole()
	if err := role.FindByCode(strings.TrimSpace(code)); err != nil || role.ID == 0 {
		return nil, e.NewBusinessError(e.RoleNotFound)
	}
	return s.snapshotRoleByID(role.ID)
}

func (s *RoleService) snapshotRoleByID(id uint) (map[string]any, error) {
	role := model.NewRole()
	if err := role.GetById(id); err != nil || role.ID == 0 {
		return nil, e.NewBusinessError(e.RoleNotFound)
	}
	menuIDs, err := model.NewRoleMenuMap().MenuIdsByRoleIds([]uint{id})
	if err != nil {
		return nil, err
	}
	sort.Slice(menuIDs, func(i, j int) bool {
		return menuIDs[i] < menuIDs[j]
	})
	return map[string]any{
		"id":          role.ID,
		"code":        role.Code,
		"name":        role.Name,
		"description": role.Description,
		"status":      role.Status,
		"pid":         role.Pid,
		"pids":        role.Pids,
		"level":       role.Level,
		"sort":        role.Sort,
		"menu_list":   menuIDs,
	}, nil
}

func buildRoleDiff(before, after map[string]any) string {
	items := auditdiff.BuildFieldDiff(before, after, roleDiffRules)
	return auditdiff.Marshal(items)
}
