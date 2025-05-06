package resources

import (
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/model/modelDict"
)

type ApiResources struct {
	ID              uint    `json:"id"`
	Name            string  `json:"name"`              // 权限名称
	Code            string  `json:"code"`              // 权限名称
	Desc            string  `json:"desc"`              // 描述
	Method          string  `json:"method"`            // 接口请求方法
	Route           string  `json:"route"`             // 接口路由
	Func            string  `json:"func"`              // 接口方法
	FuncPath        string  `json:"func_path"`         // 接口方法
	IsAuth          int8    `json:"is_auth"`           // 是否授权
	IsEffective     int8    `json:"is_effective"`      // 是否有效
	IsAuthName      *string `json:"is_auth_name"`      // 是否有效
	IsEffectiveName *string `json:"is_effective_name"` // 是否有效
	Sort            int     `json:"sort"`              // 排序
}

// ApiTransformer 权限资源转换
type ApiTransformer struct {
	BaseResources[*model.Api, *ApiResources]
}

// NewApiTransformer 实例化权限资源转换器
func NewApiTransformer() ApiTransformer {
	return ApiTransformer{
		BaseResources: BaseResources[*model.Api, *ApiResources]{
			NewResource: func() *ApiResources {
				return &ApiResources{}
			},
		},
	}
}

func (ApiTransformer) ToCollection(page, perPage int, total int64, data []*model.Api) *Collection {
	response := make([]any, 0, len(data))
	Dict := modelDict.IsMap
	for _, v := range data {
		isAuthName := Dict.Map(v.IsAuth)
		isEffective := Dict.Map(v.IsEffective)
		response = append(response, &ApiResources{
			ID:              v.ID,
			Name:            v.Name,
			Desc:            v.Desc,
			Method:          v.Method,
			Route:           v.Route,
			Func:            v.Func,
			FuncPath:        v.FuncPath,
			IsAuth:          v.IsAuth,
			IsAuthName:      &isAuthName,
			Sort:            v.Sort,
			Code:            v.Code,
			IsEffective:     v.IsEffective,
			IsEffectiveName: &isEffective,
		})
	}
	return NewCollection().SetPaginate(page, perPage, total).ToCollection(response)
}
