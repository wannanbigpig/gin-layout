package role

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	utils2 "github.com/wannanbigpig/gin-layout/pkg/utils"
)

type roleMutation struct {
	Id          uint
	Name        string
	Description string
	Status      uint8
	Pid         uint
	Sort        uint
	MenuList    []uint
}

func (s *RoleService) applyRoleMutation(params *roleMutation) error {
	role := model.NewRole()
	originPids := "0"
	originPid := uint(0)
	if params.Id > 0 {
		if err := role.GetById(params.Id); err != nil || role.ID == 0 {
			return e.NewBusinessError(e.RoleNotFound)
		}
		originPids = role.Pids
		originPid = role.Pid
	}
	if params.Id > 0 && access.NewSystemDefaultsService().IsProtectedRole(role) {
		return e.NewBusinessError(e.SuperAdminCannotModify)
	}

	if params.Pid > 0 && params.Pid != role.Pid {
		parentRole := model.NewRole()
		if err := parentRole.GetById(params.Pid); err != nil || parentRole.ID == 0 {
			return e.NewBusinessError(e.ParentRoleNotExists)
		}

		if role.ID > 0 && utils2.WouldCauseCycle(role.ID, params.Pid, parentRole.Pids) {
			return e.NewBusinessError(e.ParentRoleInvalid)
		}

		if parentRole.Pid == 0 && (role.ID == 0 || role.Pid != params.Pid) && parentRole.ChildrenNum >= maxChildrenPerTop {
			return e.NewBusinessError(e.MaxChildRoles, fmt.Sprintf("每个顶级角色下最多只能创建%d个子角色", maxChildrenPerTop))
		}

		role.Level = parentRole.Level + 1
		if parentRole.Pids == "0" || parentRole.Pids == "" {
			role.Pids = fmt.Sprintf("%d", parentRole.ID)
		} else {
			role.Pids = fmt.Sprintf("%s,%d", parentRole.Pids, parentRole.ID)
		}
		role.Pid = params.Pid
	} else if params.Pid == 0 {
		role.Level = 1
		role.Pids = "0"
		role.Pid = 0
	} else {
		role.Pid = params.Pid
	}
	if role.Level > maxRoleLevel {
		return e.NewBusinessError(e.MaxRoleDepth)
	}

	if role.Code == "" {
		role.Code = s.generateRoleCode()
	}
	role.Name = params.Name
	role.Description = params.Description
	role.Status = params.Status
	role.Sort = params.Sort

	menuList, err := model.VerifyExistingIDs(model.NewMenu(), params.MenuList)
	if err != nil {
		return e.NewBusinessError(e.MenuNotFound)
	}

	db, err := role.GetDB()
	if err != nil {
		return err
	}
	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		role.SetDB(tx)

		if err := role.Save(); err != nil {
			return err
		}
		if role.Pids != originPids {
			updateExpr := s.buildPidsUpdateExpr(originPids, role.Pids)
			roleModel := model.NewRole()
			roleModel.SetDB(tx)
			if err := roleModel.UpdateChildrenPidsByParent(role.ID, updateExpr); err != nil {
				return err
			}
		}

		if originPid > 0 && originPid != role.Pid {
			if err := model.UpdateChildrenNum(model.NewRole(), originPid, tx); err != nil {
				return err
			}
		}
		if role.Pid > 0 && role.Pid != originPid {
			if err := model.UpdateChildrenNum(model.NewRole(), role.Pid, tx); err != nil {
				return err
			}
		}

		return s.updateRoleMenu(role.ID, menuList, tx)
	})
	if err != nil {
		return err
	}
	return access.NewPermissionSyncCoordinator().SyncUsersAffectedByRoles([]uint{role.ID})
}

func (s *RoleService) generateRoleCode() string {
	return "role_" + uuid.NewString()
}

func (s *RoleService) buildPidsUpdateExpr(originPids, newPids string) string {
	if originPids == "0" {
		return fmt.Sprintf(
			"CASE WHEN pids = '0' THEN '%s' WHEN pids LIKE '0,%%' THEN CONCAT('%s,', SUBSTRING(pids, 3)) ELSE pids END",
			newPids, newPids,
		)
	}

	return fmt.Sprintf(
		"CASE WHEN pids = '%s' THEN '%s' WHEN pids LIKE '%s,%%' THEN CONCAT('%s,', SUBSTRING(pids, %d)) ELSE pids END",
		originPids, newPids, originPids, newPids, len(originPids)+2,
	)
}

func (s *RoleService) updateRoleMenu(roleId uint, menuList []uint, tx ...*gorm.DB) error {
	roleMenuMap := model.NewRoleMenuMap()
	if len(tx) > 0 {
		roleMenuMap.SetDB(tx[0])
	}

	existingIds, err := model.ExtractColumnsByCondition[model.RoleMenuMap, *model.RoleMenuMap, uint](roleMenuMap, "menu_id", "role_id = ?", roleId)
	if err != nil {
		return err
	}

	toDelete, toAdd, _ := utils.CalculateChanges(existingIds, menuList)
	if len(toDelete) > 0 {
		if err := roleMenuMap.DeleteWhere("role_id = ? AND menu_id IN (?)", []any{roleId, toDelete}...); err != nil {
			return err
		}
	}

	if len(toAdd) > 0 {
		newMappings := lo.Map(toAdd, func(menuId uint, _ int) *model.RoleMenuMap {
			return &model.RoleMenuMap{RoleId: roleId, MenuId: menuId}
		})
		if err := roleMenuMap.CreateBatch(newMappings); err != nil {
			return err
		}
	}
	return nil
}

func (s *RoleService) executeDeleteTransaction(role *model.Role, id uint) error {
	db, err := role.GetDB()
	if err != nil {
		return e.NewBusinessError(e.RoleCannotDelete)
	}
	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		role.SetDB(tx)

		roleMenuMap := model.NewRoleMenuMap()
		roleMenuMap.SetDB(tx)
		if err := roleMenuMap.DeleteWhere("role_id = ?", id); err != nil {
			return err
		}

		parentId := role.Pid
		if _, err := role.DeleteByID(id); err != nil {
			return err
		}
		if parentId > 0 {
			if err := model.UpdateChildrenNum(model.NewRole(), parentId, tx); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return e.NewBusinessError(e.RoleCannotDelete)
	}
	return access.NewPermissionSyncCoordinator().SyncUsersAffectedByRoles([]uint{id})
}
