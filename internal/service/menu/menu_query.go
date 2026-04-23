package menu

import (
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/i18n"
	"github.com/wannanbigpig/gin-layout/internal/pkg/query_builder"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// ListPage 分页查询菜单列表
func (s *MenuService) ListPage(params *form.ListMenu) *resources.Collection {
	condition, args := s.buildListCondition(params, false)
	menu := model.NewMenu()
	total, collection, err := model.ListPageE(menu, params.Page, params.PerPage, condition, args)
	if err != nil {
		return resources.NewMenuTransformer().ToCollection(params.Page, params.PerPage, 0, nil)
	}
	return resources.NewMenuTransformer().ToCollectionWithTitles(params.Page, params.PerPage, total, collection, nil)
}

// List 查询菜单树形列表
func (s *MenuService) List(params *form.ListMenu, locale string) any {
	condition, args := s.buildListCondition(params, true)
	menus, err := model.ListE(model.NewMenu(), condition, args, model.ListOptionalParams{
		OrderBy: "sort desc, id desc",
	})
	if err != nil {
		return resources.BuildMenuTree(nil, 0, nil)
	}
	localeTitles, err := s.loadLocalizedTitles(menus, locale)
	if err != nil {
		return resources.BuildMenuTree(menus, 0, nil)
	}
	return resources.BuildMenuTree(menus, 0, localeTitles)
}

// Delete 删除菜单
func (s *MenuService) Delete(id uint) error {
	menu := model.NewMenu()
	if err := menu.GetById(id); err != nil || menu.ID == 0 {
		return e.NewBusinessError(e.MenuNotFound)
	}
	if menu.ChildrenNum > 0 {
		return e.NewBusinessError(e.MenuHasChildren)
	}

	db, err := menu.GetDB()
	if err != nil {
		return e.NewBusinessError(e.MenuCannotDelete)
	}
	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		menu.SetDB(tx)
		parentID := menu.Pid
		if _, deleteErr := menu.DeleteByID(id); deleteErr != nil {
			return deleteErr
		}
		if err := model.NewMenuI18n().DeleteByMenuIDs([]uint{id}, tx); err != nil {
			return err
		}
		if parentID > 0 {
			return model.UpdateChildrenNum(model.NewMenu(), parentID, tx)
		}
		return nil
	})
	if err != nil {
		return e.NewBusinessError(e.MenuCannotDelete)
	}
	return access.NewPermissionSyncCoordinator().SyncUsersAffectedByMenus([]uint{id})
}

// Detail 获取菜单详情
func (s *MenuService) Detail(id uint, _ string) (any, error) {
	menu := model.NewMenu()
	if err := menu.GetAllById(id); err != nil || menu.ID == 0 {
		return nil, e.NewBusinessError(e.MenuNotFound)
	}
	titleI18n, err := model.NewMenuI18n().LocaleTitleMapByMenuID(menu.ID)
	if err != nil {
		return nil, err
	}
	return resources.NewMenuTransformer().ToStructWithTitles(menu, "", titleI18n), nil
}

func (s *MenuService) buildListCondition(params *form.ListMenu, includeStatus bool) (string, []any) {
	qb := query_builder.New()
	if params.Keyword != "" {
		qb.AddCondition("(id IN (SELECT menu_id FROM menu_i18n WHERE title like ?) OR path like ? OR code = ?)", "%"+params.Keyword+"%", "%"+params.Keyword+"%", params.Keyword)
	}
	qb.AddEq("is_auth", params.IsAuth)
	if includeStatus && params.Status != nil && *params.Status != allStatus {
		qb.AddEq("status", params.Status)
	}
	return qb.Build()
}

func (s *MenuService) loadLocalizedTitles(menus []*model.Menu, locale string) (map[uint]string, error) {
	menuIDs := make([]uint, 0, len(menus))
	for _, menu := range menus {
		if menu == nil {
			continue
		}
		menuIDs = append(menuIDs, menu.ID)
	}
	if len(menuIDs) == 0 {
		return map[uint]string{}, nil
	}
	return model.NewMenuI18n().LocalizedTitleMapByMenuIDs(menuIDs, menuLocalePriority(locale))
}

func menuLocalePriority(locale string) []string {
	return []string{
		i18n.NormalizeLocale(locale),
		i18n.LocaleZhCN,
		i18n.LocaleEnUS,
	}
}
