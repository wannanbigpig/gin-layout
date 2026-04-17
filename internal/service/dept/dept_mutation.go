package dept

import (
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	utils2 "github.com/wannanbigpig/gin-layout/pkg/utils"
)

type deptMutation struct {
	Id          uint
	Name        string
	Pid         uint
	Description string
	Sort        uint
}

func (s *DeptService) applyDeptMutation(params *deptMutation) error {
	dept := model.NewDepartment()
	originPids := "0"
	originPid := uint(0)
	if params.Id > 0 {
		if err := dept.GetById(params.Id); err != nil || dept.ID == 0 {
			return e.NewBusinessError(1, "编辑的部门不存在")
		}
		originPids = dept.Pids
		originPid = dept.Pid
	}
	if params.Id > 0 && access.NewSystemDefaultsService().IsProtectedDepartment(dept) {
		if params.Pid != dept.Pid || params.Sort != dept.Sort {
			return e.NewBusinessError(e.FAILURE, "默认部门只允许修改名称和描述")
		}
	}

	if params.Pid > 0 && params.Pid != dept.Pid {
		parentDept := model.NewDepartment()
		if err := parentDept.GetById(params.Pid); err != nil || parentDept.ID == 0 {
			return e.NewBusinessError(1, "上级部门不存在")
		}

		if dept.ID > 0 && utils2.WouldCauseCycle(dept.ID, params.Pid, parentDept.Pids) {
			return e.NewBusinessError(1, "上级部门不能是当前部门自身或其子部门")
		}

		dept.Level = parentDept.Level + 1
		if parentDept.Pids == "0" || parentDept.Pids == "" {
			dept.Pids = fmt.Sprintf("%d", parentDept.ID)
		} else {
			dept.Pids = fmt.Sprintf("%s,%d", parentDept.Pids, parentDept.ID)
		}
		dept.Pid = params.Pid
	} else if params.Pid == deptRootPid {
		dept.Level = 1
		dept.Pids = "0"
		dept.Pid = deptRootPid
	} else {
		dept.Pid = params.Pid
	}
	if dept.Level > maxDeptLevel {
		return e.NewBusinessError(1, "最多只能创建5级部门")
	}

	if dept.Code == "" {
		dept.Code = s.generateDeptCode()
	}
	dept.Name = params.Name
	dept.Description = params.Description
	dept.Sort = params.Sort

	db, err := dept.GetDB()
	if err != nil {
		return err
	}
	return access.RunInTransaction(db, func(tx *gorm.DB) error {
		dept.SetDB(tx)

		if err := dept.Save(); err != nil {
			return err
		}
		if dept.Pids != originPids {
			updateExpr := s.buildPidsUpdateExpr(originPids, dept.Pids)
			deptModel := model.NewDepartment()
			deptModel.SetDB(tx)
			if err := deptModel.UpdateChildrenPidsByParent(dept.ID, updateExpr); err != nil {
				return err
			}
		}

		if originPid > 0 && originPid != dept.Pid {
			if err := model.UpdateChildrenNum(model.NewDepartment(), originPid, tx); err != nil {
				return err
			}
		}
		if dept.Pid > 0 && dept.Pid != originPid {
			if err := model.UpdateChildrenNum(model.NewDepartment(), dept.Pid, tx); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *DeptService) generateDeptCode() string {
	return "dept_" + uuid.NewString()
}

func (s *DeptService) buildPidsUpdateExpr(originPids, newPids string) string {
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

func (s *DeptService) executeDeleteTransaction(dept *model.Department, id uint) error {
	db, err := dept.GetDB()
	if err != nil {
		return e.NewBusinessError(1, "删除部门失败")
	}
	err = access.NewPermissionSyncCoordinator().RunAfterCommit(db, "删除部门后刷新权限缓存失败", func(tx *gorm.DB) error {
		dept.SetDB(tx)

		affectedUserIDs, err := s.userIDsByDept(id, tx)
		if err != nil {
			return err
		}

		deptRoleMap := model.NewDeptRoleMap()
		deptRoleMap.SetDB(tx)
		if err := deptRoleMap.DeleteWhere("dept_id = ?", id); err != nil {
			return err
		}

		adminUserDeptMap := model.NewAdminUserDeptMap()
		adminUserDeptMap.SetDB(tx)
		if err := adminUserDeptMap.DeleteWhere("dept_id = ?", id); err != nil {
			return err
		}

		parentId := dept.Pid
		if _, err := dept.DeleteByID(id); err != nil {
			return err
		}
		if parentId > 0 {
			if err := model.UpdateChildrenNum(model.NewDepartment(), parentId, tx); err != nil {
				return err
			}
		}

		return access.NewPermissionSyncCoordinator().SyncUsers(affectedUserIDs, tx)
	})
	if err != nil {
		return e.NewBusinessError(1, "删除部门失败")
	}
	return nil
}
