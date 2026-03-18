package access

import "gorm.io/gorm"

// runInTransaction 统一执行事务。
func runInTransaction(db *gorm.DB, fn func(tx *gorm.DB) error) error {
	return db.Transaction(fn)
}
