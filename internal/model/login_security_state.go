package model

import (
	"strings"
	"time"

	"gorm.io/gorm"

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

// IncrementFailCount 原子递增失败计数，返回更新后的记录。
// 使用 gorm.Expr("fail_count + 1") 实现数据库层面原子递增，避免并发丢失更新。
// 当达到 maxFailures 阈值时自动设置 lock_until。
// 当记录不存在时创建新记录。
func (m *LoginSecurityState) IncrementFailCount(username string, maxFailures int, lockMinutes int) error {
	db, err := m.GetDB()
	if err != nil {
		return err
	}

	now := time.Now()
	lockUntil := now.Add(time.Duration(lockMinutes) * time.Minute)

	// 先尝试原子更新已有记录
	result := db.Model(&LoginSecurityState{}).
		Where("username = ?", username).
		Updates(map[string]any{
			"fail_count":     gorm.Expr("fail_count + 1"),
			"last_failed_at": now,
			"lock_until": gorm.Expr(
				"CASE WHEN fail_count + 1 >= ? THEN ? ELSE lock_until END",
				maxFailures, lockUntil,
			),
		})

	if result.Error != nil {
		return result.Error
	}

	// 如果没有更新到记录（不存在），则创建
	if result.RowsAffected == 0 {
		lockUntilVal := (*utils.FormatDate)(nil)
		if 1 >= maxFailures {
			lockUntilVal = &utils.FormatDate{Time: lockUntil}
		}
		return db.Create(&LoginSecurityState{
			Username:     username,
			FailCount:    1,
			LastFailedAt: &utils.FormatDate{Time: now},
			LockUntil:    lockUntilVal,
		}).Error
	}

	return nil
}
