package resources

import (
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/i18n"
)

// ApiResources 表示接口权限的响应结构。
type ApiResources struct {
	ID              uint    `json:"id"`
	Name            string  `json:"name"`              // 权限名称
	Code            string  `json:"code"`              // 权限名称
	Description     string  `json:"description"`       // 描述
	Method          string  `json:"method"`            // 接口请求方法
	Route           string  `json:"route"`             // 接口路由
	Func            string  `json:"func"`              // 接口方法
	FuncPath        string  `json:"func_path"`         // 接口方法
	IsAuth          uint8   `json:"is_auth"`           // 接口鉴权模式
	IsEffective     uint8   `json:"is_effective"`      // 是否有效
	IsAuthName      *string `json:"is_auth_name"`      // 接口鉴权模式名称
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

// ToStruct 将 API 模型转换为响应结构。
func (ApiTransformer) ToStruct(data *model.Api) *ApiResources {
	return ApiTransformer{}.ToLocalizedStruct(data, i18n.DefaultLocale)
}

// ToLocalizedStruct 将 API 模型按请求语言转换为响应结构。
func (ApiTransformer) ToLocalizedStruct(data *model.Api, locale string) *ApiResources {
	isAuthName := data.IsAuthMap()
	isEffectiveName := data.IsEffectiveMap()
	return &ApiResources{
		ID:              data.ID,
		Name:            localizedAPIMetadata(data.Name, data.NameI18n, locale),
		Description:     localizedAPIMetadata(data.Description, data.DescriptionI18n, locale),
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

// ToCollection 将 API 模型集合转换为分页响应。
func (ApiTransformer) ToCollection(page, perPage int, total int64, data []*model.Api) *Collection {
	return ApiTransformer{}.ToLocalizedCollection(page, perPage, total, data, i18n.DefaultLocale)
}

// ToLocalizedCollection 按请求语言转换 API 权限列表。
func (ApiTransformer) ToLocalizedCollection(page, perPage int, total int64, data []*model.Api, locale string) *Collection {
	response := make([]any, 0, len(data))
	for _, v := range data {
		response = append(response, ApiTransformer{}.ToLocalizedStruct(v, locale))
	}
	return NewCollection().SetPaginate(page, perPage, total).ToCollection(response)
}

// ApiOptionResources 接口权限选择器选项，仅暴露下拉所需的最小字段。
type ApiOptionResources struct {
	ID     uint   `json:"id"`
	Name   string `json:"name"`
	Code   string `json:"code"`
	Method string `json:"method"`
	Route  string `json:"route"`
}

// ToApiOptions 将 API 模型集合转换为选择器选项列表。
func ToApiOptions(apis []*model.Api, locale ...string) []ApiOptionResources {
	requestLocale := i18n.DefaultLocale
	if len(locale) > 0 {
		requestLocale = locale[0]
	}
	options := make([]ApiOptionResources, 0, len(apis))
	for _, a := range apis {
		if a == nil {
			continue
		}
		options = append(options, ApiOptionResources{
			ID:     a.ID,
			Name:   localizedAPIMetadata(a.Name, a.NameI18n, requestLocale),
			Code:   a.Code,
			Method: a.Method,
			Route:  a.Route,
		})
	}
	return options
}

func localizedAPIMetadata(defaultText, localizedRaw, locale string) string {
	// name/description 保持为中文编辑兼容字段；其他语言从 Catalog 快照解析。
	if i18n.NormalizeLocale(locale) == i18n.LocaleZhCN {
		return defaultText
	}
	return i18n.ResolveLocalizedText(defaultText, localizedRaw, locale)
}
