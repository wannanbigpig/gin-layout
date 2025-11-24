package permission

import (
	"strings"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	"github.com/wannanbigpig/gin-layout/pkg/utils"
)

// ApiService API权限服务
type ApiService struct {
	service.Base
}

// NewApiService 创建API服务实例
func NewApiService() *ApiService {
	return &ApiService{}
}

// Edit 编辑API权限（新增或更新）
func (s *ApiService) Edit(params *form.EditPermission) error {
	apiModel := model.NewApi()

	// 编辑模式：验证API是否存在
	if params.Id > 0 {
		if !apiModel.ExistsById(apiModel, params.Id) {
			return e.NewBusinessError(1, "编辑的权限不存在")
		}
		return s.updateApi(apiModel, params)
	}

	// 新增模式：验证路由唯一性并创建
	return s.createApi(apiModel, params)
}

// updateApi 更新API权限
func (s *ApiService) updateApi(apiModel *model.Api, params *form.EditPermission) error {
	data := map[string]any{
		"name":        params.Name,
		"description": params.Description,
		"is_auth":     params.IsAuth,
		"sort":        params.Sort,
	}
	return apiModel.Update(apiModel, params.Id, data)
}

// createApi 创建新API权限
func (s *ApiService) createApi(apiModel *model.Api, params *form.EditPermission) error {
	// 验证路由唯一性
	if apiModel.Exists(apiModel, "route =?", params.Route) {
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

	return apiModel.Create(apiModel, data)
}

// ListPage 分页查询API权限列表
func (s *ApiService) ListPage(params *form.ListPermission) *resources.Collection {
	condition, args := s.buildListCondition(params)

	apiModel := model.NewApi()
	total, collection := model.ListPage(
		apiModel,
		params.Page,
		params.PerPage,
		condition,
		args,
		model.ListOptionalParams{
			OrderBy: "sort desc, id desc",
		},
	)

	return resources.NewApiTransformer().ToCollection(params.Page, params.PerPage, total, collection)
}

// buildListCondition 构建列表查询条件
func (s *ApiService) buildListCondition(params *form.ListPermission) (string, []any) {
	var condition strings.Builder
	var args []any

	// 关键词搜索
	if params.Keyword != "" {
		condition.WriteString("(name like ? OR route like ? OR code = ?) AND ")
		args = append(args, "%"+params.Keyword+"%", "%"+params.Keyword+"%", params.Keyword)
	}

	// 名称过滤
	if params.Name != "" {
		condition.WriteString("name like ? AND ")
		args = append(args, "%"+params.Name+"%")
	}

	// 请求方法过滤
	if params.Method != "" {
		condition.WriteString("method = ? AND ")
		args = append(args, params.Method)
	}

	// 路由过滤
	if params.Route != "" {
		condition.WriteString("route like ? AND ")
		args = append(args, "%"+params.Route+"%")
	}

	// 鉴权状态过滤
	if params.IsAuth != nil {
		condition.WriteString("is_auth = ? AND ")
		args = append(args, params.IsAuth)
	}

	// 有效性过滤
	if params.IsEffective != nil {
		condition.WriteString("is_effective = ? AND ")
		args = append(args, params.IsEffective)
	}

	return condition.String(), args
}
