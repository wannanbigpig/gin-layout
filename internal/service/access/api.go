package access

import (
	"strings"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	"github.com/wannanbigpig/gin-layout/pkg/utils"
)

// ApiService 处理 API 权限的维护与查询。
type ApiService struct {
	service.Base
}

// NewApiService 创建 API 服务实例。
func NewApiService() *ApiService {
	return &ApiService{}
}

// Create 新增 API 权限。
func (s *ApiService) Create(params *form.CreatePermission) error {
	return s.createApi(model.NewApi(), params)
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
	return s.updateApi(apiModel, params)
}

// Edit 兼容旧编辑入口，等同于更新。
func (s *ApiService) Edit(params *form.UpdatePermission) error {
	return s.Update(params)
}

// updateApi 更新 API 权限并刷新路由缓存与权限策略。
func (s *ApiService) updateApi(apiModel *model.Api, params *form.UpdatePermission) error {
	data := map[string]any{
		"name":        params.Name,
		"description": params.Description,
		"is_auth":     params.IsAuth,
		"sort":        params.Sort,
	}
	if err := apiModel.UpdateById(params.Id, data); err != nil {
		return err
	}
	if err := NewApiRouteCacheService().RefreshCache(); err != nil {
		return err
	}
	return NewPermissionSyncCoordinator().SyncAll()
}

// createApi 创建新 API 权限并刷新路由缓存与权限策略。
func (s *ApiService) createApi(apiModel *model.Api, params *form.CreatePermission) error {
	exists, err := apiModel.Exists("route =?", params.Route)
	if err != nil {
		return err
	}
	if exists {
		return e.NewBusinessError(1, "权限路由已存在")
	}

	data := map[string]any{
		"name":        params.Name,
		"description": params.Description,
		"is_auth":     params.IsAuth,
		"sort":        params.Sort,
		"func":        params.Func,
		"func_path":   params.FuncPath,
		"method":      params.Method,
		"code":        utils.MD5(params.Method + "_" + params.Route),
		"route":       params.Route,
	}

	if err := apiModel.Create(data); err != nil {
		return err
	}
	if err := NewApiRouteCacheService().RefreshCache(); err != nil {
		return err
	}
	return NewPermissionSyncCoordinator().SyncAll()
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
	var conditions []string
	var args []any

	if params.Keyword != "" {
		conditions = append(conditions, "(name like ? OR route like ? OR code = ?)")
		args = append(args, "%"+params.Keyword+"%", "%"+params.Keyword+"%", params.Keyword)
	}

	if params.Name != "" {
		conditions = append(conditions, "name like ?")
		args = append(args, "%"+params.Name+"%")
	}

	if params.Method != "" {
		conditions = append(conditions, "method = ?")
		args = append(args, params.Method)
	}

	if params.Route != "" {
		conditions = append(conditions, "route like ?")
		args = append(args, "%"+params.Route+"%")
	}

	if params.IsAuth != nil {
		conditions = append(conditions, "is_auth = ?")
		args = append(args, params.IsAuth)
	}

	if params.IsEffective != nil {
		conditions = append(conditions, "is_effective = ?")
		args = append(args, params.IsEffective)
	}

	return strings.Join(conditions, " AND "), args
}
