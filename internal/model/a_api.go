package model

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Api 权限路由表
type Api struct {
	ContainsDeleteBaseModel
	Code        string `json:"code"`         // 权限唯一code
	GroupCode   string `json:"group_code"`   // 分组code
	Name        string `json:"name"`         // 权限名称
	Desc        string `json:"desc"`         // 描述
	Method      string `json:"method"`       // 接口请求方法
	Route       string `json:"route"`        // 接口路由
	Func        string `json:"func"`         // 接口方法
	FuncPath    string `json:"func_path"`    // 接口方法路径
	IsAuth      int8   `json:"is_auth"`      // 是否鉴权 0:否 1:是
	IsEffective int8   `json:"is_effective"` // 是否有效 0:否 1:是
	Sort        int    `json:"sort"`         // 排序，数字越大优先级越高
}

func NewApi() *Api {
	return &Api{}
}

// TableName 获取表名
func (m *Api) TableName() string {
	return "a_api"
}

// InitRegisters 注册接口，写入到DB
func (m *Api) InitRegisters(data []map[string]any, date string) error {
	return m.DB(m).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "code"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "desc", "is_auth", "func", "group_code", "func_path", "is_effective", "updated_at"}),
		}).Create(data).Error
		if err != nil {
			return err
		}
		return tx.Model(m).Where("updated_at != ?", date).Update("is_effective", 0).Error
	})
}
