package resources

import (
	"github.com/jinzhu/copier"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/internal/global"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
)

// Resources 定义模型到响应资源的转换接口。
type Resources[T any, R any] interface {
	ToStruct(data T) R
	ToCollection(page, perPage int, total int64, data []T) *Collection
}

// CustomFieldSetter 允许资源在复制基础字段后补充扩展信息。
type CustomFieldSetter[T any] interface {
	SetCustomFields(T)
}

// BaseResources 提供通用的资源转换实现。
type BaseResources[T any, R any] struct {
	NewResource func() R
}

// ToStruct 将单个模型复制为资源结构。
func (br BaseResources[T, R]) ToStruct(data T) R {
	resource, _ := toGenericStruct(data, br.NewResource)
	return resource
}

// ToCollection 将模型集合转换为统一分页响应。
func (br BaseResources[T, R]) ToCollection(page, perPage int, total int64, data []T) *Collection {
	items := make([]any, 0, len(data))
	for _, item := range data {
		items = append(items, br.ToStruct(item))
	}
	return NewCollection().SetPaginate(page, perPage, total).ToCollection(items)
}

// ToAnySlice 将泛型切片转换为 []any。
func ToAnySlice[T any](data []T) []any {
	items := make([]any, len(data))
	for i, v := range data {
		items[i] = v
	}
	return items
}

// TreeNode 表示可挂载子节点的树形资源。
type TreeNode[R any] interface {
	SetChildren(children []R)
}

// Identifiable 表示可参与树构建的节点标识接口。
type Identifiable interface {
	GetID() uint
	GetPID() uint
}

// TreeResources 定义树形资源的转换接口。
type TreeResources[T any, R TreeNode[R]] interface {
	ToStruct(data T) R
	BuildTree(data []T, pidFn func(T) uint, idFn func(T) uint) []R
}

// TreeResource 提供通用的树形资源转换实现。
type TreeResource[T any, R TreeNode[R]] struct {
	NewResource func() R
}

// ToStruct 将单个模型复制为树形资源节点。
func (tr TreeResource[T, R]) ToStruct(data T) R {
	resource, _ := toGenericStruct(data, tr.NewResource)
	return resource
}

// BuildTree 根据父子关系构建树形结果。
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

// BuildTreeByNode 使用资源节点自带的标识信息构建树。
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

// toGenericStruct 复制模型字段并补充自定义资源字段。
func toGenericStruct[T any, R any](data T, newFunc func() R) (R, error) {
	var resource = newFunc()
	err := copier.Copy(&resource, data)
	if err != nil {
		log.Logger.Error("Copy data to struct error", zap.Error(err))
		return resource, err
	}
	if cfs, ok := any(resource).(CustomFieldSetter[T]); ok {
		cfs.SetCustomFields(data)
	}
	return resource, nil
}

// Paginate 表示统一分页元数据。
type Paginate struct {
	Total       int64 `json:"total"`
	PerPage     int   `json:"per_page"`
	CurrentPage int   `json:"current_page"`
	LastPage    int   `json:"last_page"`
}

// calculateLastPage 归一化分页参数并计算最后一页。
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
	p.LastPage = int((p.Total + int64(p.PerPage) - 1) / int64(p.PerPage))
}

// ResponseCollectionInterface 定义分页集合的基础能力。
type ResponseCollectionInterface interface {
	GetPaginate() *Paginate
	SetPaginate(page, perPage int, total int64) *Collection
	ToCollection(data []any) *Collection
}

// Collection 表示带分页信息的列表响应。
type Collection struct {
	Paginate
	Data []any `json:"data"`
}

// GetPaginate 返回当前集合的分页信息。
func (p *Collection) GetPaginate() *Paginate {
	return &p.Paginate
}

// SetPaginate 设置集合的分页元数据。
func (p *Collection) SetPaginate(page, perPage int, total int64) *Collection {
	p.Paginate = Paginate{
		Total:       total,
		CurrentPage: page,
		PerPage:     perPage,
	}
	p.Paginate.calculateLastPage()
	return p
}

// ToCollection 设置集合数据项。
func (p *Collection) ToCollection(data []any) *Collection {
	p.Data = data
	return p
}

// NewCollection 创建空的分页集合。
func NewCollection() *Collection {
	return &Collection{}
}

// ToRawCollection 直接将模型切片包装为分页响应。
func ToRawCollection[T any](page, perPage int, total int64, data []T) *Collection {
	items := make([]any, len(data))
	for i, v := range data {
		items[i] = v
	}
	return NewCollection().SetPaginate(page, perPage, total).ToCollection(items)
}
