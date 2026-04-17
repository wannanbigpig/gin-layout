package access

import (
	"errors"

	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

// SystemDefaultsService 负责校验和补齐系统默认角色、部门与关联关系。
type SystemDefaultsService struct{}

// NewSystemDefaultsService 创建系统默认数据服务实例。
func NewSystemDefaultsService() *SystemDefaultsService {
	return &SystemDefaultsService{}
}

// Ensure 确保系统默认数据和关联关系存在。
func (s *SystemDefaultsService) Ensure(tx ...*gorm.DB) error {
	existingTx := FirstTx(tx)
	if existingTx != nil {
		return s.ensureWithTx(existingTx)
	}

	db, err := model.NewAdminUsers().GetDB()
	if err != nil {
		return err
	}
	return db.Transaction(func(execTx *gorm.DB) error {
		return s.ensureWithTx(execTx)
	})
}

// IsProtectedRole 判断角色是否为系统保护角色。
func (s *SystemDefaultsService) IsProtectedRole(role *model.Role) bool {
	return role != nil && role.IsSystemRole() && role.Code == global.SuperAdminRoleCode
}

// IsProtectedDepartment 判断部门是否为系统保护部门。
func (s *SystemDefaultsService) IsProtectedDepartment(dept *model.Department) bool {
	return dept != nil && dept.IsSystemDepartment() && dept.Code == global.DefaultDepartmentCode
}

// EnsureSuperAdminRoleMenus 兼容旧入口，确保超级管理员角色菜单完整。
func (s *SystemDefaultsService) EnsureSuperAdminRoleMenus(tx ...*gorm.DB) error {
	return s.Ensure(tx...)
}

func (s *SystemDefaultsService) ensureWithTx(tx *gorm.DB) error {
	dept, err := s.ensureDefaultDepartment(tx)
	if err != nil {
		return err
	}

	role, err := s.ensureSuperAdminRole(tx)
	if err != nil {
		return err
	}

	if err := s.ensureSuperAdminUser(tx); err != nil {
		return err
	}

	if err := s.ensureSuperAdminUserDept(tx, dept.ID); err != nil {
		return err
	}

	if err := s.ensureSuperAdminUserRole(tx, role.ID); err != nil {
		return err
	}

	return s.ensureSuperAdminRoleMenusWithTx(tx, role.ID)
}

func (s *SystemDefaultsService) ensureDefaultDepartment(tx *gorm.DB) (*model.Department, error) {
	dept := model.NewDepartment()
	dept.SetDB(tx)
	if err := dept.FindByCode(global.DefaultDepartmentCode); err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}

		dept.Code = global.DefaultDepartmentCode
		dept.IsSystem = global.Yes
		dept.Pid = 0
		dept.Pids = "0"
		dept.Level = 1
		dept.Name = "默认部门"
		dept.Description = "系统默认部门"
		dept.Sort = 100
		if err := dept.Save(); err != nil {
			return nil, err
		}
		return dept, nil
	}

	updates := map[string]any{}
	if dept.IsSystem != global.Yes {
		updates["is_system"] = global.Yes
	}
	if dept.Pid != 0 {
		updates["pid"] = 0
	}
	if dept.Pids != "0" {
		updates["pids"] = "0"
	}
	if dept.Level != 1 {
		updates["level"] = 1
	}
	if dept.Code != global.DefaultDepartmentCode {
		updates["code"] = global.DefaultDepartmentCode
	}
	if len(updates) > 0 {
		if err := dept.UpdateById(dept.ID, updates); err != nil {
			return nil, err
		}
	}

	return dept, nil
}

func (s *SystemDefaultsService) ensureSuperAdminRole(tx *gorm.DB) (*model.Role, error) {
	role := model.NewRole()
	role.SetDB(tx)
	if err := role.FindByCode(global.SuperAdminRoleCode); err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}

		role.Code = global.SuperAdminRoleCode
		role.IsSystem = global.Yes
		role.Pid = 0
		role.Pids = "0"
		role.Level = 1
		role.Name = "超级管理员"
		role.Description = "系统默认超级管理员角色"
		role.Sort = 100
		role.Status = global.Yes
		if err := role.Save(); err != nil {
			return nil, err
		}
		return role, nil
	}

	updates := map[string]any{}
	if role.IsSystem != global.Yes {
		updates["is_system"] = global.Yes
	}
	if role.Code != global.SuperAdminRoleCode {
		updates["code"] = global.SuperAdminRoleCode
	}
	if role.Pid != 0 {
		updates["pid"] = 0
	}
	if role.Pids != "0" {
		updates["pids"] = "0"
	}
	if role.Level != 1 {
		updates["level"] = 1
	}
	if role.Name != "超级管理员" {
		updates["name"] = "超级管理员"
	}
	if role.Description != "系统默认超级管理员角色" {
		updates["description"] = "系统默认超级管理员角色"
	}
	if role.Sort != 100 {
		updates["sort"] = 100
	}
	if role.Status != 1 {
		updates["status"] = 1
	}
	if len(updates) > 0 {
		if err := role.UpdateById(role.ID, updates); err != nil {
			return nil, err
		}
	}

	return role, nil
}

func (s *SystemDefaultsService) ensureSuperAdminUser(tx *gorm.DB) error {
	adminUser := model.NewAdminUsers()
	adminUser.SetDB(tx)
	if err := adminUser.GetById(global.SuperAdminId); err != nil {
		return err
	}

	updates := map[string]any{}
	if adminUser.IsSuperAdmin != global.Yes {
		updates["is_super_admin"] = global.Yes
	}
	if adminUser.Status != model.AdminUserStatusEnabled {
		updates["status"] = model.AdminUserStatusEnabled
	}
	if adminUser.Username != global.SuperAdminRoleCode {
		updates["username"] = global.SuperAdminRoleCode
	}
	if len(updates) == 0 {
		return nil
	}
	return adminUser.UpdateById(adminUser.ID, updates)
}

func (s *SystemDefaultsService) ensureSuperAdminUserDept(tx *gorm.DB, deptID uint) error {
	rel := model.NewAdminUserDeptMap()
	rel.SetDB(tx)
	count, err := rel.CountByCondition("uid = ? AND dept_id = ?", global.SuperAdminId, deptID)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	rel.Uid = global.SuperAdminId
	rel.DeptId = deptID
	return rel.CreateOne()
}

func (s *SystemDefaultsService) ensureSuperAdminUserRole(tx *gorm.DB, roleID uint) error {
	rel := model.NewAdminUserRoleMap()
	rel.SetDB(tx)
	count, err := rel.CountByCondition("uid = ? AND role_id = ?", global.SuperAdminId, roleID)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	rel.Uid = global.SuperAdminId
	rel.RoleId = roleID
	return rel.CreateOne()
}

func (s *SystemDefaultsService) ensureSuperAdminRoleMenusWithTx(tx *gorm.DB, roleID uint) error {
	menuModel := model.NewMenu()
	menuModel.SetDB(tx)
	allMenuIDs, err := menuModel.AllIds()
	if err != nil {
		return err
	}
	allMenuIDs = lo.Uniq(allMenuIDs)

	roleMenuMap := model.NewRoleMenuMap()
	roleMenuMap.SetDB(tx)
	existingIDs, err := model.ExtractColumnsByCondition[model.RoleMenuMap, *model.RoleMenuMap, uint](roleMenuMap, "menu_id", "role_id = ?", roleID)
	if err != nil {
		return err
	}

	toDelete, toAdd, _ := utils.CalculateChanges(existingIDs, allMenuIDs)
	if len(toDelete) > 0 {
		if err := roleMenuMap.DeleteWhere("role_id = ? AND menu_id IN (?)", roleID, toDelete); err != nil {
			return err
		}
	}
	if len(toAdd) == 0 {
		return nil
	}

	newMappings := lo.Map(toAdd, func(menuID uint, _ int) *model.RoleMenuMap {
		return &model.RoleMenuMap{RoleId: roleID, MenuId: menuID}
	})
	return roleMenuMap.CreateBatch(newMappings)
}

// RequireSuperAdminRoleForUser 确保超级管理员用户始终保留超级管理员角色。
func (s *SystemDefaultsService) RequireSuperAdminRoleForUser(uid uint, roleIDs []uint) error {
	if uid != global.SuperAdminId {
		return nil
	}

	role := model.NewRole()
	if err := role.FindByCode(global.SuperAdminRoleCode); err != nil {
		return err
	}
	if lo.Contains(roleIDs, role.ID) {
		return nil
	}
	return e.NewBusinessError(e.FAILURE, "系统默认超级管理员必须保留超级管理员角色")
}
