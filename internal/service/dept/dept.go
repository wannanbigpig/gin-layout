package dept

import (
	"context"
	"errors"

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

// List 返回部门树形列表。
func (s *DeptService) List(params *form.ListDept) (any, error) {
	condition, args := query_builder.New().
		AddLike("name", params.Name).
		AddEq("pid", params.Pid).
		Build()

	deptModel := model.NewDepartment()
	depts, err := model.ListE(deptModel, condition, args, model.ListOptionalParams{
		OrderBy: "sort desc, id desc",
	})
	transformer := resources.NewDeptTreeTransformer()
	if err != nil {
		return nil, err
	}

	// 命中筛选条件时返回扁平结果：按条件过滤后再从 root=0 建树会丢失非顶层匹配项。
	if params.Name != "" || params.Pid != nil {
		return transformer.BuildFlat(depts), nil
	}

	return transformer.BuildTreeByNode(depts, 0), nil
}

// Create 新增部门。
func (s *DeptService) Create(params *form.CreateDept) error {
	_, err := s.applyDeptMutation(&deptMutation{
		Name:        params.Name,
		Pid:         params.Pid,
		Description: params.Description,
		Sort:        params.Sort,
	})
	return err
}

// Update 更新部门。
func (s *DeptService) Update(params *form.UpdateDept) error {
	_, err := s.applyDeptMutation(&deptMutation{
		Id:          params.Id,
		Name:        params.Name,
		Pid:         params.Pid,
		Description: params.Description,
		Sort:        params.Sort,
	})
	return err
}

// Delete 删除部门。
func (s *DeptService) Delete(id uint) error {
	dept := model.NewDepartment()
	if err := dept.GetById(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return e.NewBusinessError(e.DepartmentNotFound)
		}
		return err
	}
	if dept.ID == 0 {
		return e.NewBusinessError(e.DepartmentNotFound)
	}
	if access.NewSystemDefaultsService().IsProtectedDepartment(dept) {
		return e.NewBusinessError(e.DepartmentCannotDelete, "系统内置部门不能删除")
	}
	if dept.ChildrenNum > 0 {
		return e.NewBusinessError(e.DepartmentHasChildren)
	}

	return s.executeDeleteTransaction(dept, id)
}

// Detail 获取部门详情。
func (s *DeptService) Detail(id uint) (any, error) {
	dept := model.NewDepartment()
	if err := dept.GetAllById(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, e.NewBusinessError(e.DepartmentNotFound)
		}
		return nil, err
	}
	if dept.ID == 0 {
		return nil, e.NewBusinessError(e.DepartmentNotFound)
	}
	return resources.NewDeptTreeTransformer().ToStruct(dept), nil
}

// BindRole 绑定角色到部门。
func (s *DeptService) BindRole(params *form.DeptBindRole) error {
	deptModel := model.NewDepartment()
	if err := deptModel.GetById(params.DeptId); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return e.NewBusinessError(e.DepartmentNotFound)
		}
		return err
	}
	if deptModel.ID == 0 {
		return e.NewBusinessError(e.DepartmentNotFound)
	}

	roleIds, err := model.VerifyExistingIDs(model.NewRole(), params.RoleIds)
	if err != nil {
		return err
	}

	db, err := model.NewDepartment().GetDB()
	if err != nil {
		return err
	}
	var affectedUserIDs []uint
	err = access.NewPermissionSyncCoordinator().RunAfterCommitWithCode(db, e.FAILURE, func(tx *gorm.DB) error {
		var updateErr error
		affectedUserIDs, updateErr = s.updateDeptRole(deptModel.ID, roleIds, tx)
		return updateErr
	})
	if err != nil {
		return err
	}
	access.NewDataScopeFilter().InvalidateCache(context.Background(), affectedUserIDs...)
	return nil
}

// updateDeptRole 更新部门关联的角色，使用差分算法只变更差异部分。
// 处理逻辑：
// 1. 查询部门当前已关联的角色 ID 列表
// 2. 计算差异：toDelete 需删除，toAdd 需新增
// 3. 批量删除/新增角色关联
// 4. 同步部门下所有用户的权限缓存
func (s *DeptService) updateDeptRole(deptId uint, roleIds []uint, tx ...*gorm.DB) ([]uint, error) {
	deptRoleMap := model.NewDeptRoleMap()
	if len(tx) > 0 {
		deptRoleMap.SetDB(tx[0])
	}

	// 查询部门当前已关联的角色 ID 列表
	existingIds, err := model.ExtractColumnsByCondition[model.DeptRoleMap, *model.DeptRoleMap, uint](
		deptRoleMap,
		"role_id",
		"dept_id = ?",
		deptId,
	)
	if err != nil {
		return nil, err
	}

	// 计算差异
	toDelete, toAdd, _ := utils.CalculateChanges(existingIds, roleIds)
	// 批量删除差异角色关联
	if len(toDelete) > 0 {
		if err := deptRoleMap.DeleteWhere("dept_id = ? AND role_id IN (?)", []any{deptId, toDelete}...); err != nil {
			return nil, err
		}
	}

	// 批量新增角色关联
	if len(toAdd) > 0 {
		newMappings := lo.Map(toAdd, func(roleId uint, _ int) *model.DeptRoleMap {
			return &model.DeptRoleMap{RoleId: roleId, DeptId: deptId}
		})
		if err := deptRoleMap.CreateBatch(newMappings); err != nil {
			return nil, err
		}
	}

	// 同步部门下所有用户的权限缓存
	userIDs, err := s.userIDsByDept(deptId, tx...)
	if err != nil {
		return nil, err
	}
	if err := access.NewPermissionSyncCoordinator().SyncUsers(userIDs, tx...); err != nil {
		return nil, err
	}
	return userIDs, nil
}

// userIDsByDept 查询部门下的所有用户 ID 列表。
func (s *DeptService) userIDsByDept(deptId uint, tx ...*gorm.DB) ([]uint, error) {
	deptMapModel := model.NewAdminUserDeptMap()
	if t := access.FirstTx(tx); t != nil {
		deptMapModel.SetDB(t)
	}
	return deptMapModel.UidsByDeptIds([]uint{deptId})
}
