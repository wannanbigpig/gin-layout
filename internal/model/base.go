package model

import (
	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"gorm.io/gorm"
	"gorm.io/plugin/soft_delete"
)

type BaseModel struct {
	ID        uint             `gorm:"column:id;type:int(11) unsigned AUTO_INCREMENT;not null;primarykey" json:"id"`
	CreatedAt utils.FormatDate `gorm:"column:created_at;type:timestamp;<-:create" json:"created_at"`
	UpdatedAt utils.FormatDate `gorm:"column:updated_at;type:timestamp" json:"updated_at"`
}

func (b *BaseModel) DB() *gorm.DB {
	return DB()
}

func (b *BaseModel) Paginate(page, pageSize int) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		offset := 0
		limit := global.PerPage
		if page < 1 {
			offset = page - 1
		}
		if pageSize > 0 {
			limit = pageSize
		}

		return db.Offset(offset * limit).Limit(limit)
	}
}

func (b *BaseModel) Count(model any, condition string, args []any) (count int64) {
	query := b.DB().Model(model)
	if condition != "" {
		query = query.Where(condition, args...)
	}
	err := query.Count(&count).Error
	if err != nil {
		return 0
	}
	return
}

type ContainsDeleteBaseModel struct {
	BaseModel
	DeletedAt soft_delete.DeletedAt `gorm:"column:deleted_at;type:int(11) unsigned;not null;default:0;index;" json:"-"`
}

func DB() *gorm.DB {
	return data.MysqlDB
}
