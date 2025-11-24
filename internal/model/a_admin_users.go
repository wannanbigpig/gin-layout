package model

import (
	"github.com/wannanbigpig/gin-layout/internal/model/modelDict"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	utils2 "github.com/wannanbigpig/gin-layout/pkg/utils"
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
	Status          int8               `json:"status"`            // 状态 1启用 2禁用
	LastLogin       utils.FormatDate   `json:"last_login"`        // 最后登录时间
	LastIp          string             `json:"last_ip"`           // 最后登录IP
	Department      []Department       `json:"department" gorm:"many2many:a_admin_user_department_map;foreignKey:ID;joinForeignKey:Uid;References:ID;joinReferences:DeptId"`
	RoleList        []AdminUserRoleMap `json:"role_list" gorm:"foreignKey:uid;references:id"`
}

func NewAdminUsers() *AdminUser {
	return &AdminUser{}
}

// TableName 获取表名
func (m *AdminUser) TableName() string {
	return "a_admin_user"
}

// Register 用户注册，写入到DB
func (m *AdminUser) Register() error {
	m.Password, _ = utils2.PasswordHash(m.Password)
	result := m.DB().Create(m)
	return result.Error
}

// ChangePassword 修改密码
func (m *AdminUser) ChangePassword() error {
	m.Password, _ = utils2.PasswordHash(m.Password)
	return m.DB(m).Update("password", m.Password).Error
}

// GetUserInfo 根据名称获取用户信息
func (m *AdminUser) GetUserInfo(username string) error {
	if err := m.DB().Where("username", username).First(m).Error; err != nil {
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
	return AdminUserStatusDict.Map(uint8(m.Status))
}
