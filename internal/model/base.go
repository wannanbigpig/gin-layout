package model

import (
	"gorm.io/gorm"
	"l-admin.com/data"
)

type BaseModel struct {
	ID        uint `json:"id"`
	CreatedAt uint `json:"created_at"`
	UpdatedAt uint `json:"updated_at"`
}

func (model BaseModel) DB() *gorm.DB {
	return DB()
}

func DB() *gorm.DB {
	return data.DB
}
