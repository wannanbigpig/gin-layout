package resources

import (
	"github.com/wannanbigpig/gin-layout/internal/model"
)

type ApiResources struct {
	ID              uint    `json:"id"`
	Name            string  `json:"name"`              // 权限名称
	Code            string  `json:"code"`              // 权限名称
	Description     string  `json:"description"`       // 描述
	Method          string  `json:"method"`            // 接口请求方法
	Route           string  `json:"route"`             // 接口路由
	Func            string  `json:"func"`              // 接口方法
	FuncPath        string  `json:"func_path"`         // 接口方法
	IsAuth          uint8   `json:"is_auth"`           // 是否授权
	IsEffective     uint8   `json:"is_effective"`      // 是否有效
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

func (ApiTransformer) ToStruct(data *model.Api) *ApiResources {
	isAuthName := data.IsAuthMap()
	isEffectiveName := data.IsEffectiveMap()
	return &ApiResources{
		ID:              data.ID,
		Name:            data.Name,
		Description:     data.Description,
		Method:          data.Method,
		Route:           data.Route,
		Func:            data.Func,
		FuncPath:        data.FuncPath,
		IsAuth:          data.IsAuth,
		IsAuthName:      &isAuthName,
		Sort:            data.Sort,
		Code:            data.Code,
		IsEffective:     data.IsEffective,
		IsEffectiveName: &isEffectiveName,
	}
}

func (ApiTransformer) ToCollection(page, perPage int, total int64, data []*model.Api) *Collection {
	response := make([]any, 0, len(data))
	for _, v := range data {
		response = append(response, ApiTransformer{}.ToStruct(v))
	}
	return NewCollection().SetPaginate(page, perPage, total).ToCollection(response)
}
