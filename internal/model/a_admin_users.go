package model

import (
	"golang.org/x/crypto/bcrypt"
)

// AdminUsers 总管理员表
type AdminUsers struct {
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

func NewAdminUsers() *AdminUsers {
	return &AdminUsers{}
}

// TableName 获取表名
func (u *AdminUsers) TableName() string {
	return "a_admin_user"
}

// GetUserById 根据uid获取用户信息
func (u *AdminUsers) GetUserById(id uint) *AdminUsers {
	if err := u.DB().First(u, id).Error; err != nil {
		return nil
	}
	return u
}

// Register 用户注册，写入到DB
func (u *AdminUsers) Register() error {
	u.Password, _ = u.PasswordHash(u.Password)
	result := u.DB().Create(u)
	return result.Error
}

// PasswordHash 密码hash并自动加盐
func (u *AdminUsers) PasswordHash(pwd string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	return string(hash), err
}

// ComparePasswords 比对用户密码是否正确
func (u *AdminUsers) ComparePasswords(password string) bool {
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return false
	}
	return true
}

// ChangePassword 修改密码
func (u *AdminUsers) ChangePassword() error {
	u.Password, _ = u.PasswordHash(u.Password)
	return u.DB().Model(u).Update("password", u.Password).Error
}

// GetUserInfo 根据名称获取用户信息
func (u *AdminUsers) GetUserInfo(username string) *AdminUsers {
	if err := u.DB().Where("username", username).First(u).Error; err != nil {
		return nil
	}
	return u
}
