package access

import "gorm.io/gorm"

// RunInTransaction 统一执行事务。
func RunInTransaction(db *gorm.DB, fn func(tx *gorm.DB) error) error {
	return db.Transaction(fn)
}
