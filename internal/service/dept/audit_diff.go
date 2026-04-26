package dept

import (
	"sort"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/auditdiff"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

var deptDiffRules = []auditdiff.FieldRule{
	{Field: "id", Label: "部门ID"},
	{Field: "code", Label: "部门编码"},
	{Field: "name", Label: "部门名称"},
	{Field: "pid", Label: "上级部门ID"},
	{Field: "pids", Label: "上级路径"},
	{Field: "level", Label: "层级"},
	{Field: "sort", Label: "排序"},
	{Field: "description", Label: "描述"},
	{Field: "user_number", Label: "用户数量"},
	{Field: "role_ids", Label: "角色ID列表"},
}

var deptRoleBindingDiffRules = []auditdiff.FieldRule{
	{Field: "dept_id", Label: "部门ID"},
	{Field: "role_ids", Label: "角色ID列表"},
}

// CreateWithAuditDiff 新增部门并返回精确 change_diff。
func (s *DeptService) CreateWithAuditDiff(params *form.CreateDept) (string, error) {
	if params == nil {
		return "", e.NewBusinessError(e.InvalidParameter)
	}
	deptModel, err := s.applyDeptMutation(&deptMutation{
		Name:        params.Name,
		Pid:         params.Pid,
		Description: params.Description,
		Sort:        params.Sort,
	})
	if err != nil {
		return "", err
	}
	after, err := s.snapshotDeptByID(deptModel.ID)
	if err != nil {
		return auditdiff.Marshal(nil), nil
	}
	return buildDeptDiff(nil, after), nil
}

// UpdateWithAuditDiff 更新部门并返回精确 change_diff。
func (s *DeptService) UpdateWithAuditDiff(params *form.UpdateDept) (string, error) {
	if params == nil {
		return "", e.NewBusinessError(e.InvalidParameter)
	}
	before, err := s.snapshotDeptByID(params.Id)
	if err != nil {
		return "", err
	}
	if err := s.Update(params); err != nil {
		return "", err
	}
	after, err := s.snapshotDeptByID(params.Id)
	if err != nil {
		return auditdiff.Marshal(nil), nil
	}
	return buildDeptDiff(before, after), nil
}

// DeleteWithAuditDiff 删除部门并返回精确 change_diff。
func (s *DeptService) DeleteWithAuditDiff(id uint) (string, error) {
	before, err := s.snapshotDeptByID(id)
	if err != nil {
		return "", err
	}
	if err := s.Delete(id); err != nil {
		return "", err
	}
	return buildDeptDiff(before, nil), nil
}

// BindRoleWithAuditDiff 绑定部门角色并返回精确 change_diff。
func (s *DeptService) BindRoleWithAuditDiff(params *form.DeptBindRole) (string, error) {
	if params == nil {
		return "", e.NewBusinessError(e.InvalidParameter)
	}
	before, err := s.snapshotDeptRoleBinding(params.DeptId)
	if err != nil {
		return "", err
	}
	if err := s.BindRole(params); err != nil {
		return "", err
	}
	after, err := s.snapshotDeptRoleBinding(params.DeptId)
	if err != nil {
		return auditdiff.Marshal(nil), nil
	}
	items := auditdiff.BuildFieldDiff(before, after, deptRoleBindingDiffRules)
	return auditdiff.Marshal(items), nil
}

func (s *DeptService) snapshotDeptByID(id uint) (map[string]any, error) {
	dept := model.NewDepartment()
	if err := dept.GetById(id); err != nil || dept.ID == 0 {
		return nil, e.NewBusinessError(e.DepartmentNotFound)
	}
	roleIDs, err := model.NewDeptRoleMap().RoleIdsByDeptIds([]uint{id})
	if err != nil {
		return nil, err
	}
	sort.Slice(roleIDs, func(i, j int) bool {
		return roleIDs[i] < roleIDs[j]
	})
	return map[string]any{
		"id":          dept.ID,
		"code":        dept.Code,
		"name":        dept.Name,
		"pid":         dept.Pid,
		"pids":        dept.Pids,
		"level":       dept.Level,
		"sort":        dept.Sort,
		"description": dept.Description,
		"user_number": dept.UserNumber,
		"role_ids":    roleIDs,
	}, nil
}

func (s *DeptService) snapshotDeptRoleBinding(deptID uint) (map[string]any, error) {
	dept := model.NewDepartment()
	if err := dept.GetById(deptID); err != nil || dept.ID == 0 {
		return nil, e.NewBusinessError(e.DepartmentNotFound)
	}
	roleIDs, err := model.NewDeptRoleMap().RoleIdsByDeptIds([]uint{deptID})
	if err != nil {
		return nil, err
	}
	sort.Slice(roleIDs, func(i, j int) bool {
		return roleIDs[i] < roleIDs[j]
	})
	return map[string]any{
		"dept_id":  deptID,
		"role_ids": roleIDs,
	}, nil
}

func buildDeptDiff(before, after map[string]any) string {
	items := auditdiff.BuildFieldDiff(before, after, deptDiffRules)
	return auditdiff.Marshal(items)
}
