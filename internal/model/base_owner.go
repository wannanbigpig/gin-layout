package model

import "gorm.io/gorm"

type ownerBinder interface {
	bindOwner(any)
}

// SetDB 为当前模型绑定事务或指定数据库连接。
func (m *BaseModel) SetDB(tx *gorm.DB) *BaseModel {
	m.dbInstance = tx
	return m
}

func (m *BaseModel) bindOwner(owner any) {
	m.owner = owner
}

// BindModel 为嵌入 BaseModel 的模型绑定自身实例，供通用方法回写使用。
func BindModel[T any](m T) T {
	if binder, ok := any(m).(ownerBinder); ok {
		binder.bindOwner(m)
	}
	return m
}

func (m *BaseModel) self() (any, error) {
	if m.owner == nil {
		return nil, ErrModelPtrNotImplemented
	}
	return m.owner, nil
}

// GetDB 返回当前模型可用的数据库实例，传入 model 时会附带 Model 上下文。
func (m *BaseModel) GetDB(model ...any) (*gorm.DB, error) {
	if m.dbInstance != nil {
		if len(model) > 0 {
			if err := validateModelArg(model[0]); err != nil {
				return nil, err
			}
			return m.dbInstance.Model(model[0]), nil
		}
		return m.dbInstance, nil
	}
	return GetDB(model...)
}
