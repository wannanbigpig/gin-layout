package dept

import (
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

func (s *DeptService) buildListCondition(params *form.ListDept) (string, []any) {
	return query_builder.New().
		AddLike("name", params.Name).
		AddEq("pid", params.Pid).
		Build()
}

// Create 新增部门。
func (s *DeptService) Create(params *form.CreateDept) error {
	return s.applyDeptMutation(&deptMutation{
		Name:        params.Name,
		Pid:         params.Pid,
		Description: params.Description,
		Sort:        params.Sort,
	})
}

// Update 更新部门。
func (s *DeptService) Update(params *form.UpdateDept) error {
	return s.applyDeptMutation(&deptMutation{
		Id:          params.Id,
		Name:        params.Name,
		Pid:         params.Pid,
		Description: params.Description,
		Sort:        params.Sort,
	})
}

// Delete 删除部门。
func (s *DeptService) Delete(id uint) error {
	dept := model.NewDepartment()
	if err := dept.GetById(id); err != nil || dept.ID == 0 {
		return e.NewBusinessError(1, "部门不存在")
	}
	if access.NewSystemDefaultsService().IsProtectedDepartment(dept) {
		return e.NewBusinessError(e.FAILURE, "默认部门不允许删除")
	}
	if dept.ChildrenNum > 0 {
		return e.NewBusinessError(1, "该部门有子部门，无法删除")
	}

	return s.executeDeleteTransaction(dept, id)
}

// Detail 获取部门详情。
func (s *DeptService) Detail(id uint) (any, error) {
	dept := model.NewDepartment()
	if err := dept.GetAllById(id); err != nil || dept.ID == 0 {
		return nil, e.NewBusinessError(1, "部门不存在")
	}
	return resources.NewDeptTreeTransformer().ToStruct(dept), nil
}

// BindRole 绑定角色到部门。
func (s *DeptService) BindRole(params *form.BindRole) error {
	deptModel := model.NewDepartment()
	if err := deptModel.GetById(params.Id); err != nil || deptModel.ID == 0 {
		return e.NewBusinessError(e.FAILURE, "部门不存在")
	}

	roleIds, err := model.VerifyExistingIDs(model.NewRole(), params.RoleIds)
	if err != nil {
		return e.NewBusinessError(e.FAILURE, "判断角色是否存在失败")
	}

	db, err := model.NewDepartment().GetDB()
	if err != nil {
		return e.NewBusinessError(e.FAILURE, "绑定角色失败")
	}
	err = access.NewPermissionSyncCoordinator().RunAfterCommit(db, "绑定角色后刷新权限缓存失败", func(tx *gorm.DB) error {
		return s.updateDeptRole(deptModel.ID, roleIds, tx)
	})
	if err != nil {
		return e.NewBusinessError(e.FAILURE, "绑定角色失败")
	}
	return nil
}

// updateDeptRole 更新部门关联的角色，使用差分算法只变更差异部分。
// 处理逻辑：
// 1. 查询部门当前已关联的角色 ID 列表
// 2. 计算差异：toDelete 需删除，toAdd 需新增
// 3. 批量删除/新增角色关联
// 4. 同步部门下所有用户的权限缓存
func (s *DeptService) updateDeptRole(deptId uint, roleIds []uint, tx ...*gorm.DB) error {
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
		return err
	}

	// 计算差异
	toDelete, toAdd, _ := utils.CalculateChanges(existingIds, roleIds)
	// 批量删除差异角色关联
	if len(toDelete) > 0 {
		if err := deptRoleMap.DeleteWhere("dept_id = ? AND role_id IN (?)", []any{deptId, toDelete}...); err != nil {
			return err
		}
	}

	// 批量新增角色关联
	if len(toAdd) > 0 {
		newMappings := lo.Map(toAdd, func(roleId uint, _ int) *model.DeptRoleMap {
			return &model.DeptRoleMap{RoleId: roleId, DeptId: deptId}
		})
		if err := deptRoleMap.CreateBatch(newMappings); err != nil {
			return err
		}
	}

	// 同步部门下所有用户的权限缓存
	userIDs, err := s.userIDsByDept(deptId, tx...)
	if err != nil {
		return err
	}
	return access.NewPermissionSyncCoordinator().SyncUsers(userIDs, tx...)
}

// userIDsByDept 查询部门下的所有用户 ID 列表。
func (s *DeptService) userIDsByDept(deptId uint, tx ...*gorm.DB) ([]uint, error) {
	deptMapModel := model.NewAdminUserDeptMap()
	if t := access.FirstTx(tx); t != nil {
		deptMapModel.SetDB(t)
	}
	return deptMapModel.UidsByDeptIds([]uint{deptId})
}
