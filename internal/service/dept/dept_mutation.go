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

// deptMutation 部门变更参数，用于封装新增/更新部门的请求数据。
type deptMutation struct {
	Id          uint   // 部门 ID，0 表示新增
	Name        string // 部门名称
	Pid         uint   // 父部门 ID，0 表示顶级部门
	Description string // 部门描述
	Sort        uint   // 排序权重
}

// applyDeptMutation 执行部门变更操作（新增/更新）。
// 处理逻辑：
// 1. 验证部门是否存在（更新时）
// 2. 检查受保护部门（系统默认部门只允许改名称和描述）
// 3. 验证并构建树形路径（pids, level）
// 4. 填充部门基础字段
// 5. 事务保存：部门数据、级联更新子部门 pids、更新子部门数量
func (s *DeptService) applyDeptMutation(params *deptMutation) error {
	dept := model.NewDepartment()
	originPids := "0"
	originPid := uint(0)
	// 更新场景：加载现有部门数据，记录原始 pids 用于后续级联判断
	if params.Id > 0 {
		if err := dept.GetById(params.Id); err != nil || dept.ID == 0 {
			return e.NewBusinessError(1, "编辑的部门不存在")
		}
		originPids = dept.Pids
		originPid = dept.Pid
	}
	// 检查是否为受保护部门（系统默认部门只允许修改名称和描述）
	if params.Id > 0 && access.NewSystemDefaultsService().IsProtectedDepartment(dept) {
		if params.Pid != dept.Pid || params.Sort != dept.Sort {
			return e.NewBusinessError(e.FAILURE, "默认部门只允许修改名称和描述")
		}
	}

	// 处理父部门变更：验证父部门、检测环路、计算层级和路径
	if params.Pid > 0 && params.Pid != dept.Pid {
		parentDept := model.NewDepartment()
		if err := parentDept.GetById(params.Pid); err != nil || parentDept.ID == 0 {
			return e.NewBusinessError(1, "上级部门不存在")
		}

		// 环路检测：当前部门若已在父部门的祖先路径上，选择该父部门会形成环
		if dept.ID > 0 && utils2.WouldCauseCycle(dept.ID, params.Pid, parentDept.Pids) {
			return e.NewBusinessError(1, "上级部门不能是当前部门自身或其子部门")
		}

		// 构建新的层级和路径：父层级 +1，pids = 父 pids + 父 ID
		dept.Level = parentDept.Level + 1
		if parentDept.Pids == "0" || parentDept.Pids == "" {
			dept.Pids = fmt.Sprintf("%d", parentDept.ID)
		} else {
			dept.Pids = fmt.Sprintf("%s,%d", parentDept.Pids, parentDept.ID)
		}
		dept.Pid = params.Pid
	} else if params.Pid == deptRootPid {
		// 设置为顶级部门
		dept.Level = 1
		dept.Pids = "0"
		dept.Pid = deptRootPid
	} else {
		// 父部门未变更，仅同步 pid 字段
		dept.Pid = params.Pid
	}
	// 检查部门层级深度是否超限
	if dept.Level > maxDeptLevel {
		return e.NewBusinessError(1, "最多只能创建 5 级部门")
	}

	// 生成部门 code（为空时）
	if dept.Code == "" {
		dept.Code = s.generateDeptCode()
	}
	// 填充可变更字段
	dept.Name = params.Name
	dept.Description = params.Description
	dept.Sort = params.Sort

	db, err := dept.GetDB()
	if err != nil {
		return err
	}
	// 事务执行：保存部门、级联更新子部门 pids、更新子部门数量
	return access.RunInTransaction(db, func(tx *gorm.DB) error {
		dept.SetDB(tx)

		if err := dept.Save(); err != nil {
			return err
		}
		// pids 变更时，级联更新所有子部门的 pids 路径
		if dept.Pids != originPids {
			updateExpr := s.buildPidsUpdateExpr(originPids, dept.Pids)
			deptModel := model.NewDepartment()
			deptModel.SetDB(tx)
			if err := deptModel.UpdateChildrenPidsByParent(dept.ID, updateExpr); err != nil {
				return err
			}
		}

		// 原父部门的子部门数量减 1
		if originPid > 0 && originPid != dept.Pid {
			if err := model.UpdateChildrenNum(model.NewDepartment(), originPid, tx); err != nil {
				return err
			}
		}
		// 新父部门的子部门数量加 1
		if dept.Pid > 0 && dept.Pid != originPid {
			if err := model.UpdateChildrenNum(model.NewDepartment(), dept.Pid, tx); err != nil {
				return err
			}
		}
		return nil
	})
}

// generateDeptCode 生成部门唯一编码，格式：dept_{uuid}。
func (s *DeptService) generateDeptCode() string {
	return "dept_" + uuid.NewString()
}

// buildPidsUpdateExpr 构建 SQL CASE 表达式，用于级联更新子部门的 pids 路径。
// 场景：当某部门的 pids 变更时，其所有子部门的 pids 前缀需要同步更新。
// 参数：
//   - originPids: 原始路径
//   - newPids: 新路径
// 返回：SQL CASE 表达式字符串
// 示例：originPids="1,2", newPids="1,8" 时，子部门 "1,2,3" → "1,8,3"
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

// executeDeleteTransaction 执行部门删除事务。
// 处理逻辑：
// 1. 查询部门关联的用户 ID 列表
// 2. 删除部门 - 角色、用户 - 部门关联
// 3. 删除部门记录
// 4. 更新原父部门的子部门数量
// 5. 同步受影响用户的权限缓存
func (s *DeptService) executeDeleteTransaction(dept *model.Department, id uint) error {
	db, err := dept.GetDB()
	if err != nil {
		return e.NewBusinessError(1, "删除部门失败")
	}
	err = access.NewPermissionSyncCoordinator().RunAfterCommit(db, "删除部门后刷新权限缓存失败", func(tx *gorm.DB) error {
		dept.SetDB(tx)

		// 查询部门关联的所有用户 ID，用于后续权限同步
		affectedUserIDs, err := s.userIDsByDept(id, tx)
		if err != nil {
			return err
		}

		// 删除部门关联的所有角色
		deptRoleMap := model.NewDeptRoleMap()
		deptRoleMap.SetDB(tx)
		if err := deptRoleMap.DeleteWhere("dept_id = ?", id); err != nil {
			return err
		}

		// 删除用户 - 部门关联
		adminUserDeptMap := model.NewAdminUserDeptMap()
		adminUserDeptMap.SetDB(tx)
		if err := adminUserDeptMap.DeleteWhere("dept_id = ?", id); err != nil {
			return err
		}

		// 删除部门记录
		parentId := dept.Pid
		if _, err := dept.DeleteByID(id); err != nil {
			return err
		}
		// 更新原父部门的子部门数量（减 1）
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
