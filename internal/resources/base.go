package resources

import (
	"math"

	"github.com/jinzhu/copier"

	"github.com/wannanbigpig/gin-layout/internal/global"
)

// Resources
// T 表示模型类型，例如 *model.AdminUser
// R 表示资源类型，例如 *AdminUserResources
// 实现该接口可用于将模型转换为响应资源结构体，适用于单个数据与集合分页封装
// 泛型约束使结构复用性更强
// ToStruct 将模型类型 T 转换为资源类型 R
// ToCollection 将模型切片转换为资源集合 Collection，并封装分页信息
type Resources[T any, R any] interface {
	ToStruct(data T) R
	ToCollection(page, perPage int, total int64, data []T) *Collection
}

// BaseResources 提供 Resources 接口的默认实现，供各模型资源结构体嵌入使用
// NewResource 是一个函数，返回一个空的资源结构体实例（如 &AdminUserResources{}）
type BaseResources[T any, R any] struct {
	NewResource func() R
}

// ToStruct 实现单个结构体的复制逻辑，利用 copier.Copy 自动映射字段
func (br BaseResources[T, R]) ToStruct(data T) R {
	resource := br.NewResource()
	_ = copier.Copy(&resource, data)
	return resource
}

// ToCollection 实现列表结构体复制逻辑
// 并将转换后的结果封装为统一分页格式 Collection
func (br BaseResources[T, R]) ToCollection(page, perPage int, total int64, data []T) *Collection {
	items := make([]any, len(data))
	for i, v := range data {
		items[i] = br.ToStruct(v)
	}
	return NewCollection().SetPaginate(page, perPage, total).ToCollection(items)
}

// Paginate 分页结构体
// Total 总记录数
// PerPage 每页显示的记录数
// CurrentPage 当前页码
// LastPage 最后一页页码
type Paginate struct {
	Total       int64 `json:"total"`
	PerPage     int   `json:"per_page"`
	CurrentPage int   `json:"current_page"`
	LastPage    int   `json:"last_page"`
}

// calculateLastPage 计算最后一页
// 当 CurrentPage 或 PerPage 小于 1 时，将其重置为默认值
// 最后一页页码 LastPage 计算逻辑：Total 除以 PerPage，向上取整得到最后一页页码
// 确保 LastPage 始终大于等于 1
func (p *Paginate) calculateLastPage() {
	if p.CurrentPage < 1 {
		p.CurrentPage = 1
	}

	if p.PerPage < 1 {
		p.PerPage = global.PerPage
	}

	if p.Total == 0 {
		p.LastPage = 1
		return
	}
	p.LastPage = int(math.Ceil(float64(p.Total) / float64(p.PerPage)))
}

// ResponseCollectionInterface 响应集合接口
// GetPaginate 返回集合的基础分页信息
// SetPaginate 设置集合的分页信息
// ToCollection 将数据转换为集合
type ResponseCollectionInterface interface {
	GetPaginate() *Paginate
	SetPaginate(page, perPage int, total int64) *Collection
	ToCollection(data []any) *Collection
}

// Collection 集合结构体
type Collection struct {
	Paginate
	Data []any `json:"data"`
}

// GetPaginate 返回集合的基础分页信息。
// 该方法用于获取与集合关联的分页对象，允许调用者访问和操作分页相关属性。
func (p *Collection) GetPaginate() *Paginate {
	return &p.Paginate
}

// SetPaginate 设置集合的分页信息。
func (p *Collection) SetPaginate(page, perPage int, total int64) *Collection {
	p.Paginate = Paginate{
		Total:       total,
		CurrentPage: page,
		PerPage:     perPage,
	}
	p.Paginate.calculateLastPage()
	return p
}

// ToCollection 将数据转换为集合。
func (p *Collection) ToCollection(data []any) *Collection {
	p.Data = data
	return p
}

// NewCollection 构建响应集合
func NewCollection() *Collection {
	return &Collection{}
}

// ToRawCollection 用于不需要自定义字段处理时，直接将模型结构体原样返回
// 适用于不需要字段脱敏等额外逻辑的场景
func ToRawCollection[T any](page, perPage int, total int64, data []T) *Collection {
	items := make([]any, len(data))
	for i, v := range data {
		items[i] = v
	}
	return NewCollection().SetPaginate(page, perPage, total).ToCollection(items)
}
