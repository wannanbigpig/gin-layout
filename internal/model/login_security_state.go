package model

import (
	"strings"

	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

// LoginSecurityState 记录登录失败计数与锁定状态。
type LoginSecurityState struct {
	BaseModel
	Username     string            `json:"username" gorm:"column:username;type:varchar(50);not null;default:'';uniqueIndex:lss_username;comment:登录账号"`
	FailCount    uint              `json:"fail_count" gorm:"column:fail_count;type:int unsigned;not null;default:0;comment:连续失败次数"`
	LockUntil    *utils.FormatDate `json:"lock_until" gorm:"column:lock_until;type:datetime;comment:锁定截止时间"`
	LastFailedAt *utils.FormatDate `json:"last_failed_at" gorm:"column:last_failed_at;type:datetime;comment:最近失败时间"`
}

func NewLoginSecurityState() *LoginSecurityState {
	return BindModel(&LoginSecurityState{})
}

func (m *LoginSecurityState) TableName() string {
	return "login_security_state"
}

// FindByUsername 查询指定账号的登录安全状态。
func (m *LoginSecurityState) FindByUsername(username string) error {
	username = strings.TrimSpace(username)
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Where("username = ?", username).First(m).Error
}
