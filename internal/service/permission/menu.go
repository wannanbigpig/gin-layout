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

// MenuService 菜单服务
type MenuService struct {
	service.Base
}

func NewMenuService() *MenuService {
	return &MenuService{}
}

// Edit 菜单编辑
func (s *MenuService) Edit(params *form.EditMenu) error {
	menu := model.NewMenu()
	where := ""
	originPids := "0"
	// 检查编辑模式，加载要编辑的菜单
	if params.Id > 0 {
		if err := menu.GetById(menu, params.Id); err != nil || menu.ID == 0 {
			return e.NewBusinessError(1, "编辑的菜单不存在")
		}
		originPids = menu.Pids
		where = fmt.Sprintf(" AND id != %d", params.Id)
	}

	// 如果 pid 不为 0，检查是否改变了 pid 并更新相关信息
	if err := s.handlePidChange(menu, params); err != nil {
		return err
	}

	// 赋值菜单信息
	s.assignMenuFields(menu, params)

	// 校验 Code、FullPath、name 的唯一性
	if err := s.validateUniqueFields(menu, params, where); err != nil {
		return err
	}

	// 判断是否有菜单关联权限, 如果有则判断接口是否存在，只保留存在的接口ID
	if len(params.ApiList) > 0 {
		var validApiIds []uint
		var apis []model.Api
		if err := model.NewApi().DB().Where("id IN ?", params.ApiList).Find(&apis).Error; err != nil {
			return err
		}
		for _, api := range apis {
			validApiIds = append(validApiIds, api.ID)
		}
		params.ApiList = validApiIds
	}

	// 更新菜单
	return menu.DB().Transaction(func(tx *gorm.DB) error {
		// 保存菜单
		err := tx.Save(menu).Error
		if err != nil {
			return err
		}

		// 合并更新子级pids和level为一个SQL操作
		if menu.Pids != originPids {
			err = tx.Model(model.NewMenu()).
				Where("FIND_IN_SET(?,pids)", menu.ID).
				Updates(map[string]interface{}{
					"pids":  gorm.Expr("replace(pids,?,?)", originPids, menu.Pids),
					"level": gorm.Expr("length(pids) - length(replace(pids, ',', '')) + 1"),
				}).Error
			if err != nil {
				return err
			}
		}

		// 更新菜单权限到关联中间表
		if err := s.updateMenuPermissions(menu, params.ApiList); err != nil {
			return err
		}

		return nil
	})
}

func (s *MenuService) assembleFullPath(path, parentPath string) string {
	if parentPath == "" {
		parentPath = "/"
	}
	// 判断path不是/, https:// ,http:// 开头 则需要拼接上/
	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "https://") || strings.HasPrefix(path, "http://") {
		return path
	}
	// 判断parentPath不是/ 结尾则需要拼接上/
	if !strings.HasSuffix(parentPath, "/") {
		parentPath += "/"
	}
	return parentPath + path
}

// updateMenuPermissions 更新菜单权限到关联中间表
func (s *MenuService) updateMenuPermissions(menu *model.Menu, apiList []uint) error {
	// 获取该角色现有的所有菜单关联（只查询 menu_id 字段）
	existingMaps := model.List(model.NewMenuApiMap(), "menu_id = ?", []any{menu.ID}, model.ListOptionalParams{
		SelectFields: []string{"menu_id"}, // 优化：只查询需要的字段
	})

	// 提取现有菜单ID切片
	existingIds := lo.Map(existingMaps, func(m *model.MenuApiMap, _ int) uint {
		return m.MenuId
	})

	// 2. 计算差集（一次性获取删除和新增列表）
	toDelete, toAdd := lo.Difference(existingIds, lo.Uniq(apiList))

	// 删除不再需要的关联
	if len(toDelete) > 0 {
		if err := model.NewMenuApiMap().DeleteWithCondition(
			model.NewMenuApiMap(),
			"role_id = ? AND menu_id IN (?)",
			[]any{menu.ID, toDelete}...,
		); err != nil {
			return err
		}
	}

	// 批量创建新关联
	if len(toAdd) > 0 {
		newMappings := lo.Map(toAdd, func(apiId uint, _ int) *model.MenuApiMap {
			return &model.MenuApiMap{
				MenuId: menu.ID,
				ApiId:  apiId,
			}
		})
		if err := model.NewMenuApiMap().BatchCreate(newMappings); err != nil {
			return err
		}
	}

	return nil
}

// assignMenuFields 将菜单字段从 params 赋值给 menu
func (s *MenuService) assignMenuFields(menu *model.Menu, params *form.EditMenu) {
	menu.Icon = params.Icon
	menu.Pid = params.Pid
	menu.Title = params.Title
	menu.Code = params.Code
	menu.Path = params.Path
	menu.Name = params.Name
	menu.Component = params.Component
	menu.Status = params.Status
	menu.Redirect = params.Redirect
	menu.AnimateEnter = params.AnimateEnter
	menu.AnimateLeave = params.AnimateLeave
	menu.AnimateDuration = params.AnimateDuration
	menu.IsShow = params.IsShow
	menu.IsAuth = params.IsAuth
	menu.IsNewWindow = params.IsNewWindow
	menu.Sort = params.Sort
	menu.Type = params.Type
	menu.Desc = params.Desc
	menu.IsExternalLinks = params.IsExternalLinks
}

// 处理 PID 变化，如果 PID 发生改变，更新菜单的层级和 Pids,full_path，如果只有path变化则更新full_path
func (s *MenuService) handlePidChange(menu *model.Menu, params *form.EditMenu) error {
	if (params.Pid > 0 && params.Pid != menu.Pid) || (params.Path != menu.Path && params.Pid > 0) {
		var parentMenu model.Menu
		if err := parentMenu.GetById(&parentMenu, params.Pid); err != nil || parentMenu.ID == 0 {
			return e.NewBusinessError(1, "上级菜单不存在")
		}

		// 防止循环引用
		if utils.WouldCauseCycle(parentMenu.Pids, menu.ID) {
			return e.NewBusinessError(1, "不能将菜单父节点设置为其自身的子级")
		}

		menu.Level = parentMenu.Level + 1
		menu.FullPath = s.assembleFullPath(params.Path, parentMenu.FullPath)
		menu.Pids = strings.TrimPrefix(fmt.Sprintf("%s,%d", parentMenu.Pids, parentMenu.ID), ",")
	} else if params.Pid == 0 || menu.Pid == 0 {
		menu.Level = 1
		menu.Pids = "0"
		menu.FullPath = s.assembleFullPath(params.Path, "/")
	}
	menu.Pid = params.Pid
	return nil
}

// 验证菜单 Code、Name 和 FullPath 的唯一性
func (s *MenuService) validateUniqueFields(menu *model.Menu, params *form.EditMenu, where string) error {
	// 验证权限标识唯一性
	if params.Code != "" && menu.Exists(menu, "code = ?"+where, params.Code) {
		return e.NewBusinessError(1, "权限标识已存在")
	}

	if params.Name != "" && menu.Exists(menu, "name = ?"+where, params.Name) {
		return e.NewBusinessError(1, "路由名称已存在")
	}

	// 验证路由唯一性
	if menu.Path != "" && menu.Exists(menu, "full_path = ?"+where, menu.FullPath) {
		return e.NewBusinessError(1, "路由已存在")
	}

	return nil
}

// ListPage 菜单列表
func (s *MenuService) ListPage(params *form.ListMenu) *resources.Collection {
	var condition strings.Builder
	var args []any
	if params.Keyword != "" {
		condition.WriteString("(title like ? OR path like ? OR code = ?) AND ")
		args = append(args, "%"+params.Keyword+"%")
		args = append(args, "%"+params.Keyword+"%")
		args = append(args, params.Keyword)
	}
	if params.IsAuth != nil {
		condition.WriteString("is_auth = ? AND ")
		args = append(args, params.IsAuth)
	}
	conditionStr := condition.String()
	if conditionStr != "" {
		conditionStr = strings.TrimSuffix(condition.String(), "AND ")
	}

	menu := model.NewMenu()
	total, collection := model.ListPage[model.Menu](menu, params.Page, params.PerPage, conditionStr, args)
	return resources.NewMenuTransformer().ToCollection(params.Page, params.PerPage, total, collection)
}

func (s *MenuService) Delete(id uint) error {
	menu := model.NewMenu()
	err := menu.GetById(menu, id)
	if err != nil || menu.ID == 0 {
		return e.NewBusinessError(1, "菜单不存在")
	}
	// 判断是否有子菜单
	ok, err := model.HasChildren[model.Menu](menu, menu.ID)
	if err != nil {
		return e.NewBusinessError(1, "查询是否有子菜单失败")
	}

	if ok {
		return e.NewBusinessError(1, "该菜单有子菜单，无法删除")
	}

	result := menu.DB().Select("ApiList").Delete(menu)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return e.NewBusinessError(1, "删除菜单失败")
	}

	return nil
}

func (s *MenuService) List(params *form.ListMenu) any {
	var condition strings.Builder
	var args []any
	if params.Keyword != "" {
		condition.WriteString("(title like ? OR path like ? OR code = ?) AND ")
		args = append(args, "%"+params.Keyword+"%")
		args = append(args, "%"+params.Keyword+"%")
		args = append(args, params.Keyword)
	}
	if params.IsAuth != nil {
		condition.WriteString("is_auth = ? AND ")
		args = append(args, params.IsAuth)
	}
	var allStatus int8 = 2
	if params.Status != nil && *params.Status != allStatus {
		condition.WriteString("status = ? AND ")
		args = append(args, params.Status)
	}

	conditionStr := condition.String()
	if conditionStr != "" {
		conditionStr = strings.TrimSuffix(condition.String(), "AND ")
	}
	menuModel := model.NewMenu()
	menus := model.List[model.Menu](menuModel, conditionStr, args, model.ListOptionalParams{
		OrderBy: "sort desc, id desc",
	})

	return resources.NewMenuTreeTransformer().BuildTreeByNode(menus, 0)
}

func (s *MenuService) Detail(id uint) (any, error) {
	menu := model.NewMenu()
	if err := menu.GetAllById(menu, id); err != nil || menu.ID == 0 {
		return nil, e.NewBusinessError(1, "菜单不存在")
	}
	return resources.NewMenuTransformer().ToStruct(menu), nil
}
