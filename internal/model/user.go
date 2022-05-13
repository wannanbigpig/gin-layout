package model

import (
	"golang.org/x/crypto/bcrypt"
	"log"
)

type User struct {
	BaseModel
	Username   string `json:"username"`
	Password   string `json:"password"`
	Email      string `json:"email"`
	Nickname   string `json:"nickname"`
	TotalSpace uint64 `json:"total_space"` // 为用户分配的最大存储空间， 若为0则代表不限制
	UsedSpace  uint64 `json:"used_space"`  // 已上传的空间占用
}

// GetUserById 根据uid获取用户信息
func (u *User) GetUserById(id uint) *User {
	err := u.DB().Where(id).First(u).Error
	if err != nil {
		return nil
	}
	return u
}

// Register 用户注册，写入到DB
func (u *User) Register() error {
	u.Password, _ = u.HashAndSalt(u.Password)
	result := u.DB().Create(u)
	return result.Error
}

// HashAndSalt 密码hash并自动加盐
func (u *User) HashAndSalt(pwd string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	return string(hash), err
}

// ComparePasswords 比对用户密码是否正确
func (u *User) ComparePasswords(password string) bool {
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		log.Println(err)
		return false
	}
	return true
}

func (u *User) UpdateUsedSpace(size uint64) error {
	u.UsedSpace += size
	return u.DB().Model(u).Update("used_space", u.UsedSpace).Error
}

func (u *User) ChangePassword() error {
	u.Password, _ = u.HashAndSalt(u.Password)
	return u.DB().Model(u).Update("password", u.Password).Error
}
