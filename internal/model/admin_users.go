package model

import (
	"golang.org/x/crypto/bcrypt"
)

type AdminUsers struct {
	BaseModel
	MerchantId string `json:"merchant_id"`
	IsAdmin    string `json:"is_admin"`
	Nickname   string `json:"nickname"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	Mobile     string `json:"mobile"`
	Email      string `json:"email"`
	Avatar     string `json:"avatar"`
	Status     string `json:"status"`
	DeletedAt  string `json:"deleted_at"`
}

func AdminUsersModel() *AdminUsers {
	return &AdminUsers{}
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
	u.Password, _ = u.HashAndSalt(u.Password)
	result := u.DB().Create(u)
	return result.Error
}

// HashAndSalt 密码hash并自动加盐
func (u *AdminUsers) HashAndSalt(pwd string) (string, error) {
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
	u.Password, _ = u.HashAndSalt(u.Password)
	return u.DB().Model(u).Update("password", u.Password).Error
}

// GetUserInfo 根据名称获取用户信息
func (u *AdminUsers) GetUserInfo(username string) *AdminUsers {
	if err := u.DB().Where("username", username).First(u).Error; err != nil {
		return nil
	}
	return u
}
