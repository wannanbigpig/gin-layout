package admin

import (
	"fmt"

	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// BindDept 绑定部门。
func (s *AdminUserService) BindDept(uid uint, deptId []uint, tx ...*gorm.DB) (err error) {
	var dbTx *gorm.DB
	if len(tx) > 0 {
		dbTx = tx[0]
	} else {
		dbTx, err = model.NewAdminUserDeptMap().GetDB()
		if err != nil {
			return err
		}
	}

	adminUserDeptMap := model.NewAdminUserDeptMap()
	adminUserDeptMap.SetDB(dbTx)

	existingIds, err := model.ExtractColumnsByCondition[model.AdminUserDeptMap, *model.AdminUserDeptMap, uint](adminUserDeptMap, "dept_id", "uid = ?", uid)
	if err != nil {
		return err
	}

	toDelete, toAdd, _ := utils.CalculateChanges(existingIds, deptId)
	if len(toDelete) > 0 {
		if err := adminUserDeptMap.DeleteWhere("uid = ? AND dept_id IN (?)", []any{uid, toDelete}...); err != nil {
			return err
		}
		if err := s.updateDeptUserNumber(toDelete, -1, dbTx); err != nil {
			return err
		}
	}

	if len(toAdd) > 0 {
		newMappings := lo.Map(toAdd, func(deptId uint, _ int) *model.AdminUserDeptMap {
			return &model.AdminUserDeptMap{DeptId: deptId, Uid: uid}
		})
		if err := adminUserDeptMap.CreateBatch(newMappings); err != nil {
			return err
		}
		if err := s.updateDeptUserNumber(toAdd, 1, dbTx); err != nil {
			return err
		}
	}

	return access.NewPermissionSyncCoordinator().SyncUser(uid, tx...)
}

func (s *AdminUserService) updateDeptUserNumber(deptIds []uint, delta int, tx *gorm.DB) error {
	if len(deptIds) == 0 {
		return nil
	}

	deptModel := model.NewDepartment()
	deptModel.SetDB(tx)

	var updateExpr string
	if delta < 0 {
		updateExpr = fmt.Sprintf("GREATEST(user_number + %d, 0)", delta)
	} else {
		updateExpr = fmt.Sprintf("user_number + %d", delta)
	}

	return deptModel.UpdateUserNumberByIds(deptIds, updateExpr)
}

// BindRole 绑定角色。
func (s *AdminUserService) BindRole(params *form.BindRole) error {
	adminUserModel := model.NewAdminUsers()
	err := adminUserModel.GetById(params.UserId)
	if err != nil {
		return e.NewBusinessError(e.FAILURE, "用户不存在")
	}

	ids, err := model.VerifyExistingIDs(model.NewRole(), params.RoleIds)
	if err != nil {
		return e.NewBusinessError(e.FAILURE, "判断角色是否存在失败")
	}
	if err := access.NewSystemDefaultsService().RequireSuperAdminRoleForUser(adminUserModel.ID, ids); err != nil {
		return err
	}

	db, err := model.NewAdminUserRoleMap().GetDB()
	if err != nil {
		return e.NewBusinessError(e.FAILURE, "绑定角色失败")
	}
	err = access.NewPermissionSyncCoordinator().RunAfterCommit(db, "绑定角色后刷新权限缓存失败", func(tx *gorm.DB) error {
		return s.updateAdminUserRole(adminUserModel.ID, ids, tx)
	})
	if err != nil {
		return e.NewBusinessError(e.FAILURE, "绑定角色失败")
	}
	return nil
}

func (s *AdminUserService) updateAdminUserRole(uid uint, roleIds []uint, tx ...*gorm.DB) error {
	if err := access.NewSystemDefaultsService().RequireSuperAdminRoleForUser(uid, roleIds); err != nil {
		return err
	}

	adminUserRoleMap := model.NewAdminUserRoleMap()
	if len(tx) > 0 {
		adminUserRoleMap.SetDB(tx[0])
	}
	existingIds, err := model.ExtractColumnsByCondition[model.AdminUserRoleMap, *model.AdminUserRoleMap, uint](adminUserRoleMap, "role_id", "uid = ?", uid)
	if err != nil {
		return err
	}

	toDelete, toAdd, _ := utils.CalculateChanges(existingIds, roleIds)
	if len(toDelete) > 0 {
		if err := adminUserRoleMap.DeleteWhere("uid = ? AND role_id IN (?)", []any{uid, toDelete}...); err != nil {
			return err
		}
	}

	if len(toAdd) > 0 {
		newMappings := lo.Map(toAdd, func(roleId uint, _ int) *model.AdminUserRoleMap {
			return &model.AdminUserRoleMap{RoleId: roleId, Uid: uid}
		})
		if err := adminUserRoleMap.CreateBatch(newMappings); err != nil {
			return err
		}
	}

	return access.NewPermissionSyncCoordinator().SyncUser(uid, tx...)
}
