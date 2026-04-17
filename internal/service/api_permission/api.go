package api_permission

import (
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/query_builder"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/service/access"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

// ApiService 处理 API 权限的维护与查询。
type ApiService struct {
	service.Base
}

// NewApiService 创建 API 服务实例。
func NewApiService() *ApiService {
	return &ApiService{}
}

// Update 更新 API 权限。
func (s *ApiService) Update(params *form.UpdatePermission) error {
	apiModel := model.NewApi()
	exists, err := apiModel.ExistsById(params.Id)
	if err != nil {
		return err
	}
	if !exists {
		return e.NewBusinessError(1, "编辑的权限不存在")
	}
	data := map[string]any{
		"name":        params.Name,
		"description": params.Description,
		"is_auth":     params.IsAuth,
		"sort":        params.Sort,
	}
	if err := apiModel.UpdateById(params.Id, data); err != nil {
		return err
	}
	if err := access.NewApiRouteCacheService().RefreshCache(); err != nil {
		return err
	}
	return access.NewPermissionSyncCoordinator().SyncUsersAffectedByAPIs([]uint{params.Id})
}

// ListPage 分页查询 API 权限列表。
func (s *ApiService) ListPage(params *form.ListPermission) *resources.Collection {
	condition, args := s.buildListCondition(params)

	apiModel := model.NewApi()
	total, collection, err := model.ListPageE(
		apiModel,
		params.Page,
		params.PerPage,
		condition,
		args,
		model.ListOptionalParams{
			OrderBy: "sort desc, id desc",
		},
	)
	if err != nil {
		return resources.NewApiTransformer().ToCollection(params.Page, params.PerPage, 0, nil)
	}

	return resources.NewApiTransformer().ToCollection(params.Page, params.PerPage, total, collection)
}

// buildListCondition 构建 API 权限列表查询条件。
func (s *ApiService) buildListCondition(params *form.ListPermission) (string, []any) {
	qb := query_builder.New()
	if params.Keyword != "" {
		qb.AddCondition("(name like ? OR route like ? OR code = ?)", "%"+params.Keyword+"%", "%"+params.Keyword+"%", params.Keyword)
	}

	qb.AddLike("name", params.Name).
		AddEq("method", emptyToNil(params.Method)).
		AddLike("route", params.Route).
		AddEq("is_auth", params.IsAuth).
		AddEq("is_effective", params.IsEffective)

	return qb.Build()
}

func emptyToNil(value string) any {
	if value == "" {
		return nil
	}
	return value
}
