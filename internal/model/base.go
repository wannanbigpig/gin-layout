package model

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
	"gorm.io/plugin/soft_delete"

	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

type BaseModel struct {
	ID        uint             `gorm:"column:id;type:int(11) unsigned AUTO_INCREMENT;not null;primarykey" json:"id"`
	CreatedAt utils.FormatDate `gorm:"column:created_at;type:datetime;<-:create" json:"created_at"`
	UpdatedAt utils.FormatDate `gorm:"column:updated_at;type:datetime" json:"updated_at"`
}

func (m *BaseModel) DB(model ...any) *gorm.DB {
	return DB(model...)
}

func (m *BaseModel) Paginate(page, pageSize int) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		// 确保页码从 1 开始
		if page < 1 {
			page = 1
		}

		// 如果 pageSize 大于 0，则使用 pageSize，否则使用默认的 global.PerPage
		limit := global.PerPage
		if pageSize > 0 {
			limit = pageSize
		}

		// 计算 offset，确保从正确的位置开始分页
		offset := (page - 1) * limit

		// 设置 Offset 和 Limit
		return db.Offset(offset).Limit(limit)
	}
}

func (m *BaseModel) Count(model any, condition string, args []any) (count int64, err error) {
	query := m.DB(model)
	if condition != "" {
		query = query.Where(condition, args...)
	}
	err = query.Count(&count).Error
	if err != nil {
		return 0, err
	}
	return
}

// GetById  根据id获取信息
func (m *BaseModel) GetById(model any, id uint) error {
	return m.DB().First(model, id).Error
}

// GetAllById  根据id获取信息,同时获取全部关联表信息
func (m *BaseModel) GetAllById(model any, id uint) error {
	return m.DB().Preload(clause.Associations).First(model, id).Error
}

func (m *BaseModel) GetDetail(model any, condition string, val []any) error {
	return m.DB().Where(condition, val).First(model).Error
}

// ExistsById checks if a record exists with the given ID
func (m *BaseModel) ExistsById(model schema.Tabler, id uint) bool {
	if id == 0 {
		return false
	}
	return m.Exists(model, "id = ?", id)
}

// Exists checks if any record exists matching the given conditions
// Returns false if error occurs or no record found
func (m *BaseModel) Exists(model schema.Tabler, condition string, args ...any) bool {
	if model == nil {
		return false
	}

	var exists bool
	err := m.DB().Model(model).
		Select("1").
		Where(condition, args...).
		Limit(1).
		Scan(&exists).Error

	return err == nil && exists
}

// Update 更新
func (m *BaseModel) Update(model any, id uint, data map[string]any) error {
	return m.DB().Model(model).Where("id = ?", id).Updates(data).Error
}

// Delete 根据ID删除
func (m *BaseModel) Delete(model any, conds ...any) error {
	return m.DB().Delete(model, conds...).Error
}

// DeleteWithCondition 根据条件删除
func (m *BaseModel) DeleteWithCondition(model any, condition string, args ...any) error {
	return m.DB().Where(condition, args...).Delete(model).Error
}

// Create 创建
func (m *BaseModel) Create(model any, data map[string]any) error {
	return m.DB().Model(model).Create(data).Error
}

type ContainsDeleteBaseModel struct {
	BaseModel
	DeletedAt soft_delete.DeletedAt `gorm:"column:deleted_at;type:int(11) unsigned;not null;default:0;index;" json:"-"`
}

func DB(model ...any) *gorm.DB {
	db := data.MysqlDB
	if model != nil {
		return db.Model(model[0])
	}
	return db
}

// AnyModelInterface 定义一个接口约束，包含所需的方法
type AnyModelInterface[T any] interface {
	Count(model any, condition string, args []any) (int64, error)
	DB(model ...any) *gorm.DB
	Paginate(page, perPage int) func(*gorm.DB) *gorm.DB
	*T // 确保我们可以返回指针类型
}

// ListOptionalParams 定义可选参数结构体
type ListOptionalParams struct {
	SelectFields []string
	OrderBy      string
}

// ListPage 获取分页列表
func ListPage[T any, M AnyModelInterface[T]](model M, page, perPage int, condition string, args []any, optional ...ListOptionalParams) (int64, []*T) {
	total, err := model.Count(model, condition, args)
	if err != nil || total == 0 {
		return total, nil
	}
	query := model.DB(model).Scopes(model.Paginate(page, perPage))
	if condition != "" {
		query = query.Where(condition, args...)
	}

	res := make([]*T, 0, perPage)

	if len(optional) > 0 {
		if len(optional[0].SelectFields) > 0 {
			query = query.Select(optional[0].SelectFields)
		}
		if optional[0].OrderBy != "" {
			query = query.Order(optional[0].OrderBy)
		} else {
			query = query.Order("id desc")
		}
	}
	err = query.Find(&res).Error
	if err != nil {
		return total, nil
	}
	return total, res
}

// List 获取列表
func List[T any, M AnyModelInterface[T]](model M, condition string, args []any, optional ...ListOptionalParams) []*Menu {
	query := model.DB(model)
	if condition != "" {
		query = query.Where(condition, args...)
	}
	var res []*Menu
	if len(optional) > 0 {
		if len(optional[0].SelectFields) > 0 {
			query = query.Select(optional[0].SelectFields)
		}
		if optional[0].OrderBy != "" {
			query = query.Order(optional[0].OrderBy)
		} else {
			query = query.Order("id desc")
		}
	}
	err := query.Find(&res).Error
	if err != nil {
		return nil
	}
	return res
}

// Save 保存
func Save(model any) error {
	return DB().Save(model).Error
}

// HasChildren 判断是否有子菜单
func HasChildren[T any, M AnyModelInterface[T]](model M, pid uint) (bool, error) {
	count, err := model.Count(model, "pid =?", []any{pid})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
