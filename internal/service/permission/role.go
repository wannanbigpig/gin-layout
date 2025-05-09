package permission

import (
	"fmt"
	"strings"

	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	"github.com/wannanbigpig/gin-layout/pkg/utils"
)

// RoleService 角色服务
type RoleService struct {
	service.Base
}

func NewRoleService() *RoleService {
	return &RoleService{}
}

func (s *RoleService) List(params *form.RoleList) interface{} {
	var condition strings.Builder
	var args []any
	if params.Name != "" {
		condition.WriteString("name like ? AND ")
		args = append(args, "%"+params.Name+"%")
	}

	if params.Status != nil {
		condition.WriteString("status = ? AND ")
		args = append(args, params.Status)
	}

	condition.WriteString("pid = ? AND ")
	if params.Pid != nil {
		args = append(args, params.Pid)
	} else {
		args = append(args, 0)
	}

	conditionStr := condition.String()
	if conditionStr != "" {
		conditionStr = strings.TrimSuffix(condition.String(), "AND ")
	}

	RoleModel := model.NewRole()
	ListOptionalParams := model.ListOptionalParams{
		OrderBy: "sort desc, id desc",
	}

	total, collection := model.ListPage[model.Role](RoleModel, params.Page, params.PerPage, conditionStr, args, ListOptionalParams)

	return resources.ToRawCollection(params.Page, params.PerPage, total, collection)
}

func (s *RoleService) Edit(params *form.EditRole) error {
	role := model.NewRole()
	originPids := "0"
	// 检查编辑模式，加载要编辑的菜单
	if params.Id > 0 {
		if err := role.GetById(role, params.Id); err != nil || role.ID == 0 {
			return e.NewBusinessError(1, "编辑的角色不存在")
		}
		originPids = role.Pids
	}

	// 如果 pid 不为 0，检查是否改变了 pid 并更新相关信息
	if err := s.handlePidChange(role, params); err != nil {
		return err
	}

	// 赋值菜单信息
	role.Name = params.Name
	role.Description = params.Description
	role.Status = params.Status
	role.Pid = params.Pid
	role.Sort = params.Sort

	// 判断是否有菜单关联权限, 如果有则判断接口是否存在，只保留存在的接口ID
	if len(params.MenuList) > 0 {
		var validMenuIds []uint
		var apis []model.Api
		if err := model.NewMenu().DB().Where("id IN ?", params.MenuList).Find(&apis).Error; err != nil {
			return err
		}
		for _, api := range apis {
			validMenuIds = append(validMenuIds, api.ID)
		}
		params.MenuList = validMenuIds
	}

	// 更新菜单
	return role.DB().Transaction(func(tx *gorm.DB) error {
		// 保存菜单
		err := tx.Save(role).Error
		if err != nil {
			return err
		}

		// 合并更新子级pids和level为一个SQL操作
		if role.Pids != originPids {
			err = tx.Model(model.NewRole()).
				Where("FIND_IN_SET(?,pids)", role.ID).
				Updates(map[string]interface{}{
					"pids":  gorm.Expr("replace(pids,?,?)", originPids, role.Pids),
					"level": gorm.Expr("length(pids) - length(replace(pids, ',', '')) + 1"),
				}).Error
			if err != nil {
				return err
			}
		}

		// 更新角色菜单到关联中间表
		if err := s.updateRoleMenu(role, params.MenuList); err != nil {
			return err
		}

		return nil
	})
}

// updateRoleMenu 更新角色菜单关联
func (s *RoleService) updateRoleMenu(role *model.Role, menuList []uint) error {
	// 获取该角色现有的所有菜单关联（只查询 menu_id 字段）
	existingMaps := model.List(model.NewRoleMenuMap(), "role_id = ?", []any{role.ID}, model.ListOptionalParams{
		SelectFields: []string{"menu_id"}, // 优化：只查询需要的字段
	})

	// 提取现有菜单ID切片
	existingIds := lo.Map(existingMaps, func(m *model.RoleMenuMap, _ int) uint {
		return m.MenuId
	})

	// 2. 计算差集（一次性获取删除和新增列表）
	toDelete, toAdd := lo.Difference(existingIds, lo.Uniq(menuList))

	// 删除不再需要的关联
	if len(toDelete) > 0 {
		if err := model.NewRoleMenuMap().DeleteWithCondition(
			model.NewRoleMenuMap(),
			"role_id = ? AND menu_id IN (?)",
			[]any{role.ID, toDelete}...,
		); err != nil {
			return err
		}
	}

	// 批量创建新关联
	if len(toAdd) > 0 {
		newMappings := lo.Map(toAdd, func(menuId uint, _ int) *model.RoleMenuMap {
			return &model.RoleMenuMap{
				RoleId: role.ID,
				MenuId: menuId,
			}
		})
		if err := model.NewRoleMenuMap().BatchCreate(newMappings); err != nil {
			return err
		}
	}

	return nil
}

// handlePidChange 处理 PID 变化，如果 PID 发生改变，更新菜单的层级和 Pids
func (s *RoleService) handlePidChange(role *model.Role, params *form.EditRole) error {
	if params.Pid > 0 && params.Pid != role.Pid {
		var parentRole model.Role
		if err := parentRole.GetById(&parentRole, params.Pid); err != nil || parentRole.ID == 0 {
			return e.NewBusinessError(1, "上级菜单不存在")
		}

		// 防止循环引用
		if utils.WouldCauseCycle(parentRole.Pids, role.ID) {
			return e.NewBusinessError(1, "不能将菜单父节点设置为其自身的子级")
		}

		role.Level = parentRole.Level + 1
		role.Pids = strings.TrimPrefix(fmt.Sprintf("%s,%d", parentRole.Pids, parentRole.ID), ",")
	} else if params.Pid == 0 || role.Pid == 0 {
		role.Level = 1
		role.Pids = "0"
	}
	role.Pid = params.Pid
	return nil
}
