package model

import (
	"fmt"

	"gorm.io/gorm/clause"

	"github.com/wannanbigpig/gin-layout/internal/model/modelDict"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

// 管理员状态常量
const (
	AdminUserStatusEnabled  uint8 = 1 // 启用
	AdminUserStatusDisabled uint8 = 0 // 禁用（数据库定义：1启用 0禁用）
)

// 管理员状态字典
var AdminUserStatusDict modelDict.Dict = map[uint8]string{
	AdminUserStatusEnabled:  "启用",
	AdminUserStatusDisabled: "禁用",
}

var adminUserUniqueFieldAllowList = map[string]struct{}{
	"username":          {},
	"full_phone_number": {},
	"email":             {},
}

// AdminUser 总管理员表
type AdminUser struct {
	ContainsDeleteBaseModel
	IsSuperAdmin    uint8              `json:"is_super_admin"`    // 是否是总管理员
	Nickname        string             `json:"nickname"`          // 用户昵称
	Username        string             `json:"username"`          // 用户名
	Password        string             `json:"password"`          // 密码
	PhoneNumber     string             `json:"phone_number"`      // 手机号
	FullPhoneNumber string             `json:"full_phone_number"` // 完整手机号
	CountryCode     string             `json:"country_code"`      // 国际区号
	Email           string             `json:"email"`             // 邮箱
	Avatar          string             `json:"avatar"`            // 头像
	Status          uint8              `json:"status"`            // 状态 1启用 0禁用
	LastLogin       utils.FormatDate   `json:"last_login"`        // 最后登录时间
	LastIp          string             `json:"last_ip"`           // 最后登录IP
	Department      []Department       `json:"department" gorm:"many2many:admin_user_department_map;foreignKey:ID;joinForeignKey:Uid;References:ID;joinReferences:DeptId"`
	RoleList        []AdminUserRoleMap `json:"role_list" gorm:"foreignKey:uid;references:id"`
}

func NewAdminUsers() *AdminUser {
	return BindModel(&AdminUser{})
}

// TableName 获取表名
func (m *AdminUser) TableName() string {
	return "admin_user"
}

// GetUserInfo 根据名称获取用户信息
func (m *AdminUser) GetUserInfo(username string) error {
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	if err := db.Where("username", username).First(m).Error; err != nil {
		return err
	}
	return nil
}

// IsSuperAdminMap 是否为超级管理员映射
func (m *AdminUser) IsSuperAdminMap() string {
	return modelDict.IsMap.Map(m.IsSuperAdmin)
}

// StatusMap 状态映射
func (m *AdminUser) StatusMap() string {
	return AdminUserStatusDict.Map(m.Status)
}

// SyncUserRow 权限同步时需要的用户简要信息。
type SyncUserRow struct {
	ID           uint
	Status       uint8
	IsSuperAdmin uint8
}

// SyncUserRows 根据用户 ID 列表查询未删除用户的同步信息（id, status, is_super_admin）。
func (m *AdminUser) SyncUserRows(userIDs []uint) ([]SyncUserRow, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	db, err := m.GetDB()
	if err != nil {
		return nil, err
	}
	var rows []SyncUserRow
	if err := db.Table(m.TableName()).
		Select("id,status,is_super_admin").
		Where("id IN ? AND deleted_at = 0", userIDs).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// AllIds 查询所有未删除用户的 ID 列表。
func (m *AdminUser) AllIds() ([]uint, error) {
	db, err := m.GetDB(m)
	if err != nil {
		return nil, err
	}
	var ids []uint
	if err := db.Where("deleted_at = 0").Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// ExistsWithLock 带行锁检查指定条件的记录是否存在。
func (m *AdminUser) ExistsWithLock(condition string, args ...any) (bool, error) {
	db, err := m.GetDB()
	if err != nil {
		return false, err
	}
	var exists bool
	if err := db.Model(m).
		Select("1").
		Where(condition, args...).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Limit(1).
		Scan(&exists).Error; err != nil {
		return false, err
	}
	return exists, nil
}

// ExistsWithLockExcludeId 带行锁检查指定条件的记录是否存在（排除指定 ID）。
func (m *AdminUser) ExistsWithLockExcludeId(field string, value string, excludeId uint) (bool, error) {
	if _, ok := adminUserUniqueFieldAllowList[field]; !ok {
		return false, fmt.Errorf("field is not allowed for unique check: %s", field)
	}

	db, err := m.GetDB()
	if err != nil {
		return false, err
	}
	var exists bool
	if err := db.Model(m).
		Select("1").
		Where(clause.Eq{Column: clause.Column{Name: field}, Value: value}).
		Where("id != ? AND deleted_at = 0", excludeId).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Limit(1).
		Scan(&exists).Error; err != nil {
		return false, err
	}
	return exists, nil
}

// GetByIdWithPreload 根据 ID 获取用户并预加载指定关联。
func (m *AdminUser) GetByIdWithPreload(id uint, relations ...string) error {
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	for _, relation := range relations {
		db = db.Preload(relation)
	}
	return db.First(m, id).Error
}
