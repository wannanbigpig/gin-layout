package admin

import (
	"sort"
	"strings"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/auditdiff"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

var adminUserDiffRules = []auditdiff.FieldRule{
	{Field: "id", Label: "用户ID"},
	{Field: "username", Label: "用户名"},
	{Field: "nickname", Label: "昵称"},
	{Field: "phone_number", Label: "手机号"},
	{Field: "country_code", Label: "国家区号"},
	{Field: "email", Label: "邮箱"},
	{Field: "avatar", Label: "头像"},
	{
		Field: "status",
		Label: "状态",
		ValueLabels: map[string]string{
			"0": "禁用",
			"1": "启用",
		},
	},
	{
		Field: "is_super_admin",
		Label: "超级管理员",
		ValueLabels: map[string]string{
			"0": "否",
			"1": "是",
		},
	},
	{Field: "dept_ids", Label: "部门ID列表"},
	{Field: "role_ids", Label: "角色ID列表"},
}

var adminUserRoleBindingDiffRules = []auditdiff.FieldRule{
	{Field: "user_id", Label: "用户ID"},
	{Field: "role_ids", Label: "角色ID列表"},
}

// CreateWithAuditDiff 新增管理员并返回精确 change_diff。
func (s *AdminUserService) CreateWithAuditDiff(params *form.CreateAdminUser) (string, error) {
	if params == nil {
		return "", e.NewBusinessError(e.InvalidParameter)
	}
	if err := s.Create(params); err != nil {
		return "", err
	}
	username := ""
	if params.Username != nil {
		username = strings.TrimSpace(*params.Username)
	}
	if username == "" {
		return auditdiff.Marshal(nil), nil
	}
	after, err := s.snapshotAdminUserByUsername(username)
	if err != nil {
		return auditdiff.Marshal(nil), nil
	}
	return buildAdminUserDiff(nil, after), nil
}

// UpdateWithAuditDiff 更新管理员并返回精确 change_diff。
func (s *AdminUserService) UpdateWithAuditDiff(params *form.UpdateAdminUser) (string, error) {
	if params == nil {
		return "", e.NewBusinessError(e.InvalidParameter)
	}
	before, err := s.snapshotAdminUserByID(params.Id)
	if err != nil {
		return "", err
	}
	if err := s.Update(params); err != nil {
		return "", err
	}
	after, err := s.snapshotAdminUserByID(params.Id)
	if err != nil {
		return auditdiff.Marshal(nil), nil
	}
	return buildAdminUserDiff(before, after), nil
}

// DeleteWithAuditDiff 删除管理员并返回精确 change_diff。
func (s *AdminUserService) DeleteWithAuditDiff(id uint) (string, error) {
	before, err := s.snapshotAdminUserByID(id)
	if err != nil {
		return "", err
	}
	if err := s.Delete(id); err != nil {
		return "", err
	}
	return buildAdminUserDiff(before, nil), nil
}

// UpdateProfileWithAuditDiff 更新个人资料并返回精确 change_diff。
func (s *AdminUserService) UpdateProfileWithAuditDiff(uid uint, params *form.UpdateProfile) (string, error) {
	if params == nil {
		return "", e.NewBusinessError(e.InvalidParameter)
	}
	before, err := s.snapshotAdminUserByID(uid)
	if err != nil {
		return "", err
	}
	if err := s.UpdateProfile(uid, params); err != nil {
		return "", err
	}
	after, err := s.snapshotAdminUserByID(uid)
	if err != nil {
		return auditdiff.Marshal(nil), nil
	}
	return buildAdminUserDiff(before, after), nil
}

// BindRoleWithAuditDiff 绑定角色并返回精确 change_diff。
func (s *AdminUserService) BindRoleWithAuditDiff(params *form.BindRole) (string, error) {
	if params == nil {
		return "", e.NewBusinessError(e.InvalidParameter)
	}
	before, err := s.snapshotAdminUserRoleBinding(params.UserId)
	if err != nil {
		return "", err
	}
	if err := s.BindRole(params); err != nil {
		return "", err
	}
	after, err := s.snapshotAdminUserRoleBinding(params.UserId)
	if err != nil {
		return auditdiff.Marshal(nil), nil
	}
	items := auditdiff.BuildFieldDiff(before, after, adminUserRoleBindingDiffRules)
	return auditdiff.Marshal(items), nil
}

func (s *AdminUserService) snapshotAdminUserByUsername(username string) (map[string]any, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, e.NewBusinessError(e.InvalidParameter)
	}
	user := model.NewAdminUsers()
	if err := user.GetDetail("username = ? AND deleted_at = 0", username); err != nil || user.ID == 0 {
		return nil, e.NewBusinessError(e.UserDoesNotExist)
	}
	return s.snapshotAdminUserByID(user.ID)
}

func (s *AdminUserService) snapshotAdminUserByID(id uint) (map[string]any, error) {
	user := model.NewAdminUsers()
	if err := user.GetById(id); err != nil || user.ID == 0 {
		return nil, e.NewBusinessError(e.UserDoesNotExist)
	}
	deptIDs, err := model.NewAdminUserDeptMap().DeptIdsByUid(user.ID)
	if err != nil {
		return nil, err
	}
	roleIDs, err := model.NewAdminUserRoleMap().RoleIdsByUid(user.ID)
	if err != nil {
		return nil, err
	}
	sortUintSlice(deptIDs)
	sortUintSlice(roleIDs)
	return map[string]any{
		"id":             user.ID,
		"username":       user.Username,
		"nickname":       user.Nickname,
		"phone_number":   user.PhoneNumber,
		"country_code":   user.CountryCode,
		"email":          user.Email,
		"avatar":         user.Avatar,
		"status":         user.Status,
		"is_super_admin": user.IsSuperAdmin,
		"dept_ids":       deptIDs,
		"role_ids":       roleIDs,
	}, nil
}

func (s *AdminUserService) snapshotAdminUserRoleBinding(userID uint) (map[string]any, error) {
	user := model.NewAdminUsers()
	if err := user.GetById(userID); err != nil || user.ID == 0 {
		return nil, e.NewBusinessError(e.UserDoesNotExist)
	}
	roleIDs, err := model.NewAdminUserRoleMap().RoleIdsByUid(userID)
	if err != nil {
		return nil, err
	}
	sortUintSlice(roleIDs)
	return map[string]any{
		"user_id":  userID,
		"role_ids": roleIDs,
	}, nil
}

func buildAdminUserDiff(before, after map[string]any) string {
	items := auditdiff.BuildFieldDiff(before, after, adminUserDiffRules)
	return auditdiff.Marshal(items)
}

func sortUintSlice(values []uint) {
	if len(values) == 0 {
		return
	}
	sort.Slice(values, func(i, j int) bool {
		return values[i] < values[j]
	})
}
