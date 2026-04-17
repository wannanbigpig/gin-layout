package menu

import (
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
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
	return resources.NewMenuTransformer().ToCollection(params.Page, params.PerPage, total, collection)
}

// List 查询菜单树形列表
func (s *MenuService) List(params *form.ListMenu) any {
	condition, args := s.buildListCondition(params, true)
	menus, err := model.ListE(model.NewMenu(), condition, args, model.ListOptionalParams{
		OrderBy: "sort desc, id desc",
	})
	if err != nil {
		return resources.NewMenuTreeTransformer().BuildTreeByNode(nil, 0)
	}
	return resources.NewMenuTreeTransformer().BuildTreeByNode(menus, 0)
}

// Delete 删除菜单
func (s *MenuService) Delete(id uint) error {
	menu := model.NewMenu()
	if err := menu.GetById(id); err != nil || menu.ID == 0 {
		return e.NewBusinessError(1, "菜单不存在")
	}
	if menu.ChildrenNum > 0 {
		return e.NewBusinessError(1, "该菜单有子菜单，无法删除")
	}

	db, err := menu.GetDB()
	if err != nil {
		return e.NewBusinessError(1, "删除菜单失败")
	}
	err = access.RunInTransaction(db, func(tx *gorm.DB) error {
		menu.SetDB(tx)
		parentID := menu.Pid
		if _, deleteErr := menu.DeleteByID(id); deleteErr != nil {
			return deleteErr
		}
		if parentID > 0 {
			return model.UpdateChildrenNum(model.NewMenu(), parentID, tx)
		}
		return nil
	})
	if err != nil {
		return e.NewBusinessError(1, "删除菜单失败")
	}
	return access.NewPermissionSyncCoordinator().SyncUsersAffectedByMenus([]uint{id})
}

// Detail 获取菜单详情
func (s *MenuService) Detail(id uint) (any, error) {
	menu := model.NewMenu()
	if err := menu.GetAllById(id); err != nil || menu.ID == 0 {
		return nil, e.NewBusinessError(1, "菜单不存在")
	}
	return resources.NewMenuTransformer().ToStruct(menu), nil
}

func (s *MenuService) buildListCondition(params *form.ListMenu, includeStatus bool) (string, []any) {
	qb := query_builder.New()
	if params.Keyword != "" {
		qb.AddCondition("(title like ? OR path like ? OR code = ?)", "%"+params.Keyword+"%", "%"+params.Keyword+"%", params.Keyword)
	}
	qb.AddEq("is_auth", params.IsAuth)
	if includeStatus && params.Status != nil && *params.Status != allStatus {
		qb.AddEq("status", params.Status)
	}
	return qb.Build()
}
