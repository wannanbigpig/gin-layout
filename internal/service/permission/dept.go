package permission

import (
	"fmt"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	casbinx "github.com/wannanbigpig/gin-layout/internal/pkg/utils/casbin"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	utils2 "github.com/wannanbigpig/gin-layout/pkg/utils"
)

const (
	maxDeptLevel = 5
	deptRootPid  = 0
)

// DeptService 部门服务
type DeptService struct {
	service.Base
}

// NewDeptService 创建部门服务实例
func NewDeptService() *DeptService {
	return &DeptService{}
}

// List 查询部门树形列表
func (s *DeptService) List(params *form.ListDept) any {
	condition, args := s.buildListCondition(params)

	deptModel := model.NewDepartment()
	depts := model.List(deptModel, condition, args, model.ListOptionalParams{
		OrderBy: "sort desc, id desc",
	})

	return resources.NewDeptTreeTransformer().BuildTreeByNode(depts, 0)
}

// buildListCondition 构建列表查询条件
func (s *DeptService) buildListCondition(params *form.ListDept) (string, []any) {
	var conditions []string
	var args []any

	if params.Name != "" {
		conditions = append(conditions, "name like ?")
		args = append(args, "%"+params.Name+"%")
	}

	if params.Pid != nil {
		conditions = append(conditions, "pid = ?")
		args = append(args, params.Pid)
	}

	return strings.Join(conditions, " AND "), args
}

// Edit 编辑部门（新增或更新）
func (s *DeptService) Edit(params *form.EditDept) error {
	dept := model.NewDepartment()
	editContext, err := s.prepareEditContext(dept, params)
	if err != nil {
		return err
	}

	// 处理父级变化
	if err := s.handleParentChange(dept, params); err != nil {
		return err
	}

	// 验证层级限制
	if dept.Level > maxDeptLevel {
		return e.NewBusinessError(1, "最多只能创建5级部门")
	}

	// 赋值部门字段
	s.assignDeptFields(dept, params)

	// 执行编辑事务
	return s.executeEditTransaction(dept, editContext.originPids, editContext.originPid)
}

// deptEditContext 部门编辑上下文信息
type deptEditContext struct {
	originPids string
	originPid  uint
}

// prepareEditContext 准备编辑上下文
func (s *DeptService) prepareEditContext(dept *model.Department, params *form.EditDept) (*deptEditContext, error) {
	ctx := &deptEditContext{originPids: "0", originPid: 0}

	if params.Id > 0 {
		if err := dept.GetById(dept, params.Id); err != nil || dept.ID == 0 {
			return nil, e.NewBusinessError(1, "编辑的部门不存在")
		}
		ctx.originPids = dept.Pids
		ctx.originPid = dept.Pid
	}

	return ctx, nil
}

// handleParentChange 处理父级变化
func (s *DeptService) handleParentChange(dept *model.Department, params *form.EditDept) error {
	if params.Pid > 0 && params.Pid != dept.Pid {
		return s.updateDeptWithParent(dept, params)
	}

	if params.Pid == deptRootPid {
		s.setRootDeptFields(dept)
	}

	dept.Pid = params.Pid
	return nil
}

// updateDeptWithParent 更新有父级的部门信息
func (s *DeptService) updateDeptWithParent(dept *model.Department, params *form.EditDept) error {
	var parentDept model.Department
	if err := parentDept.GetById(&parentDept, params.Pid); err != nil || parentDept.ID == 0 {
		return e.NewBusinessError(1, "上级部门不存在")
	}

	// 防止循环引用
	if dept.ID > 0 && utils2.WouldCauseCycle(dept.ID, params.Pid, parentDept.Pids) {
		return e.NewBusinessError(1, "上级部门不能是当前部门自身或其子部门")
	}

	dept.Level = parentDept.Level + 1
	dept.Pids = s.buildPids(parentDept.Pids, parentDept.ID)
	dept.Pid = params.Pid

	return nil
}

// setRootDeptFields 设置根部门字段
func (s *DeptService) setRootDeptFields(dept *model.Department) {
	dept.Level = 1
	dept.Pids = "0"
	dept.Pid = deptRootPid
}

// buildPids 构建父级ID序列
func (s *DeptService) buildPids(parentPids string, parentID uint) string {
	if parentPids == "0" || parentPids == "" {
		return fmt.Sprintf("%d", parentID)
	}
	return fmt.Sprintf("%s,%d", parentPids, parentID)
}

// assignDeptFields 赋值部门字段
func (s *DeptService) assignDeptFields(dept *model.Department, params *form.EditDept) {
	dept.Name = params.Name
	dept.Description = params.Description
	dept.Sort = params.Sort
	// 注意：dept.Pid 已在 handleParentChange 中设置
}

// executeEditTransaction 执行编辑事务
func (s *DeptService) executeEditTransaction(dept *model.Department, originPids string, originPid uint) error {
	return dept.DB().Transaction(func(tx *gorm.DB) error {
		// 保存部门信息
		if err := tx.Save(dept).Error; err != nil {
			return err
		}

		// 更新子部门层级
		if err := s.updateChildrenLevels(dept, originPids, tx); err != nil {
			return err
		}

		// 更新父级的 children_num
		// 如果 pid 发生变化，需要更新旧父级和新父级
		// 如果是新增操作（originPid = 0），只需要更新新父级
		if originPid > 0 && originPid != dept.Pid {
			// 更新旧父级的 children_num（编辑操作且父级发生变化时）
			if err := model.UpdateChildrenNum(model.NewDepartment(), originPid, tx); err != nil {
				return err
			}
		}
		// 更新新父级的 children_num（新增或父级变化时都需要更新）
		if dept.Pid > 0 && dept.Pid != originPid {
			if err := model.UpdateChildrenNum(model.NewDepartment(), dept.Pid, tx); err != nil {
				return err
			}
		}

		return nil
	})
}

// updateChildrenLevels 更新子部门层级
func (s *DeptService) updateChildrenLevels(dept *model.Department, originPids string, tx *gorm.DB) error {
	if dept.Pids == originPids {
		return nil
	}

	// 构建更新表达式
	updateExpr := s.buildPidsUpdateExpr(originPids, dept.Pids)

	// 更新子部门的 pids 和 level
	return tx.Model(model.NewDepartment()).
		Where("FIND_IN_SET(?,pids)", dept.ID).
		Updates(map[string]interface{}{
			"pids":  gorm.Expr(updateExpr),
			"level": gorm.Expr("length(pids) - length(replace(pids, ',', '')) + 1"),
		}).Error
}

// buildPidsUpdateExpr 构建pids更新表达式
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

// Delete 删除部门
func (s *DeptService) Delete(id uint) error {
	dept := model.NewDepartment()
	if err := dept.GetById(dept, id); err != nil || dept.ID == 0 {
		return e.NewBusinessError(1, "部门不存在")
	}

	// 检查是否有子部门（使用 children_num 字段判断，性能更好）
	if dept.ChildrenNum > 0 {
		return e.NewBusinessError(1, "该部门有子部门，无法删除")
	}

	// 执行删除事务
	return s.executeDeleteTransaction(dept, id)
}

// executeDeleteTransaction 执行删除事务
func (s *DeptService) executeDeleteTransaction(dept *model.Department, id uint) error {
	err := dept.DB().Transaction(func(tx *gorm.DB) error {
		// 删除部门角色关联
		deptRoleMap := model.NewDeptRoleMap()
		deptRoleMap.SetDB(tx)
		if err := deptRoleMap.DeleteWithCondition(deptRoleMap, "dept_id = ?", id); err != nil {
			return err
		}

		// 删除用户与部门的关联
		adminUsesDeptMap := model.NewAdminUsesDeptMap()
		adminUsesDeptMap.SetDB(tx)
		if err := adminUsesDeptMap.DeleteWithCondition(adminUsesDeptMap, "dept_id = ?", id); err != nil {
			return err
		}

		// 保存父级ID，用于后续更新 children_num
		parentId := dept.Pid

		// 删除部门
		if err := tx.Delete(dept, id).Error; err != nil {
			return err
		}

		// 更新父级的 children_num
		if parentId > 0 {
			if err := model.UpdateChildrenNum(model.NewDepartment(), parentId, tx); err != nil {
				return err
			}
		}

		// 删除Casbin策略（会自动删除所有引用该部门的策略）
		return s.deleteAllPoliciesForDept(id, tx)
	})

	if err != nil {
		// 如果事务失败，重新加载策略以确保一致性
		_ = casbinx.GetEnforcer().LoadPolicy()
		return e.NewBusinessError(1, "删除部门失败")
	}

	return nil
}

// Detail 获取部门详情
func (s *DeptService) Detail(id uint) (any, error) {
	dept := model.NewDepartment()
	if err := dept.DB().Preload("RoleList").First(dept, id).Error; err != nil || dept.ID == 0 {
		return nil, e.NewBusinessError(1, "部门不存在")
	}
	return resources.NewDeptTreeTransformer().ToStruct(dept), nil
}

// GetDeptRoles 获取部门的所有角色（直接绑定的角色）
func (s *DeptService) GetDeptRoles(deptId uint) ([]string, error) {
	dept := model.NewDepartment()
	if err := dept.GetById(dept, deptId); err != nil || dept.ID == 0 {
		return nil, e.NewBusinessError(1, "部门不存在")
	}
	return s.getImplicitRolesForDept(deptId)
}

// getImplicitRolesForDept 获取部门的所有角色（直接绑定的角色）
func (s *DeptService) getImplicitRolesForDept(deptId uint) ([]string, error) {
	enforcer := casbinx.GetEnforcer()
	if enforcer.Error() != nil {
		return nil, e.NewBusinessError(1, "获取失败")
	}
	deptName := fmt.Sprintf("%s%s%d", global.CasbinDeptPrefix, global.CasbinSeparator, deptId)
	permissions, err := enforcer.GetImplicitRolesForUser(deptName)
	if err != nil {
		return nil, e.NewBusinessError(1, "获取失败~")
	}
	return permissions, nil
}

// deleteAllPoliciesForDept 删除部门的所有Casbin策略
// 包括：
// 1. 部门作为 user 的所有策略（部门关联的角色）
// 2. 所有引用该部门的策略（用户引用该部门的策略）
func (s *DeptService) deleteAllPoliciesForDept(deptId uint, tx *gorm.DB) error {
	enforcer := casbinx.GetEnforcer()
	if enforcer == nil || enforcer.Error() != nil {
		return nil
	}

	enforcer.SetDB(tx)
	deptName := fmt.Sprintf("%s%s%d", global.CasbinDeptPrefix, global.CasbinSeparator, deptId)

	return enforcer.WithTransaction(func(e casbin.IEnforcer) error {
		// 删除部门作为 user 的所有策略（部门关联的角色）
		_, _ = e.DeleteRolesForUser(deptName)

		// 删除所有引用该部门的策略（用户引用该部门的策略）
		// 使用 RemoveFilteredGroupingPolicy 删除所有第二个参数匹配的策略
		// [g, adminUser:*, dept:deptId]
		_, _ = e.RemoveFilteredGroupingPolicy(1, deptName)

		return nil
	})
}

// BindRole 绑定角色到部门
func (s *DeptService) BindRole(params *form.BindRole) error {
	deptModel := model.NewDepartment()
	if err := deptModel.GetById(deptModel, params.Id); err != nil || deptModel.ID == 0 {
		return e.NewBusinessError(e.FAILURE, "部门不存在")
	}

	// 验证角色ID有效性
	roleIds, err := model.VerifyExistingIDs(model.NewRole(), params.RoleIds)
	if err != nil {
		return e.NewBusinessError(e.FAILURE, "判断角色是否存在失败")
	}

	// 执行绑定事务
	return s.executeBindRoleTransaction(deptModel.ID, roleIds)
}

// executeBindRoleTransaction 执行绑定角色事务
func (s *DeptService) executeBindRoleTransaction(deptId uint, roleIds []uint) error {
	err := model.DB().Transaction(func(tx *gorm.DB) error {
		return s.updateDeptRole(deptId, roleIds, tx)
	})

	if err != nil {
		_ = casbinx.GetEnforcer().LoadPolicy()
		return e.NewBusinessError(e.FAILURE, "绑定角色失败")
	}

	return nil
}

// updateDeptRole 更新部门角色关联
func (s *DeptService) updateDeptRole(deptId uint, roleIds []uint, tx ...*gorm.DB) error {
	deptRoleMap := model.NewDeptRoleMap()
	if len(tx) > 0 {
		deptRoleMap.SetDB(tx[0])
	}

	// 获取现有关联
	existingIds, err := model.ExtractColumnsByCondition[model.DeptRoleMap, *model.DeptRoleMap, uint](
		deptRoleMap,
		"role_id",
		"dept_id = ?",
		deptId,
	)
	if err != nil {
		return err
	}

	// 计算差集
	toDelete, toAdd, remainingList := utils.CalculateChanges(existingIds, roleIds)

	// 删除不再需要的关联
	if len(toDelete) > 0 {
		if err := deptRoleMap.DeleteWithCondition(
			deptRoleMap,
			"dept_id = ? AND role_id IN (?)",
			[]any{deptId, toDelete}...,
		); err != nil {
			return err
		}
	}

	// 批量创建新关联
	if len(toAdd) > 0 {
		newMappings := lo.Map(toAdd, func(roleId uint, _ int) *model.DeptRoleMap {
			return &model.DeptRoleMap{
				RoleId: roleId,
				DeptId: deptId,
			}
		})
		if err := deptRoleMap.BatchCreate(newMappings); err != nil {
			return err
		}
	}

	// 更新Casbin策略（如果 remainingList 为空，会自动删除所有策略）
	return s.editDeptPolicyRoles(deptId, global.CasbinRolePrefix, remainingList, tx...)
}

// editDeptPolicyRoles 编辑部门的策略角色
func (s *DeptService) editDeptPolicyRoles(deptId uint, childPrefix string, childIds []uint, tx ...*gorm.DB) error {
	enforcer := casbinx.GetEnforcer()
	if enforcer.Error() != nil {
		return e.NewBusinessError(1, "编辑失败")
	}
	// 设置事务，如果没有传入事务则清理之前的事务状态
	if len(tx) > 0 {
		enforcer.SetDB(tx[0])
	} else {
		enforcer.SetDB(nil)
	}
	deptName := fmt.Sprintf("%s%s%d", global.CasbinDeptPrefix, global.CasbinSeparator, deptId)
	// 如果 childIds 为空，直接删除所有策略
	if len(childIds) == 0 {
		_, err := enforcer.Enforcer.DeleteRolesForUser(deptName)
		if err != nil {
			return err
		}
		return nil
	}
	policy := lo.Map(childIds, func(id uint, _ int) string {
		return fmt.Sprintf("%s:%d", childPrefix, id)
	})
	err := enforcer.EditPolicyRoles(deptName, policy)
	if err != nil {
		return e.NewBusinessError(1, "编辑失败~")
	}
	return nil
}
