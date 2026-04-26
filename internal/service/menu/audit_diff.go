package menu

import (
	"sort"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/auditdiff"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

var menuDiffRules = []auditdiff.FieldRule{
	{Field: "id", Label: "菜单ID"},
	{Field: "icon", Label: "图标"},
	{Field: "title_i18n", Label: "标题"},
	{Field: "code", Label: "权限标识"},
	{Field: "path", Label: "路由路径"},
	{Field: "full_path", Label: "完整路径"},
	{Field: "name", Label: "路由名称"},
	{Field: "component", Label: "组件"},
	{
		Field: "status",
		Label: "状态",
		ValueLabels: map[string]string{
			"0": "禁用",
			"1": "启用",
		},
	},
	{
		Field: "type",
		Label: "类型",
		ValueLabels: map[string]string{
			"1": "目录",
			"2": "菜单",
			"3": "按钮",
		},
	},
	{
		Field: "is_show",
		Label: "显示",
		ValueLabels: map[string]string{
			"0": "否",
			"1": "是",
		},
	},
	{
		Field: "is_auth",
		Label: "鉴权",
		ValueLabels: map[string]string{
			"0": "否",
			"1": "是",
		},
	},
	{
		Field: "is_new_window",
		Label: "新窗口",
		ValueLabels: map[string]string{
			"0": "否",
			"1": "是",
		},
	},
	{
		Field: "is_external_links",
		Label: "外链",
		ValueLabels: map[string]string{
			"0": "否",
			"1": "是",
		},
	},
	{Field: "sort", Label: "排序"},
	{Field: "pid", Label: "上级菜单ID"},
	{Field: "pids", Label: "上级路径"},
	{Field: "level", Label: "层级"},
	{Field: "redirect", Label: "重定向"},
	{Field: "animate_enter", Label: "进入动画"},
	{Field: "animate_leave", Label: "离开动画"},
	{Field: "animate_duration", Label: "动画时长"},
	{Field: "description", Label: "描述"},
	{Field: "api_list", Label: "接口ID列表"},
}

// CreateWithAuditDiff 新增菜单并返回精确 change_diff。
func (s *MenuService) CreateWithAuditDiff(params *form.CreateMenu, _ string) (string, error) {
	if params == nil {
		return "", e.NewBusinessError(e.InvalidParameter)
	}
	menuModel, err := s.applyMenuMutation(&menuMutation{
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
	if err != nil {
		return "", err
	}
	after, err := s.snapshotMenuByID(menuModel.ID)
	if err != nil {
		return auditdiff.Marshal(nil), nil
	}
	return buildMenuDiff(nil, after), nil
}

// UpdateWithAuditDiff 更新菜单并返回精确 change_diff。
func (s *MenuService) UpdateWithAuditDiff(params *form.UpdateMenu, _ string) (string, error) {
	if params == nil {
		return "", e.NewBusinessError(e.InvalidParameter)
	}
	before, err := s.snapshotMenuByID(params.Id)
	if err != nil {
		return "", err
	}
	if _, err := s.applyMenuMutation(&menuMutation{
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
	}); err != nil {
		return "", err
	}
	after, err := s.snapshotMenuByID(params.Id)
	if err != nil {
		return auditdiff.Marshal(nil), nil
	}
	return buildMenuDiff(before, after), nil
}

// DeleteWithAuditDiff 删除菜单并返回精确 change_diff。
func (s *MenuService) DeleteWithAuditDiff(id uint) (string, error) {
	before, err := s.snapshotMenuByID(id)
	if err != nil {
		return "", err
	}
	if err := s.Delete(id); err != nil {
		return "", err
	}
	return buildMenuDiff(before, nil), nil
}

func (s *MenuService) snapshotMenuByID(id uint) (map[string]any, error) {
	menuModel := model.NewMenu()
	if err := menuModel.GetById(id); err != nil || menuModel.ID == 0 {
		return nil, e.NewBusinessError(e.MenuNotFound)
	}
	titleI18n, err := model.NewMenuI18n().LocaleTitleMapByMenuID(menuModel.ID)
	if err != nil {
		return nil, err
	}
	apiIDs, err := model.NewMenuApiMap().ApiIdsByMenuId(menuModel.ID)
	if err != nil {
		return nil, err
	}
	sort.Slice(apiIDs, func(i, j int) bool {
		return apiIDs[i] < apiIDs[j]
	})
	return map[string]any{
		"id":                menuModel.ID,
		"icon":              menuModel.Icon,
		"title_i18n":        titleI18n,
		"code":              menuModel.Code,
		"path":              menuModel.Path,
		"full_path":         menuModel.FullPath,
		"name":              menuModel.Name,
		"component":         menuModel.Component,
		"status":            menuModel.Status,
		"type":              menuModel.Type,
		"is_show":           menuModel.IsShow,
		"is_auth":           menuModel.IsAuth,
		"is_new_window":     menuModel.IsNewWindow,
		"is_external_links": menuModel.IsExternalLinks,
		"sort":              menuModel.Sort,
		"pid":               menuModel.Pid,
		"pids":              menuModel.Pids,
		"level":             menuModel.Level,
		"redirect":          menuModel.Redirect,
		"animate_enter":     menuModel.AnimateEnter,
		"animate_leave":     menuModel.AnimateLeave,
		"animate_duration":  menuModel.AnimateDuration,
		"description":       menuModel.Description,
		"api_list":          apiIDs,
	}, nil
}

func buildMenuDiff(before, after map[string]any) string {
	items := auditdiff.BuildFieldDiff(before, after, menuDiffRules)
	return auditdiff.Marshal(items)
}
