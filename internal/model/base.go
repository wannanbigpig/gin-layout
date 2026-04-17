package model

import (
	"errors"
	"fmt"
	"reflect"

	"gorm.io/gorm"
	"gorm.io/plugin/soft_delete"

	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

// BaseModel 提供模型通用字段与基础 CRUD 能力。
type BaseModel struct {
	dbInstance *gorm.DB
	owner      any
	ID         uint             `gorm:"column:id;type:int(11) unsigned AUTO_INCREMENT;not null;primarykey" json:"id"`
	CreatedAt  utils.FormatDate `gorm:"column:created_at;type:datetime;<-:create" json:"created_at"`
	UpdatedAt  utils.FormatDate `gorm:"column:updated_at;type:datetime" json:"updated_at"`
}

// ErrDBUninitialized 表示数据库连接尚未初始化。
var ErrDBUninitialized = errors.New("database connection is not initialized")

// ErrModelPtrNotImplemented 表示模型尚未完成 owner 绑定。
var ErrModelPtrNotImplemented = errors.New("model owner binding is not initialized")

// ErrInvalidModelArg 表示传入 GetDB 的模型参数无效（如 typed nil 指针）。
var ErrInvalidModelArg = errors.New("invalid model argument")

// ContainsDeleteBaseModel 在 BaseModel 基础上增加软删除字段。
type ContainsDeleteBaseModel struct {
	BaseModel
	DeletedAt soft_delete.DeletedAt `gorm:"column:deleted_at;type:int(11) unsigned;not null;default:0;index;" json:"-"`
}

// GetDB 返回全局数据库实例，传入 model 时会附带 Model 上下文。
func GetDB(model ...any) (*gorm.DB, error) {
	if len(model) > 0 {
		if err := validateModelArg(model[0]); err != nil {
			return nil, err
		}
	}

	db := data.MysqlDB()
	if db == nil {
		if initErr := data.MysqlInitError(); initErr != nil {
			return nil, fmt.Errorf("%w: %v", ErrDBUninitialized, initErr)
		}
		return nil, ErrDBUninitialized
	}
	if len(model) > 0 && model[0] != nil {
		return db.Model(model[0]), nil
	}
	return db, nil
}

func validateModelArg(model any) error {
	if model == nil {
		return nil
	}
	value := reflect.ValueOf(model)
	switch value.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Interface, reflect.Func, reflect.Chan:
		if value.IsNil() {
			return ErrInvalidModelArg
		}
	}
	return nil
}
