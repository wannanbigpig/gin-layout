package menu

import (
	"fmt"
	"strings"

	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	utils2 "github.com/wannanbigpig/gin-layout/pkg/utils"
)

// Create 新增菜单。
func (s *MenuService) Create(params *form.CreateMenu) error {
	return s.applyMenuMutation(&menuMutation{
		Icon:            params.Icon,
		Title:           params.Title,
		Code:            params.Code,
		Path:            params.Path,
		Name:            params.Name,
		AnimateEnter:    params.AnimateEnter,
		AnimateLeave:    params.AnimateLeave,
		AnimateDuration: params.AnimateDuration,
		IsShow:          params.IsShow,
		IsAuth:          params.IsAuth,
		IsNewWindow:     params.IsNewWindow,
		Sort:            params.Sort,
		Type:            params.Type,
		Pid:             params.Pid,
		Description:     params.Description,
		ApiList:         params.ApiList,
		Component:       params.Component,
		Status:          params.Status,
		Redirect:        params.Redirect,
		IsExternalLinks: params.IsExternalLinks,
	})
}

// Update 更新菜单。
func (s *MenuService) Update(params *form.UpdateMenu) error {
	return s.applyMenuMutation(&menuMutation{
		Id:              params.Id,
		Icon:            params.Icon,
		Title:           params.Title,
		Code:            params.Code,
		Path:            params.Path,
		Name:            params.Name,
		AnimateEnter:    params.AnimateEnter,
		AnimateLeave:    params.AnimateLeave,
		AnimateDuration: params.AnimateDuration,
		IsShow:          params.IsShow,
		IsAuth:          params.IsAuth,
		IsNewWindow:     params.IsNewWindow,
		Sort:            params.Sort,
		Type:            params.Type,
		Pid:             params.Pid,
		Description:     params.Description,
		ApiList:         params.ApiList,
		Component:       params.Component,
		Status:          params.Status,
		Redirect:        params.Redirect,
		IsExternalLinks: params.IsExternalLinks,
	})
}

type menuMutation struct {
	Id              uint
	Icon            string
	Title           string
	Code            string
	Path            string
	Name            string
	AnimateEnter    string
	AnimateLeave    string
	AnimateDuration float32
	IsShow          uint8
	IsAuth          uint8
	IsNewWindow     uint8
	Sort            uint
	Type            uint8
	Pid             uint
	Description     string
	ApiList         []uint
	Component       string
	Status          uint8
	Redirect        string
	IsExternalLinks uint8
}

type menuEditContext struct {
	originPids     string
	originPid      uint
	originFullPath string
	excludeId      uint
}

func (s *MenuService) applyMenuMutation(params *menuMutation) error {
	menu := model.NewMenu()
	editContext := &menuEditContext{
		originPids:     menuRootPid,
		originPid:      0,
		originFullPath: "",
		excludeId:      0,
	}
	if params.Id > 0 {
		if err := menu.GetById(params.Id); err != nil || menu.ID == 0 {
			return e.NewBusinessError(1, "编辑的菜单不存在")
		}
		editContext.originPids = menu.Pids
		editContext.originPid = menu.Pid
		editContext.originFullPath = menu.FullPath
		editContext.excludeId = params.Id
	}
	if err := s.resolveMenuHierarchy(menu, params); err != nil {
		return err
	}
	if menu.Level > maxMenuLevel {
		return e.NewBusinessError(1, "最多只能创建4层菜单")
	}

	s.assignMenuFields(menu, params)
	if err := s.validateUniqueFields(menu, params, editContext.excludeId); err != nil {
		return err
	}
	if len(params.ApiList) > 0 {
		apis, err := model.NewApi().FindByIds(params.ApiList)
		if err != nil {
			return err
		}
		params.ApiList = lo.Map(apis, func(api model.Api, _ int) uint {
			return api.ID
		})
	}
	return s.executeEditTransaction(menu, params.ApiList, editContext)
}

// resolveMenuHierarchy 统一处理菜单层级、父级合法性和 full_path 计算，避免主流程在多个小函数间跳转。
func (s *MenuService) resolveMenuHierarchy(menu *model.Menu, params *menuMutation) error {
	needRefreshByParent := (params.Pid > 0 && params.Pid != menu.Pid) ||
		(params.Pid > 0 && params.Path != menu.Path)
	if !needRefreshByParent {
		if params.Pid == 0 {
			menu.Level = menuRootLevel
			menu.Pids = menuRootPid
			menu.FullPath = s.buildFullPath(params.Path, rootPath, params.Type)
		}
		menu.Pid = params.Pid
		return nil
	}

	parentMenu := model.NewMenu()
	if err := parentMenu.GetById(params.Pid); err != nil || parentMenu.ID == 0 {
		return e.NewBusinessError(1, "上级菜单不存在")
	}
	if parentMenu.Type == model.BUTTON {
		return e.NewBusinessError(1, "上级菜单不能是按钮类型")
	}
	if utils2.WouldCauseCycle(menu.ID, params.Pid, parentMenu.Pids) {
		return e.NewBusinessError(1, "上级菜单不能是当前菜单自身或其子菜单")
	}

	menu.Level = parentMenu.Level + 1
	menu.Pids = s.buildPids(parentMenu.Pids, parentMenu.ID)
	menu.FullPath = s.buildFullPath(params.Path, parentMenu.FullPath, params.Type)
	menu.Pid = params.Pid
	return nil
}

func (s *MenuService) buildPids(parentPids string, parentID uint) string {
	return strings.TrimPrefix(fmt.Sprintf("%s,%d", parentPids, parentID), ",")
}

func (s *MenuService) buildFullPath(path, parentPath string, menuType uint8) string {
	if menuType == model.BUTTON {
		return ""
	}
	if parentPath == "" {
		parentPath = rootPath
	}
	if strings.HasPrefix(path, rootPath) ||
		strings.HasPrefix(path, "https://") ||
		strings.HasPrefix(path, "http://") {
		return path
	}
	if !strings.HasSuffix(parentPath, "/") {
		parentPath += "/"
	}
	return parentPath + path
}

func (s *MenuService) assignMenuFields(menu *model.Menu, params *menuMutation) {
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
	menu.Description = params.Description
	menu.IsExternalLinks = params.IsExternalLinks
	if params.Type == model.BUTTON {
		menu.FullPath = ""
	}
}

func (s *MenuService) validateUniqueFields(menu *model.Menu, params *menuMutation, excludeId uint) error {
	codeExists, err := menu.ExistsExcludeId("code", params.Code, excludeId)
	if err != nil {
		return err
	}
	if params.Code != "" && codeExists {
		return e.NewBusinessError(1, "权限标识已存在")
	}

	nameExists, err := menu.ExistsExcludeId("name", params.Name, excludeId)
	if err != nil {
		return err
	}
	if params.Name != "" && nameExists {
		return e.NewBusinessError(1, "路由名称已存在")
	}

	if params.Type != model.BUTTON && menu.Path != "" {
		pathExists, err := menu.ExistsExcludeId("full_path", menu.FullPath, excludeId)
		if err != nil {
			return err
		}
		if pathExists {
			return e.NewBusinessError(1, "路由已存在")
		}
	}
	return nil
}

func (s *MenuService) executeEditTransaction(menu *model.Menu, apiList []uint, editContext *menuEditContext) error {
	db, err := menu.GetDB()
	if err != nil {
		return err
	}

	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		menu.SetDB(tx)
		if err := menu.Save(); err != nil {
			return err
		}
		if err := s.updateChildrenLevels(menu, editContext.originPids, editContext.originFullPath, tx); err != nil {
			return err
		}
		if editContext.originPid > 0 && editContext.originPid != menu.Pid {
			if err := model.UpdateChildrenNum(model.NewMenu(), editContext.originPid, tx); err != nil {
				return err
			}
		}
		if menu.Pid > 0 && menu.Pid != editContext.originPid {
			if err := model.UpdateChildrenNum(model.NewMenu(), menu.Pid, tx); err != nil {
				return err
			}
		}
		return s.updateMenuPermissions(menu, apiList, tx)
	})
	if err != nil {
		return err
	}
	return access.NewPermissionSyncCoordinator().SyncUsersAffectedByMenus([]uint{menu.ID})
}

func (s *MenuService) updateChildrenLevels(menu *model.Menu, originPids string, originFullPath string, tx *gorm.DB) error {
	if menu.Pids == originPids && menu.FullPath == originFullPath {
		return nil
	}

	descendantModel := model.NewMenu()
	descendantModel.SetDB(tx)
	descendants, err := descendantModel.FindDescendantsById(menu.ID)
	if err != nil {
		return err
	}
	if len(descendants) == 0 {
		return nil
	}

	childrenByPID := make(map[uint][]*model.Menu, len(descendants))
	for _, child := range descendants {
		childrenByPID[child.Pid] = append(childrenByPID[child.Pid], &child)
	}
	return s.rebuildMenuDescendants(tx, menu, childrenByPID)
}

func (s *MenuService) rebuildMenuDescendants(tx *gorm.DB, parent *model.Menu, childrenByPID map[uint][]*model.Menu) error {
	menuModel := model.NewMenu()
	menuModel.SetDB(tx)
	for _, child := range childrenByPID[parent.ID] {
		child.Pids = s.buildPids(parent.Pids, parent.ID)
		child.Level = parent.Level + 1
		child.FullPath = s.buildFullPath(child.Path, parent.FullPath, child.Type)
		if child.Type == model.BUTTON {
			child.FullPath = ""
		}
		if err := menuModel.UpdateById(child.ID, map[string]any{
			"pids":      child.Pids,
			"level":     child.Level,
			"full_path": child.FullPath,
		}); err != nil {
			return err
		}
		if err := s.rebuildMenuDescendants(tx, child, childrenByPID); err != nil {
			return err
		}
	}
	return nil
}

func (s *MenuService) updateMenuPermissions(menu *model.Menu, apiList []uint, tx ...*gorm.DB) error {
	menuApiMap := model.NewMenuApiMap()
	if len(tx) > 0 {
		menuApiMap.SetDB(tx[0])
	}

	existingMaps, err := model.ListE(menuApiMap, "menu_id = ?", []any{menu.ID}, model.ListOptionalParams{
		SelectFields: []string{"api_id"},
	})
	if err != nil {
		return err
	}

	existingIDs := lo.Map(existingMaps, func(m *model.MenuApiMap, _ int) uint {
		return m.ApiId
	})
	apiList = lo.Uniq(apiList)
	toDelete, toAdd := lo.Difference(existingIDs, apiList)

	if len(toDelete) > 0 {
		if err := menuApiMap.DeleteWhere("menu_id = ? AND api_id IN (?)", []any{menu.ID, toDelete}...); err != nil {
			return err
		}
	}
	if len(toAdd) == 0 {
		return nil
	}

	newMappings := lo.Map(toAdd, func(apiID uint, _ int) *model.MenuApiMap {
		return &model.MenuApiMap{MenuId: menu.ID, ApiId: apiID}
	})
	return menuApiMap.CreateBatch(newMappings)
}

// UpdateAllMenuPermissions 批量更新所有菜单的权限到 Casbin
func (s *MenuService) UpdateAllMenuPermissions() error {
	return access.NewPermissionSyncCoordinator().SyncAll()
}
