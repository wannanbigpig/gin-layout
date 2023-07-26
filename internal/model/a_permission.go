package model

import (
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"gorm.io/gorm/clause"
)

// Permission 权限路由表
type Permission struct {
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

func NewPermission() *Permission {
	return &Permission{}
}

// TableName 获取表名
func (a *Permission) TableName() string {
	return "a_permission"
}

// Registers 注册接口，写入到DB
func (a *Permission) Registers(data []map[string]any) error {
	return a.DB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "route"}, {Name: "deleted_at"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "route", "method", "func", "func_path", "updated_at"}),
	}).Model(a).Create(data).Error
}

// Update 更新权限
func (a *Permission) Update(id uint, data map[string]any) error {
	return a.DB().Model(a).Where("id = ?", id).UpdateColumns(data).Error
}

// Create 更新权限
func (a *Permission) Create(data map[string]any) error {
	return a.DB().Model(a).Create(data).Error
}

// HasRoute 判断路由是否存在
func (a *Permission) HasRoute(route string) (count int64, err error) {
	err = a.DB().Model(a).Where("route = ?", route).Count(&count).Error
	return
}

// ListPage 分页
func (a *Permission) ListPage(page, perPage int, condition string, args []any) *resources.PermissionCollection {
	res := resources.NewPermissionCollection()
	res.Total = a.Count(a, condition, args)
	if res.Total == 0 {
		return res
	}
	query := a.DB().Model(a).Scopes(a.Paginate(page, perPage))
	if condition != "" {
		query = query.Where(condition, args...)
	}
	err := query.Order("sort,id desc").Find(&res.Data).Error
	if err != nil {
		return nil
	}
	return res
}
