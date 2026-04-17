package model

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wannanbigpig/gin-layout/internal/global"
)

// Paginate 返回 GORM 分页作用域，页码小于 1 时会自动修正为第 1 页。
func (m *BaseModel) Paginate(page, pageSize int) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if page < 1 {
			page = 1
		}

		limit := global.PerPage
		if pageSize > 0 {
			limit = pageSize
		}

		offset := (page - 1) * limit
		return db.Offset(offset).Limit(limit)
	}
}

// Count 按条件统计当前模型记录总数。
func (m *BaseModel) Count(condition string, args ...any) (count int64, err error) {
	self, err := m.self()
	if err != nil {
		return 0, err
	}
	query, err := m.GetDB(self)
	if err != nil {
		return 0, err
	}
	if condition != "" {
		query = query.Where(condition, args...)
	}
	err = query.Count(&count).Error
	if err != nil {
		return 0, err
	}
	return
}

// GetById 根据 ID 获取当前模型信息。
func (m *BaseModel) GetById(id uint) error {
	self, err := m.self()
	if err != nil {
		return err
	}
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.First(self, id).Error
}

// GetAllById 根据 ID 获取当前模型及全部关联表信息。
func (m *BaseModel) GetAllById(id uint) error {
	self, err := m.self()
	if err != nil {
		return err
	}
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Preload(clause.Associations).First(self, id).Error
}

// GetDetail 按条件查询当前模型的单条详情记录。
func (m *BaseModel) GetDetail(condition string, val ...any) error {
	self, err := m.self()
	if err != nil {
		return err
	}
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Where(condition, val...).First(self).Error
}

// ExistsById 判断指定 ID 的记录是否存在。
func (m *BaseModel) ExistsById(id uint) (bool, error) {
	if id == 0 {
		return false, nil
	}
	return m.Exists("id = ?", id)
}

// Exists 判断是否存在满足条件的记录。
func (m *BaseModel) Exists(condition string, args ...any) (bool, error) {
	self, err := m.self()
	if err != nil {
		return false, err
	}

	db, err := m.GetDB()
	if err != nil {
		return false, err
	}
	var count int64
	err = db.Model(self).Where(condition, args...).Limit(1).Count(&count).Error
	return count > 0, err
}

// UpdateById 根据 ID 更新当前模型记录。
func (m *BaseModel) UpdateById(id uint, data map[string]any) error {
	self, err := m.self()
	if err != nil {
		return err
	}
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Model(self).Where("id = ?", id).Updates(data).Error
}

// DeleteByID 根据 ID 删除当前模型记录。
func (m *BaseModel) DeleteByID(id uint) (int64, error) {
	self, err := m.self()
	if err != nil {
		return 0, err
	}
	db, err := m.GetDB()
	if err != nil {
		return 0, err
	}
	result := db.Delete(self, id)
	return result.RowsAffected, result.Error
}

// DeleteWhere 按条件删除当前模型记录，空条件会被拒绝以防误删全表。
func (m *BaseModel) DeleteWhere(condition string, args ...any) error {
	if condition == "" {
		return fmt.Errorf("delete condition is empty, operation refused to prevent full table deletion")
	}
	self, err := m.self()
	if err != nil {
		return err
	}
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Where(condition, args...).Delete(self).Error
}

// Create 使用字段映射创建一条当前模型记录。
func (m *BaseModel) Create(data map[string]any) error {
	self, err := m.self()
	if err != nil {
		return err
	}
	db, err := m.GetDB(self)
	if err != nil {
		return err
	}
	return db.Create(data).Error
}

// CreateBatch 使用字段映射批量创建当前模型记录。
func (m *BaseModel) CreateBatch(data []map[string]any) error {
	self, err := m.self()
	if err != nil {
		return err
	}
	db, err := m.GetDB(self)
	if err != nil {
		return err
	}
	return db.Create(data).Error
}

// Save 保存当前模型实例，存在主键时会执行更新。
func (m *BaseModel) Save() error {
	self, err := m.self()
	if err != nil {
		return err
	}
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Save(self).Error
}
