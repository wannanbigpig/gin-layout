package dept

import (
	"fmt"

	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/query_builder"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	utils2 "github.com/wannanbigpig/gin-layout/pkg/utils"
)

const (
	maxDeptLevel = 5
	deptRootPid  = 0
)

// DeptService 处理部门的增删改查和角色绑定。
type DeptService struct {
	service.Base
}

// NewDeptService 创建部门服务实例。
func NewDeptService() *DeptService {
	return &DeptService{}
}

func (s *DeptService) resolveDB(tx ...*gorm.DB) (*gorm.DB, error) {
	if db := access.FirstTx(tx); db != nil {
		return db, nil
	}
	return model.GetDB()
}

// List 返回部门树形列表。
func (s *DeptService) List(params *form.ListDept) any {
	condition, args := s.buildListCondition(params)

	deptModel := model.NewDepartment()
	depts, err := model.ListE(deptModel, condition, args, model.ListOptionalParams{
		OrderBy: "sort desc, id desc",
	})
	if err != nil {
		return resources.NewDeptTreeTransformer().BuildTreeByNode(nil, 0)
	}

	return resources.NewDeptTreeTransformer().BuildTreeByNode(depts, 0)
}

// buildListCondition 构建部门列表查询条件。
func (s *DeptService) buildListCondition(params *form.ListDept) (string, []any) {
	return query_builder.New().
		AddLike("name", params.Name).
		AddEq("pid", params.Pid).
		Build()
}

// Create 新增部门。
func (s *DeptService) Create(params *form.CreateDept) error {
	return s.edit(&deptMutation{
		Name:        params.Name,
		Pid:         params.Pid,
		Description: params.Description,
		Sort:        params.Sort,
	})
}

// Update 更新部门。
func (s *DeptService) Update(params *form.UpdateDept) error {
	return s.edit(&deptMutation{
		Id:          params.Id,
		Name:        params.Name,
		Pid:         params.Pid,
		Description: params.Description,
		Sort:        params.Sort,
	})
}

// Edit 兼容旧编辑入口，等同于更新。
func (s *DeptService) Edit(params *form.UpdateDept) error {
	return s.Update(params)
}

type deptMutation struct {
	Id          uint
	Name        string
	Pid         uint
	Description string
	Sort        uint
}

func (s *DeptService) edit(params *deptMutation) error {
	dept := model.NewDepartment()
	editContext, err := s.prepareEditContext(dept, params)
	if err != nil {
		return err
	}
	if params.Id > 0 && access.NewSystemDefaultsService().IsProtectedDepartment(dept) {
		if params.Pid != dept.Pid || params.Sort != dept.Sort {
			return e.NewBusinessError(e.FAILURE, "默认部门只允许修改名称和描述")
		}
	}

	if err := s.handleParentChange(dept, params); err != nil {
		return err
	}

	if dept.Level > maxDeptLevel {
		return e.NewBusinessError(1, "最多只能创建5级部门")
	}

	s.assignDeptFields(dept, params)

	return s.executeEditTransaction(dept, editContext.originPids, editContext.originPid)
}

// deptEditContext 保存编辑前的父级信息。
type deptEditContext struct {
	originPids string
	originPid  uint
}

// prepareEditContext 加载编辑前的部门状态。
func (s *DeptService) prepareEditContext(dept *model.Department, params *deptMutation) (*deptEditContext, error) {
	ctx := &deptEditContext{originPids: "0", originPid: 0}

	if params.Id > 0 {
		if err := dept.GetById(params.Id); err != nil || dept.ID == 0 {
			return nil, e.NewBusinessError(1, "编辑的部门不存在")
		}
		ctx.originPids = dept.Pids
		ctx.originPid = dept.Pid
	}

	return ctx, nil
}

// handleParentChange 根据新父级重建层级字段。
func (s *DeptService) handleParentChange(dept *model.Department, params *deptMutation) error {
	if params.Pid > 0 && params.Pid != dept.Pid {
		return s.updateDeptWithParent(dept, params)
	}

	if params.Pid == deptRootPid {
		s.setRootDeptFields(dept)
	}

	dept.Pid = params.Pid
	return nil
}

// updateDeptWithParent 根据父部门更新当前部门层级信息。
func (s *DeptService) updateDeptWithParent(dept *model.Department, params *deptMutation) error {
	parentDept := model.NewDepartment()
	if err := parentDept.GetById(params.Pid); err != nil || parentDept.ID == 0 {
		return e.NewBusinessError(1, "上级部门不存在")
	}

	if dept.ID > 0 && utils2.WouldCauseCycle(dept.ID, params.Pid, parentDept.Pids) {
		return e.NewBusinessError(1, "上级部门不能是当前部门自身或其子部门")
	}

	dept.Level = parentDept.Level + 1
	dept.Pids = s.buildPids(parentDept.Pids, parentDept.ID)
	dept.Pid = params.Pid

	return nil
}

// setRootDeptFields 重置根部门的层级字段。
func (s *DeptService) setRootDeptFields(dept *model.Department) {
	dept.Level = 1
	dept.Pids = "0"
	dept.Pid = deptRootPid
}

// buildPids 构建逗号分隔的父级链路。
func (s *DeptService) buildPids(parentPids string, parentID uint) string {
	if parentPids == "0" || parentPids == "" {
		return fmt.Sprintf("%d", parentID)
	}
	return fmt.Sprintf("%s,%d", parentPids, parentID)
}

// assignDeptFields 回填可直接写入部门表的字段。
func (s *DeptService) assignDeptFields(dept *model.Department, params *deptMutation) {
	dept.Name = params.Name
	dept.Description = params.Description
	dept.Sort = params.Sort
}

// executeEditTransaction 持久化部门并同步层级统计。
func (s *DeptService) executeEditTransaction(dept *model.Department, originPids string, originPid uint) error {
	db, err := dept.GetDB()
	if err != nil {
		return err
	}
	return access.RunInTransaction(db, func(tx *gorm.DB) error {
		dept.SetDB(tx)

		if err := dept.Save(); err != nil {
			return err
		}

		if err := s.updateChildrenLevels(dept, originPids, tx); err != nil {
			return err
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

// updateChildrenLevels 在父链变化后批量修正子部门层级。
func (s *DeptService) updateChildrenLevels(dept *model.Department, originPids string, tx *gorm.DB) error {
	if dept.Pids == originPids {
		return nil
	}

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
	if err := dept.GetById(id); err != nil || dept.ID == 0 {
		return e.NewBusinessError(1, "部门不存在")
	}
	if access.NewSystemDefaultsService().IsProtectedDepartment(dept) {
		return e.NewBusinessError(e.FAILURE, "默认部门不允许删除")
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

		// 删除部门角色关联
		deptRoleMap := model.NewDeptRoleMap()
		deptRoleMap.SetDB(tx)
		if err := deptRoleMap.DeleteWhere("dept_id = ?", id); err != nil {
			return err
		}

		// 删除用户与部门的关联
		adminUserDeptMap := model.NewAdminUserDeptMap()
		adminUserDeptMap.SetDB(tx)
		if err := adminUserDeptMap.DeleteWhere("dept_id = ?", id); err != nil {
			return err
		}

		// 保存父级ID，用于后续更新 children_num
		parentId := dept.Pid

		// 删除部门
		if _, err := dept.DeleteByID(id); err != nil {
			return err
		}

		// 更新父级的 children_num
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

// Detail 获取部门详情
func (s *DeptService) Detail(id uint) (any, error) {
	dept := model.NewDepartment()
	db, err := dept.GetDB()
	if err != nil {
		return nil, e.NewBusinessError(1, "部门不存在")
	}
	if err := db.Preload("RoleList").First(dept, id).Error; err != nil || dept.ID == 0 {
		return nil, e.NewBusinessError(1, "部门不存在")
	}
	return resources.NewDeptTreeTransformer().ToStruct(dept), nil
}

// BindRole 绑定角色到部门
func (s *DeptService) BindRole(params *form.BindRole) error {
	deptModel := model.NewDepartment()
	if err := deptModel.GetById(params.Id); err != nil || deptModel.ID == 0 {
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
	db, err := model.GetDB()
	if err != nil {
		return e.NewBusinessError(e.FAILURE, "绑定角色失败")
	}
	err = access.NewPermissionSyncCoordinator().RunAfterCommit(db, "绑定角色后刷新权限缓存失败", func(tx *gorm.DB) error {
		return s.updateDeptRole(deptId, roleIds, tx)
	})

	if err != nil {
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
	toDelete, toAdd, _ := utils.CalculateChanges(existingIds, roleIds)

	// 删除不再需要的关联
	if len(toDelete) > 0 {
		if err := deptRoleMap.DeleteWhere(
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
		if err := deptRoleMap.CreateBatch(newMappings); err != nil {
			return err
		}
	}

	userIDs, err := s.userIDsByDept(deptId, tx...)
	if err != nil {
		return err
	}
	return access.NewPermissionSyncCoordinator().SyncUsers(userIDs, tx...)
}

func (s *DeptService) userIDsByDept(deptId uint, tx ...*gorm.DB) ([]uint, error) {
	db, err := s.resolveDB(tx...)
	if err != nil {
		return nil, err
	}
	return access.NewUserPermissionSyncService().QueryUintColumn(db.Table("admin_user_department_map").Where("dept_id = ?", deptId), "uid")
}
