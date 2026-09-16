package service

import "gorm.io/gorm"

// CreateWithExplicitFields 保留创建请求中可能被 GORM default 标签覆盖的零值。
// 两次写入在同一事务内完成，Updates 同时恢复 value 中的显式字段。
func CreateWithExplicitFields(db *gorm.DB, value any, fields map[string]any) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(value).Error; err != nil {
			return err
		}
		return tx.Model(value).Updates(fields).Error
	})
}
