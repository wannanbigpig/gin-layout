package menu

import (
	"fmt"
	"strings"

	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/i18n"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	utils2 "github.com/wannanbigpig/gin-layout/pkg/utils"
)

// Create 新增菜单。
func (s *MenuService) Create(params *form.CreateMenu, _ string) error {
	_, err := s.applyMenuMutation(&menuMutation{
		Icon:            params.Icon,
		TitleI18n:       params.TitleI18n,
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
	return err
}

// Update 更新菜单。
func (s *MenuService) Update(params *form.UpdateMenu, _ string) error {
	_, err := s.applyMenuMutation(&menuMutation{
		Id:              params.Id,
		Icon:            params.Icon,
		TitleI18n:       params.TitleI18n,
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
	return err
}

// menuMutation 菜单变更参数，用于封装新增/更新菜单的请求数据。
type menuMutation struct {
	Id              uint   // 菜单 ID，0 表示新增
	Icon            string // 图标
	TitleI18n       map[string]string
	Code            string  // 权限标识
	Path            string  // 路径
	Name            string  // 路由名称
	AnimateEnter    string  // 进入动画
	AnimateLeave    string  // 离开动画
	AnimateDuration float32 // 动画时长
	IsShow          uint8   // 是否显示
	IsAuth          uint8   // 是否鉴权
	IsNewWindow     uint8   // 是否新窗口打开
	Sort            uint    // 排序权重
	Type            uint8   // 菜单类型（目录/菜单/按钮）
	Pid             uint    // 父菜单 ID
	Description     string  // 描述
	ApiList         []uint  // 关联的 API ID 列表
	Component       string  // 组件路径
	Status          uint8   // 状态
	Redirect        string  // 重定向路径
	IsExternalLinks uint8   // 是否外链
}

// menuEditContext 菜单编辑上下文，保存更新前的状态用于级联判断。
type menuEditContext struct {
	originPids     string // 原始路径
	originPid      uint   // 原始父 ID
	originFullPath string // 原始完整路径
	excludeId      uint   // 排除的当前菜单 ID
}

// applyMenuMutation 执行菜单变更操作（新增/更新）。
// 处理逻辑：
// 1. 验证菜单是否存在（更新时）
// 2. 构建树形层级（pids, level, full_path）
// 3. 检查层级深度
// 4. 填充菜单字段
// 5. 验证唯一字段（code, name, path）
// 6. 验证 API 列表
// 7. 事务保存：菜单数据、级联更新子菜单、更新子菜单数量、同步菜单权限
func (s *MenuService) applyMenuMutation(params *menuMutation) (*model.Menu, error) {
	menu, editContext, err := s.prepareMutationContext(params)
	if err != nil {
		return nil, err
	}

	// 1) 规范化标题输入
	if err := s.normalizeMenuTitles(params); err != nil {
		return nil, err
	}
	// 2) 构建树形层级并验证合法性
	if err := s.resolveMenuHierarchy(menu, params); err != nil {
		return nil, err
	}
	// 3) 检查层级深度
	if menu.Level > maxMenuLevel {
		return nil, e.NewBusinessError(e.MaxMenuDepth)
	}

	// 4) 填充字段并验证唯一性
	s.assignMenuFields(menu, params)
	if err := s.validateUniqueFields(menu, params, editContext.excludeId); err != nil {
		return nil, err
	}

	// 5) 规范化关联 API 列表（仅保留有效且去重后的 ID）
	if err := s.normalizeMenuAPIList(params); err != nil {
		return nil, err
	}

	// 6) 持久化及事务后处理
	if err := s.executeEditTransaction(menu, params.ApiList, params.TitleI18n, editContext); err != nil {
		return nil, err
	}
	return menu, nil
}

// prepareMutationContext 根据新增/更新场景初始化菜单模型与编辑上下文。
func (s *MenuService) prepareMutationContext(params *menuMutation) (*model.Menu, *menuEditContext, error) {
	menu := model.NewMenu()
	editContext := newMenuEditContext()
	if params.Id == 0 {
		return menu, editContext, nil
	}

	// 更新场景：加载原菜单，供唯一性校验与子树级联更新使用。
	if err := menu.GetById(params.Id); err != nil || menu.ID == 0 {
		return nil, nil, e.NewBusinessError(e.MenuNotFound)
	}
	editContext.originPids = menu.Pids
	editContext.originPid = menu.Pid
	editContext.originFullPath = menu.FullPath
	editContext.excludeId = params.Id
	return menu, editContext, nil
}

func newMenuEditContext() *menuEditContext {
	return &menuEditContext{
		originPids:     menuRootPid,
		originPid:      0,
		originFullPath: "",
		excludeId:      0,
	}
}

// normalizeMenuAPIList 校验 API ID 是否存在，并仅保留有效去重结果。
func (s *MenuService) normalizeMenuAPIList(params *menuMutation) error {
	if len(params.ApiList) == 0 {
		return nil
	}

	apis, err := model.NewApi().FindByIds(params.ApiList)
	if err != nil {
		return err
	}
	params.ApiList = lo.Map(apis, func(api model.Api, _ int) uint {
		return api.ID
	})
	return nil
}

// normalizeMenuTitles 规范化菜单标题输入，要求中英至少一种语言非空。
func (s *MenuService) normalizeMenuTitles(params *menuMutation) error {
	normalized := make(map[string]string, len(params.TitleI18n))
	for locale, title := range params.TitleI18n {
		normalizedLocale := i18n.NormalizeLocale(locale)
		if !isSupportedMenuLocale(normalizedLocale) {
			return e.NewBusinessError(e.InvalidParameter)
		}
		trimmedTitle := strings.TrimSpace(title)
		if trimmedTitle == "" {
			continue
		}
		normalized[normalizedLocale] = trimmedTitle
	}

	zhTitle := strings.TrimSpace(normalized[i18n.LocaleZhCN])
	enTitle := strings.TrimSpace(normalized[i18n.LocaleEnUS])
	if zhTitle == "" && enTitle == "" {
		return e.NewBusinessError(e.InvalidParameter)
	}
	params.TitleI18n = normalized
	return nil
}

func isSupportedMenuLocale(locale string) bool {
	return locale == i18n.LocaleZhCN || locale == i18n.LocaleEnUS
}

// resolveMenuHierarchy 统一处理菜单层级、父级合法性和 full_path 计算，避免主流程在多个小函数间跳转。
// 处理逻辑：
// 1. 判断是否需要更新父级信息（pid 变更或 path 变更）
// 2. 验证父菜单是否存在且不是按钮类型
// 3. 检测环路
// 4. 计算新的 level、pids、full_path
func (s *MenuService) resolveMenuHierarchy(menu *model.Menu, params *menuMutation) error {
	needRefreshByParent := (params.Pid > 0 && params.Pid != menu.Pid) ||
		(params.Pid > 0 && params.Path != menu.Path)
	if !needRefreshByParent {
		// 无需更新父级信息，仅处理顶级菜单场景
		if params.Pid == 0 {
			menu.Level = menuRootLevel
			menu.Pids = menuRootPid
			menu.FullPath = s.buildFullPath(params.Path, rootPath, params.Type)
		}
		menu.Pid = params.Pid
		return nil
	}

	// 验证父菜单是否存在
	parentMenu := model.NewMenu()
	if err := parentMenu.GetById(params.Pid); err != nil || parentMenu.ID == 0 {
		return e.NewBusinessError(e.ParentMenuNotExists)
	}
	// 父菜单不能是按钮类型
	if parentMenu.Type == model.BUTTON {
		return e.NewBusinessError(e.ParentMenuTypeInvalid)
	}
	// 环路检测
	if utils2.WouldCauseCycle(menu.ID, params.Pid, parentMenu.Pids) {
		return e.NewBusinessError(e.ParentMenuInvalid)
	}

	// 计算新的层级和路径
	menu.Level = parentMenu.Level + 1
	menu.Pids = s.buildPids(parentMenu.Pids, parentMenu.ID)
	menu.FullPath = s.buildFullPath(params.Path, parentMenu.FullPath, params.Type)
	menu.Pid = params.Pid
	return nil
}

// buildPids 构建子节点的 pids 路径：父 pids + 父 ID。
func (s *MenuService) buildPids(parentPids string, parentID uint) string {
	return strings.TrimPrefix(fmt.Sprintf("%s,%d", parentPids, parentID), ",")
}

// buildFullPath 构建菜单的完整路径。
// 规则：
// 1. 按钮类型无 full_path
// 2. 已有完整路径前缀（/、http、https）则直接使用
// 3. 否则拼接父路径 + 当前路径
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

// assignMenuFields 填充菜单模型字段。
func (s *MenuService) assignMenuFields(menu *model.Menu, params *menuMutation) {
	menu.Icon = params.Icon
	menu.Pid = params.Pid
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
	// 按钮类型无 full_path
	if params.Type == model.BUTTON {
		menu.FullPath = ""
	}
}

// validateUniqueFields 验证菜单唯一字段：code、name、full_path。
func (s *MenuService) validateUniqueFields(menu *model.Menu, params *menuMutation, excludeId uint) error {
	// 验证 code 唯一性
	codeExists, err := menu.ExistsExcludeId("code", params.Code, excludeId)
	if err != nil {
		return err
	}
	if params.Code != "" && codeExists {
		return e.NewBusinessError(e.MenuCodeExists)
	}

	// 验证 name 唯一性
	nameExists, err := menu.ExistsExcludeId("name", params.Name, excludeId)
	if err != nil {
		return err
	}
	if params.Name != "" && nameExists {
		return e.NewBusinessError(e.MenuRouteNameExists)
	}

	// 验证 full_path 唯一性（按钮类型除外）
	if params.Type != model.BUTTON && menu.Path != "" {
		pathExists, err := menu.ExistsExcludeId("full_path", menu.FullPath, excludeId)
		if err != nil {
			return err
		}
		if pathExists {
			return e.NewBusinessError(e.MenuPathExists)
		}
	}
	return nil
}

// executeEditTransaction 执行菜单编辑事务。
// 处理逻辑：
// 1. 保存菜单数据
// 2. 级联更新子菜单的 level、pids、full_path
// 3. 更新原父菜单和新父菜单的子菜单数量
// 4. 更新菜单关联的 API 权限
// 5. 同步受影响用户的权限缓存
func (s *MenuService) executeEditTransaction(menu *model.Menu, apiList []uint, titleI18n map[string]string, editContext *menuEditContext) error {
	db, err := menu.GetDB()
	if err != nil {
		return err
	}

	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		// 保存菜单数据
		if err := s.persistMenu(menu, tx); err != nil {
			return err
		}
		if err := model.NewMenuI18n().UpsertMenuTitles(menu.ID, titleI18n, tx); err != nil {
			return err
		}
		// 级联更新子菜单
		if err := s.updateChildrenLevels(menu, editContext.originPids, editContext.originFullPath, tx); err != nil {
			return err
		}
		if err := s.updateParentChildrenNum(menu, editContext, tx); err != nil {
			return err
		}
		// 更新菜单关联的 API 权限
		return s.updateMenuPermissions(menu, apiList, tx)
	})
	if err != nil {
		return err
	}
	// 同步受影响用户的权限缓存
	return access.NewPermissionSyncCoordinator().SyncUsersAffectedByMenus([]uint{menu.ID})
}

// persistMenu 持久化菜单数据。
func (s *MenuService) persistMenu(menu *model.Menu, tx *gorm.DB) error {
	menu.SetDB(tx)
	return menu.Save()
}

// updateParentChildrenNum 在父节点变更后刷新原父与新父的 children_num。
func (s *MenuService) updateParentChildrenNum(menu *model.Menu, editContext *menuEditContext, tx *gorm.DB) error {
	// 原父节点的子节点数减一
	if editContext.originPid > 0 && editContext.originPid != menu.Pid {
		if err := model.UpdateChildrenNum(model.NewMenu(), editContext.originPid, tx); err != nil {
			return err
		}
	}
	// 新父节点的子节点数加一
	if menu.Pid > 0 && menu.Pid != editContext.originPid {
		if err := model.UpdateChildrenNum(model.NewMenu(), menu.Pid, tx); err != nil {
			return err
		}
	}
	return nil
}

// updateChildrenLevels 更新子菜单的层级信息（pids, level, full_path）。
// 当菜单的 pids 或 full_path 变更时，需要级联更新所有子代菜单。
func (s *MenuService) updateChildrenLevels(menu *model.Menu, originPids string, originFullPath string, tx *gorm.DB) error {
	// pids 和 full_path 都未变更，无需级联更新
	if menu.Pids == originPids && menu.FullPath == originFullPath {
		return nil
	}

	// 查询所有子代菜单
	descendantModel := model.NewMenu()
	descendantModel.SetDB(tx)
	descendants, err := descendantModel.FindDescendantsById(menu.ID)
	if err != nil {
		return err
	}
	if len(descendants) == 0 {
		return nil
	}

	// 按 pid 分组，便于递归重建
	childrenByPID := s.groupDescendantsByPID(descendants)
	return s.rebuildMenuDescendants(tx, menu, childrenByPID)
}

// groupDescendantsByPID 按父节点分组子菜单。
// 这里必须使用索引取址，避免 range 临时变量取址导致所有指针指向同一对象。
func (s *MenuService) groupDescendantsByPID(descendants []model.Menu) map[uint][]*model.Menu {
	childrenByPID := make(map[uint][]*model.Menu, len(descendants))
	for i := range descendants {
		child := &descendants[i]
		childrenByPID[child.Pid] = append(childrenByPID[child.Pid], child)
	}
	return childrenByPID
}

// rebuildMenuDescendants 递归重建子代菜单的层级信息。
func (s *MenuService) rebuildMenuDescendants(tx *gorm.DB, parent *model.Menu, childrenByPID map[uint][]*model.Menu) error {
	menuModel := model.NewMenu()
	menuModel.SetDB(tx)
	for _, child := range childrenByPID[parent.ID] {
		// 重建子菜单的 pids、level、full_path
		child.Pids = s.buildPids(parent.Pids, parent.ID)
		child.Level = parent.Level + 1
		child.FullPath = s.buildFullPath(child.Path, parent.FullPath, child.Type)
		if child.Type == model.BUTTON {
			child.FullPath = ""
		}
		// 批量更新数据库
		if err := menuModel.UpdateById(child.ID, map[string]any{
			"pids":      child.Pids,
			"level":     child.Level,
			"full_path": child.FullPath,
		}); err != nil {
			return err
		}
		// 递归处理下一级子菜单
		if err := s.rebuildMenuDescendants(tx, child, childrenByPID); err != nil {
			return err
		}
	}
	return nil
}

// updateMenuPermissions 更新菜单关联的 API 权限。
// 使用差分算法：计算需要删除和新增的 API ID，只变更差异部分。
func (s *MenuService) updateMenuPermissions(menu *model.Menu, apiList []uint, tx ...*gorm.DB) error {
	menuApiMap := model.NewMenuApiMap()
	if len(tx) > 0 {
		menuApiMap.SetDB(tx[0])
	}

	// 查询菜单当前已关联的 API ID 列表
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
	// 计算差异
	toDelete, toAdd := lo.Difference(existingIDs, apiList)

	// 删除差异 API 关联
	if len(toDelete) > 0 {
		if err := menuApiMap.DeleteWhere("menu_id = ? AND api_id IN (?)", []any{menu.ID, toDelete}...); err != nil {
			return err
		}
	}
	if len(toAdd) == 0 {
		return nil
	}

	// 新增 API 关联
	newMappings := lo.Map(toAdd, func(apiID uint, _ int) *model.MenuApiMap {
		return &model.MenuApiMap{MenuId: menu.ID, ApiId: apiID}
	})
	return menuApiMap.CreateBatch(newMappings)
}

// UpdateAllMenuPermissions 批量更新所有菜单的权限到 Casbin。
func (s *MenuService) UpdateAllMenuPermissions() error {
	return access.NewPermissionSyncCoordinator().SyncAll()
}
