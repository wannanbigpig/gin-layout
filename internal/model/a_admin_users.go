package model

import (
	"golang.org/x/crypto/bcrypt"
)

// AdminUser 总管理员表
type AdminUser struct {
	ContainsDeleteBaseModel
	IsAdmin  int8   `json:"is_admin"` // 是否是总管理员
	Nickname string `json:"nickname"` // 用户昵称
	Username string `json:"username"` // 用户名
	Password string `json:"password"` // 密码
	Mobile   string `json:"mobile"`   // 手机号
	Email    string `json:"email"`    // 邮箱
	Avatar   string `json:"avatar"`   // 头像
	Status   int8   `json:"status"`   // 上一次登录ip
}

func NewAdminUsers() *AdminUser {
	return &AdminUser{}
}

// TableName 获取表名
func (m *AdminUser) TableName() string {
	return "a_admin_user"
}

// GetUserById 根据uid获取用户信息
func (m *AdminUser) GetUserById(id uint) *AdminUser {
	if err := m.DB().First(m, id).Error; err != nil {
		return nil
	}
	return m
}

// Register 用户注册，写入到DB
func (m *AdminUser) Register() error {
	m.Password, _ = m.PasswordHash(m.Password)
	result := m.DB().Create(m)
	return result.Error
}

// PasswordHash 密码hash并自动加盐
func (m *AdminUser) PasswordHash(pwd string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	return string(hash), err
}

// ComparePasswords 比对用户密码是否正确
func (m *AdminUser) ComparePasswords(password string) bool {
	if err := bcrypt.CompareHashAndPassword([]byte(m.Password), []byte(password)); err != nil {
		return false
	}
	return true
}

// ChangePassword 修改密码
func (m *AdminUser) ChangePassword() error {
	m.Password, _ = m.PasswordHash(m.Password)
	return m.DB(m).Update("password", m.Password).Error
}

// GetUserInfo 根据名称获取用户信息
func (m *AdminUser) GetUserInfo(username string) *AdminUser {
	if err := m.DB().Where("username", username).First(m).Error; err != nil {
		return nil
	}
	return m
}
