package model

import (
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/plugin/soft_delete"
)

// AdminUsers 总管理员表
type AdminUsers struct {
	BaseModel
	IsAdmin     int8                  `gorm:"column:is_admin;type:tinyint(1);not null;default:0" json:"is_admin"`                                                                      // 是否是总管理员
	Nickname    string                `gorm:"column:nickname;type:varchar(60);not null;default:''" json:"nickname"`                                                                    // 用户昵称
	Username    string                `gorm:"uniqueIndex:a_u_username_unique;column:username;type:varchar(60);not null;default:''" json:"username"`                                    // 用户名
	Password    string                `gorm:"column:password;type:varchar(255);not null;default:''" json:"password"`                                                                   // 密码
	Mobile      string                `gorm:"uniqueIndex:a_u_mobile_unique;column:mobile;type:varchar(15);not null;default:''" json:"mobile"`                                          // 手机号
	Email       string                `gorm:"column:email;type:varchar(120);not null;default:''" json:"email"`                                                                         // 邮箱
	Avatar      string                `gorm:"column:avatar;type:varchar(160);not null;default:''" json:"avatar"`                                                                       // 头像
	Status      bool                  `gorm:"column:status;type:tinyint(1);not null;default:0" json:"status"`                                                                          // 状态,0正常，1禁用
	LoginAt     utils.FormatDate      `gorm:"column:login_at;type:timestamp;default:null" json:"login_at"`                                                                             // 登录时间
	LoginIP     string                `gorm:"column:login_ip;type:varchar(15);not null;default:''" json:"login_ip"`                                                                    // 最后一次登录ip
	LastLoginAt utils.FormatDate      `gorm:"column:last_login_at;type:timestamp;default:null" json:"last_login_at"`                                                                   // 上一次登录时间
	LastLoginIP string                `gorm:"column:last_login_ip;type:varchar(15);not null;default:''" json:"last_login_ip"`                                                          // 上一次登录ip
	DeletedAt   soft_delete.DeletedAt `gorm:"column:deleted_at;type:int(11) unsigned;not null;default:0;index;uniqueIndex:a_u_username_unique;uniqueIndex:a_u_mobile_unique" json:"-"` // 删除时间戳
}

func NewAdminUsers() *AdminUsers {
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
