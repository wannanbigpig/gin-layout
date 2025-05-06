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

// ApiService 登录授权服务
type ApiService struct {
	service.Base
}

func NewApiService() *ApiService {
	return &ApiService{}
}

// Edit 权限编辑
func (s *ApiService) Edit(params *form.EditPermission) error {
	apiModel := model.NewApi()
	data := map[string]any{
		"name":    params.Name,
		"desc":    params.Desc,
		"is_auth": params.IsAuth,
		"sort":    params.Sort,
	}
	if params.Id > 0 {
		if !apiModel.ExistsById(apiModel, params.Id) {
			return e.NewBusinessError(1, "编辑的权限不存在")
		}
		return apiModel.Update(apiModel, params.Id, data)
	}
	data["func"] = params.Func
	data["func_path"] = params.FuncPath
	data["method"] = params.Method
	data["code"] = utils.MD5(params.Method + "_" + params.Route)
	data["route"] = params.Route
	exists := apiModel.Exists(apiModel, "route =?", params.Route)
	if exists {
		return e.NewBusinessError(1, "权限路由已存在")
	}
	return apiModel.Create(apiModel, data)
}

// ListPage 权限列表
func (s *ApiService) ListPage(params *form.ListPermission) *resources.Collection {
	var condition strings.Builder
	var args []any
	if params.Keyword != "" {
		condition.WriteString("(name like ? OR route like ? OR code = ?) AND ")
		args = append(args, "%"+params.Keyword+"%")
		args = append(args, "%"+params.Keyword+"%")
		args = append(args, params.Keyword)
	}
	if params.Name != "" {
		condition.WriteString("name like ? AND ")
		args = append(args, "%"+params.Name+"%")
	}
	if params.Method != "" {
		condition.WriteString("method = ? AND ")
		args = append(args, params.Method)
	}
	if params.Route != "" {
		condition.WriteString("route like ? AND ")
		args = append(args, "%"+params.Route+"%")
	}
	if params.IsAuth != nil {
		condition.WriteString("is_auth = ? AND ")
		args = append(args, params.IsAuth)
	}

	if params.IsEffective != nil {
		condition.WriteString("is_effective = ? AND ")
		args = append(args, params.IsEffective)
	}
	conditionStr := condition.String()
	if conditionStr != "" {
		conditionStr = strings.TrimSuffix(condition.String(), "AND ")
	}

	apiModel := model.NewApi()
	total, collection := model.ListPage[model.Api](apiModel, params.Page, params.PerPage, conditionStr, args, model.ListOptionalParams{
		OrderBy: "sort desc, id desc",
	})
	return resources.NewApiTransformer().ToCollection(params.Page, params.PerPage, total, collection)
}
