package model

import (
	"github.com/wannanbigpig/gin-layout/data"
	"gorm.io/gorm"
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
