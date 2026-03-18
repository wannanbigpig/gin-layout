package access

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	utils2 "github.com/wannanbigpig/gin-layout/pkg/utils"
)

const (
	maxRoleLevel      = 2 // 最多2层（即3级：顶级、二级、三级）
	maxChildrenPerTop = 5 // 每个顶级角色下最多5个子角色
)

// RoleService 角色服务
type RoleService struct {
	service.Base
}

// NewRoleService 创建角色服务实例
func NewRoleService() *RoleService {
	return &RoleService{}
}

// List 分页查询角色列表
func (s *RoleService) List(params *form.RoleList) interface{} {
	condition, args := s.buildListCondition(params)

	roleModel := model.NewRole()
	total, collection, err := model.ListPageE(
		roleModel,
		params.Page,
		params.PerPage,
		condition,
		args,
		model.ListOptionalParams{
			OrderBy: "sort desc, id desc",
		},
	)
	if err != nil {
		return resources.ToRawCollection(params.Page, params.PerPage, 0, make([]*model.Role, 0))
	}

	return resources.ToRawCollection(params.Page, params.PerPage, total, collection)
}

// buildListCondition 构建列表查询条件
func (s *RoleService) buildListCondition(params *form.RoleList) (string, []any) {
	var conditions []string
	var args []any

	if params.Name != "" {
		conditions = append(conditions, "name like ?")
		args = append(args, "%"+params.Name+"%")
	}

	if params.Status != nil {
		conditions = append(conditions, "status = ?")
		args = append(args, params.Status)
	}

	// 父级ID过滤
	if params.Pid != nil {
		conditions = append(conditions, "pid = ?")
		args = append(args, params.Pid)
	}

	return strings.Join(conditions, " AND "), args
}

// Create 新增角色。
func (s *RoleService) Create(params *form.CreateRole) error {
	return s.edit(&roleMutation{
		Name:        params.Name,
		Description: params.Description,
		Status:      params.Status,
		Pid:         params.Pid,
		Sort:        params.Sort,
		MenuList:    params.MenuList,
	})
}

// Update 更新角色。
func (s *RoleService) Update(params *form.UpdateRole) error {
	return s.edit(&roleMutation{
		Id:          params.Id,
		Name:        params.Name,
		Description: params.Description,
		Status:      params.Status,
		Pid:         params.Pid,
		Sort:        params.Sort,
		MenuList:    params.MenuList,
	})
}

// Edit 兼容旧编辑入口，等同于更新。
func (s *RoleService) Edit(params *form.UpdateRole) error {
	return s.Update(params)
}

type roleMutation struct {
	Id          uint
	Name        string
	Description string
	Status      uint8
	Pid         uint
	Sort        uint
	MenuList    []uint
}

func (s *RoleService) edit(params *roleMutation) error {
	role := model.NewRole()
	editContext, err := s.prepareEditContext(role, params)
	if err != nil {
		return err
	}
	if params.Id > 0 && NewSystemDefaultsService().IsProtectedRole(role) {
		return e.NewBusinessError(e.FAILURE, "系统默认超级管理员角色不允许修改")
	}

	// 处理父级变化
	if err := s.handleParentChange(role, params); err != nil {
		return err
	}

	// 验证层级限制
	if role.Level > maxRoleLevel {
		return e.NewBusinessError(1, "最多只能创建2层角色")
	}

	// 赋值角色字段
	s.assignRoleFields(role, params)

	// 验证菜单ID有效性
	menuList, err := model.VerifyExistingIDs(model.NewMenu(), params.MenuList)
	if err != nil {
		return e.NewBusinessError(e.FAILURE, "判断菜单是否存在失败")
	}

	// 执行编辑事务
	return s.executeEditTransaction(role, menuList, editContext.originPids, editContext.originPid)
}

// roleEditContext 角色编辑上下文信息
type roleEditContext struct {
	originPids string
	originPid  uint
}

// prepareEditContext 准备编辑上下文
func (s *RoleService) prepareEditContext(role *model.Role, params *roleMutation) (*roleEditContext, error) {
	ctx := &roleEditContext{originPids: "0", originPid: 0}

	if params.Id > 0 {
		if err := role.GetById(params.Id); err != nil || role.ID == 0 {
			return nil, e.NewBusinessError(1, "编辑的角色不存在")
		}
		ctx.originPids = role.Pids
		ctx.originPid = role.Pid
	}

	return ctx, nil
}

// handleParentChange 处理父级变化
func (s *RoleService) handleParentChange(role *model.Role, params *roleMutation) error {
	if params.Pid > 0 && params.Pid != role.Pid {
		return s.updateRoleWithParent(role, params)
	}

	if params.Pid == 0 {
		s.setRootRoleFields(role)
	}

	role.Pid = params.Pid
	return nil
}

// updateRoleWithParent 更新有父级的角色信息
func (s *RoleService) updateRoleWithParent(role *model.Role, params *roleMutation) error {
	parentRole := model.NewRole()
	if err := parentRole.GetById(params.Pid); err != nil || parentRole.ID == 0 {
		return e.NewBusinessError(1, "上级角色不存在")
	}

	// 防止循环引用
	if role.ID > 0 && utils2.WouldCauseCycle(role.ID, params.Pid, parentRole.Pids) {
		return e.NewBusinessError(1, "上级角色不能是当前角色自身或其子角色")
	}

	// 检查顶级角色下的子角色数量限制
	if parentRole.Pid == 0 {
		// 父角色是顶级角色，使用 children_num 字段检查其子角色数量
		// 如果是编辑操作且父级未变化，不需要检查（因为当前角色已经计算在内）
		// 如果是新增或父级变化，需要检查是否超过限制
		if role.ID == 0 || role.Pid != params.Pid {
			if parentRole.ChildrenNum >= maxChildrenPerTop {
				return e.NewBusinessError(1, fmt.Sprintf("每个顶级角色下最多只能创建%d个子角色", maxChildrenPerTop))
			}
		}
	}

	role.Level = parentRole.Level + 1
	role.Pids = s.buildPids(parentRole.Pids, parentRole.ID)
	role.Pid = params.Pid

	return nil
}

// setRootRoleFields 设置根角色字段
func (s *RoleService) setRootRoleFields(role *model.Role) {
	role.Level = 1
	role.Pids = "0"
	role.Pid = 0
}

// buildPids 构建父级ID序列
func (s *RoleService) buildPids(parentPids string, parentID uint) string {
	if parentPids == "0" || parentPids == "" {
		return fmt.Sprintf("%d", parentID)
	}
	return fmt.Sprintf("%s,%d", parentPids, parentID)
}

// assignRoleFields 赋值角色字段
func (s *RoleService) assignRoleFields(role *model.Role, params *roleMutation) {
	role.Name = params.Name
	role.Description = params.Description
	role.Status = params.Status
	role.Sort = params.Sort
	// 注意：role.Pid 已在 handleParentChange 中设置
}

// executeEditTransaction 执行编辑事务
func (s *RoleService) executeEditTransaction(role *model.Role, menuList []uint, originPids string, originPid uint) error {
	db, err := role.GetDB()
	if err != nil {
		return err
	}
	err = runInTransaction(db, func(tx *gorm.DB) error {
		role.SetDB(tx)

		// 保存角色信息
		if err := role.Save(); err != nil {
			return err
		}

		// 更新子角色层级
		if err := s.updateChildrenLevels(role, originPids, tx); err != nil {
			return err
		}

		// 更新父级的 children_num
		// 如果 pid 发生变化，需要更新旧父级和新父级
		// 如果是新增操作（originPid = 0），只需要更新新父级
		if originPid > 0 && originPid != role.Pid {
			// 更新旧父级的 children_num（编辑操作且父级发生变化时）
			if err := model.UpdateChildrenNum(model.NewRole(), originPid, tx); err != nil {
				return err
			}
		}
		// 更新新父级的 children_num（新增或父级变化时都需要更新）
		if role.Pid > 0 && role.Pid != originPid {
			if err := model.UpdateChildrenNum(model.NewRole(), role.Pid, tx); err != nil {
				return err
			}
		}

		// 更新角色菜单关联
		if err := s.updateRoleMenu(role.ID, menuList, tx); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	return NewPermissionSyncCoordinator().SyncAll()
}

// updateChildrenLevels 更新子角色层级
func (s *RoleService) updateChildrenLevels(role *model.Role, originPids string, tx *gorm.DB) error {
	if role.Pids == originPids {
		return nil
	}

	// 构建更新表达式
	updateExpr := s.buildPidsUpdateExpr(originPids, role.Pids)

	// 获取所有子角色ID
	var childRoleIds []uint
	if err := tx.Model(model.NewRole()).
		Where("FIND_IN_SET(?,pids)", role.ID).
		Pluck("id", &childRoleIds).Error; err != nil {
		return err
	}

	// 更新子角色的 pids 和 level
	if err := tx.Model(model.NewRole()).
		Where("FIND_IN_SET(?,pids)", role.ID).
		Updates(map[string]interface{}{
			"pids":  gorm.Expr(updateExpr),
			"level": gorm.Expr("length(pids) - length(replace(pids, ',', '')) + 1"),
		}).Error; err != nil {
		return err
	}

	return nil
}

// buildPidsUpdateExpr 构建pids更新表达式
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

// updateRoleMenu 更新角色菜单关联
func (s *RoleService) updateRoleMenu(roleId uint, menuList []uint, tx ...*gorm.DB) error {
	roleMenuMap := model.NewRoleMenuMap()
	if len(tx) > 0 {
		roleMenuMap.SetDB(tx[0])
	}

	// 保存用户实际选择的菜单到数据库（不包含自动添加的父级目录）
	// 这样回显时只显示用户实际选择的菜单

	// 获取现有关联
	existingIds, err := model.ExtractColumnsByCondition[model.RoleMenuMap, *model.RoleMenuMap, uint](
		roleMenuMap,
		"menu_id",
		"role_id = ?",
		roleId,
	)
	if err != nil {
		return err
	}

	// 计算差集（基于用户实际选择的菜单）
	toDelete, toAdd, _ := utils.CalculateChanges(existingIds, menuList)

	// 删除不再需要的关联
	if len(toDelete) > 0 {
		if err := roleMenuMap.DeleteWhere(
			"role_id = ? AND menu_id IN (?)",
			[]any{roleId, toDelete}...,
		); err != nil {
			return err
		}
	}

	// 批量创建新关联（只保存用户实际选择的菜单）
	if len(toAdd) > 0 {
		newMappings := lo.Map(toAdd, func(menuId uint, _ int) *model.RoleMenuMap {
			return &model.RoleMenuMap{
				RoleId: roleId,
				MenuId: menuId,
			}
		})
		if err := roleMenuMap.CreateBatch(newMappings); err != nil {
			return err
		}
	}

	return nil
}

// Delete 删除角色
func (s *RoleService) Delete(id uint) error {
	role := model.NewRole()
	if err := role.GetById(id); err != nil || role.ID == 0 {
		return e.NewBusinessError(1, "角色不存在")
	}
	if NewSystemDefaultsService().IsProtectedRole(role) {
		return e.NewBusinessError(e.FAILURE, "系统默认超级管理员角色不允许删除")
	}

	// 检查是否有子角色（使用 children_num 字段判断，性能更好）
	if role.ChildrenNum > 0 {
		return e.NewBusinessError(1, "该角色有子角色，无法删除")
	}

	// 执行删除事务
	return s.executeDeleteTransaction(role, id)
}

// executeDeleteTransaction 执行删除事务
func (s *RoleService) executeDeleteTransaction(role *model.Role, id uint) error {
	db, err := role.GetDB()
	if err != nil {
		return e.NewBusinessError(1, "删除角色失败")
	}
	err = runInTransaction(db, func(tx *gorm.DB) error {
		role.SetDB(tx)

		// 删除角色菜单关联
		roleMenuMap := model.NewRoleMenuMap()
		roleMenuMap.SetDB(tx)
		if err := roleMenuMap.DeleteWhere("role_id = ?", id); err != nil {
			return err
		}

		// 保存父级ID，用于后续更新 children_num
		parentId := role.Pid

		// 删除角色
		if _, err := role.DeleteByID(id); err != nil {
			return err
		}

		// 更新父级的 children_num
		if parentId > 0 {
			if err := model.UpdateChildrenNum(model.NewRole(), parentId, tx); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return e.NewBusinessError(1, "删除角色失败")
	}

	return NewPermissionSyncCoordinator().SyncAll()
}

// Detail 获取角色详情
func (s *RoleService) Detail(id uint) (any, error) {
	role := model.NewRole()
	if err := role.GetAllById(id); err != nil || role.ID == 0 {
		return nil, e.NewBusinessError(1, "角色不存在")
	}
	return resources.NewRoleTransformer().ToStruct(role), nil
}

// GetRoleMenus 获取角色的所有菜单（包括直接绑定的菜单和继承自父角色的菜单）
func (s *RoleService) GetRoleMenus(roleId uint) ([]string, error) {
	role := model.NewRole()
	if err := role.GetById(roleId); err != nil || role.ID == 0 {
		return nil, e.NewBusinessError(1, "角色不存在")
	}

	menuIDs, err := NewUserPermissionSyncService().roleMenuIDs([]uint{roleId})
	if err != nil {
		return nil, e.NewBusinessError(1, "获取失败")
	}

	result := make([]string, 0, len(menuIDs))
	for _, menuID := range menuIDs {
		result = append(result, strconv.FormatUint(uint64(menuID), 10))
	}
	return result, nil
}
