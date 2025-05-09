package resources

import (
	"math"

	"github.com/jinzhu/copier"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/internal/global"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
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

// CustomFieldSetter 定义资源结构体可选的自定义字段设置接口
type CustomFieldSetter[T any] interface {
	SetCustomFields(T)
}

// BaseResources 提供 Resources 接口的默认实现，供各模型资源结构体嵌入使用
// NewResource 是一个函数，返回一个空的资源结构体实例（如 &AdminUserResources{}）
type BaseResources[T any, R any] struct {
	NewResource func() R
}

// ToStruct 实现单个结构体的复制逻辑，利用 copier.Copy 自动映射字段
func (br BaseResources[T, R]) ToStruct(data T) R {
	resource, _ := toGenericStruct(data, br.NewResource)
	return resource
}

// ToCollection 实现列表结构体复制逻辑
// 并将转换后的结果封装为统一分页格式 Collection
func (br BaseResources[T, R]) ToCollection(page, perPage int, total int64, data []T) *Collection {
	return NewCollection().SetPaginate(page, perPage, total).ToCollection(ToAnySlice(data))
}

// ToAnySlice 将泛型切片转换为 any 类型切片
// 适用于将资源切片转换为 Collection 中的 Items 字段
func ToAnySlice[T any](data []T) []any {
	items := make([]any, len(data))
	for i, v := range data {
		items[i] = v
	}
	return items
}

// TreeNode 定义具有 Children 的资源节点接口
// 所有用于构建树的资源结构体应实现此接口

type TreeNode[R any] interface {
	SetChildren(children []R)
}

// Identifiable 定义具有 GetID 和 GetPID 的资源节点接口
type Identifiable interface {
	GetID() uint
	GetPID() uint
}

// TreeResources 泛型树形资源转换接口
// R 需实现 TreeNode 接口，适用于如菜单这类树状结构
type TreeResources[T any, R TreeNode[R]] interface {
	ToStruct(data T) R
	BuildTree(data []T, pidFn func(T) uint, idFn func(T) uint) []R
}

// TreeResource 提供 TreeResources 接口的默认实现
// 专用于需要树形结构的资源转换
type TreeResource[T any, R TreeNode[R]] struct {
	NewResource func() R
}

// ToStruct 实现单个结构体的复制逻辑，利用 copier.Copy 自动映射字段
// 支持自定义字段设置，如菜单的图标、链接等
func (tr TreeResource[T, R]) ToStruct(data T) R {
	resource, _ := toGenericStruct(data, tr.NewResource)
	return resource
}

// BuildTree 递归构建树形结构
// 利用 pidFn 和 idFn 函数，将数据转换为树状结构
// 适用于菜单、分类等树形结构的构建
func (tr TreeResource[T, R]) BuildTree(data []T, pidFn func(T) uint, idFn func(T) uint, rootID uint) []R {
	parentMap := make(map[uint][]T)
	for _, item := range data {
		pid := pidFn(item)
		parentMap[pid] = append(parentMap[pid], item)
	}

	var build func(uint) []R
	build = func(parentID uint) []R {
		children, ok := parentMap[parentID]
		if !ok {
			return nil
		}

		var tree []R
		for _, v := range children {
			resource := tr.ToStruct(v)
			resource.SetChildren(build(idFn(v)))
			tree = append(tree, resource)
		}
		return tree
	}

	return build(rootID)
}

// BuildTreeByNode 自动使用资源结构体的 GetPID 和 GetID，无需外部传入函数
func (tr TreeResource[T, R]) BuildTreeByNode(data []T, rootID uint) []R {
	if len(data) == 0 {
		return []R{}
	}

	parentMap := make(map[uint][]T)
	for _, item := range data {
		resource := tr.ToStruct(item)
		if identifiable, ok := any(resource).(Identifiable); ok {
			pid := identifiable.GetPID()
			parentMap[pid] = append(parentMap[pid], item)
		}
	}

	var build func(uint) []R
	build = func(parentID uint) []R {
		children := parentMap[parentID]
		var tree []R
		for _, v := range children {
			resource := tr.ToStruct(v)
			if identifiable, ok := any(resource).(Identifiable); ok {
				resource.SetChildren(build(identifiable.GetID()))
			}
			tree = append(tree, resource)
		}
		return tree
	}

	return build(rootID)
}

// 抽象出通用的 ToStruct 方法
func toGenericStruct[T any, R any](data T, newFunc func() R) (R, error) {
	var resource = newFunc()
	err := copier.Copy(&resource, data)
	if err != nil {
		log.Logger.Error("Copy data to struct error", zap.Error(err))
		// 根据实际需求决定如何处理错误
		return resource, err
	}
	if cfs, ok := any(resource).(CustomFieldSetter[T]); ok {
		cfs.SetCustomFields(data)
	}
	return resource, nil
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

	if p.PerPage < 0 {
		p.PerPage = 10 // fallback 默认值
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
