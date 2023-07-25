package model

import "gorm.io/gorm/clause"

// Api 权限路由表
type Api struct {
	ContainsDeleteBaseModel
	Name     string `json:"name"`      // 权限名称
	Desc     string `json:"desc"`      // 描述
	Method   string `json:"method"`    // 接口请求方法
	Route    string `json:"route"`     // 接口路由
	Func     string `json:"func"`      // 接口方法
	FuncPath string `json:"func_path"` // 接口方法
	IsAuth   int8   `json:"is_auth"`   // 接口方法
	Sort     int32  `json:"sort"`      // 排序
}

func NewPermission() *Api {
	return &Api{}
}

// TableName 获取表名
func (a *Api) TableName() string {
	return "a_permission"
}

// Registers 注册接口，写入到DB
func (a *Api) Registers(data []map[string]any) error {
	return a.DB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "route"}, {Name: "deleted_at"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "route", "method", "func", "func_path", "updated_at"}),
	}).Model(a).Create(data).Error
}

// Update 更新权限
func (a *Api) Update(id uint, data map[string]any) error {
	return a.DB().Model(a).Where("id = ?", id).UpdateColumns(data).Error
}

// Create 更新权限
func (a *Api) Create(data map[string]any) error {
	return a.DB().Model(a).Create(data).Error
}

// HasRoute 判断路由是否存在
func (a *Api) HasRoute(route string) (count int64, err error) {
	err = a.DB().Model(a).Where("route = ?", route).Count(&count).Error
	return
}
