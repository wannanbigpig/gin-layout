package service

import (
	"gorm.io/gorm"
)

type Base struct {
	aUid *uint
}

func Paginate(page, pageSize int) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		offset := 0
		if page > 0 {
			offset = page - 1
		}
		if pageSize < 1 {
			pageSize = 10
		}

		return db.Offset(offset * pageSize).Limit(pageSize)
	}
}
