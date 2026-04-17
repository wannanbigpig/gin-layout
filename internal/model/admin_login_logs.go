package model

import (
	"time"

	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model/modelDict"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

// 登录操作类型常量
const (
	LoginTypeLogin   uint8 = 1 // 登录操作
	LoginTypeRefresh uint8 = 2 // 刷新token
)

// 登录状态常量
const (
	LoginStatusSuccess uint8 = 1 // 登录成功
	LoginStatusFail    uint8 = 0 // 登录失败
)

// 登录状态字典
var LoginStatusDict modelDict.Dict = map[uint8]string{
	LoginStatusFail:    "失败",
	LoginStatusSuccess: "成功",
}

// 登录操作类型字典
var LoginTypeDict modelDict.Dict = map[uint8]string{
	LoginTypeLogin:   "登录操作",
	LoginTypeRefresh: "刷新token",
}

// 是否被撤销常量（使用 global.Yes/No，这里定义别名以便使用）
const (
	IsRevokedNo  = global.No  // 否
	IsRevokedYes = global.Yes // 是
)

// 撤销原因码常量
const (
	RevokedCodeUserLogout          uint8 = 1 // 用户主动登出（退出登录）
	RevokedCodeSystemForce         uint8 = 2 // 系统强制登出（账号被封）
	RevokedCodeTokenRefresh        uint8 = 3 // 系统刷新token
	RevokedCodeUserDisable         uint8 = 4 // 用户禁用（针对某个设备下线操作）
	RevokedCodeOther               uint8 = 5 // 其他原因
	RevokedCodePasswordChangeSelf  uint8 = 6 // 用户自己修改密码
	RevokedCodePasswordChangeAdmin uint8 = 7 // 管理员修改密码
)

// RevokedCodeDict 撤销原因码字典
var RevokedCodeDict modelDict.Dict = map[uint8]string{
	RevokedCodeUserLogout:          "用户主动登出（退出登录）",
	RevokedCodeSystemForce:         "系统强制登出（账号被封）",
	RevokedCodeTokenRefresh:        "系统刷新token",
	RevokedCodeUserDisable:         "用户禁用（针对某个设备下线操作）",
	RevokedCodeOther:               "其他原因",
	RevokedCodePasswordChangeSelf:  "用户自己修改密码",
	RevokedCodePasswordChangeAdmin: "管理员修改密码",
}

// AdminLoginLogs 登录日志表
type AdminLoginLogs struct {
	ContainsDeleteBaseModel
	UID              uint              `json:"uid"`                // 用户ID（登录失败时为0）
	Username         string            `json:"username"`           // 登录账号
	JwtID            string            `json:"jwt_id"`             // JWT唯一标识(jti claim)
	AccessToken      string            `json:"access_token"`       // 访问令牌
	RefreshToken     string            `json:"refresh_token"`      // 刷新令牌
	TokenHash        string            `json:"token_hash"`         // Token的SHA256哈希值
	RefreshTokenHash string            `json:"refresh_token_hash"` // Refresh Token的哈希值
	IP               string            `json:"ip"`                 // 登录IP(支持IPv6)
	UserAgent        string            `json:"user_agent"`         // 用户代理（浏览器/设备信息）
	OS               string            `json:"os"`                 // 操作系统
	Browser          string            `json:"browser"`            // 浏览器
	ExecutionTime    int               `json:"execution_time"`     // 登录耗时（毫秒）
	LoginStatus      uint8             `json:"login_status"`       // 登录状态：1=成功, 0=失败
	LoginFailReason  string            `json:"login_fail_reason"`  // 登录失败原因
	Type             uint8             `json:"type"`               // 操作类型：1=登录操作, 2=刷新token
	IsRevoked        uint8             `json:"is_revoked"`         // 是否被撤销：0=否, 1=是
	RevokedCode      uint8             `json:"revoked_code"`       // 撤销原因码：1=用户主动登出（退出登录）, 2=系统强制登出（账号被封）, 3=系统刷新token, 4=用户禁用（针对某个设备下线操作） 5=其他原因
	RevokedReason    string            `json:"revoked_reason"`     // 撤销原因
	RevokedAt        *utils.FormatDate `json:"revoked_at"`         // 撤销时间
	TokenExpires     *utils.FormatDate `json:"token_expires"`      // Token过期时间
	RefreshExpires   *utils.FormatDate `json:"refresh_expires"`    // Refresh Token过期时间
}

func NewAdminLoginLogs() *AdminLoginLogs {
	return BindModel(&AdminLoginLogs{})
}

// TableName 获取表名
func (m *AdminLoginLogs) TableName() string {
	return "admin_login_logs"
}

// LoginStatusMap 登录状态映射
func (m *AdminLoginLogs) LoginStatusMap() string {
	return LoginStatusDict.Map(m.LoginStatus)
}

// TypeMap 操作类型映射
func (m *AdminLoginLogs) TypeMap() string {
	return LoginTypeDict.Map(m.Type)
}

// IsRevokedMap 是否被撤销映射
func (m *AdminLoginLogs) IsRevokedMap() string {
	return modelDict.IsMap.Map(m.IsRevoked)
}

// RevokedCodeMap 撤销原因码映射
func (m *AdminLoginLogs) RevokedCodeMap() string {
	return RevokedCodeDict.Map(m.RevokedCode)
}

// Create 创建单条登录日志记录。
func (m *AdminLoginLogs) Create() error {
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Create(m).Error
}

// FindByJwtId 根据 jwtId 查找登录日志。
func (m *AdminLoginLogs) FindByJwtId(jwtId string) error {
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Where("jwt_id = ? AND deleted_at = 0", jwtId).First(m).Error
}

// UpdateRevokedStatusByJwtIds 批量更新 token 撤销状态。
func (m *AdminLoginLogs) UpdateRevokedStatusByJwtIds(jwtIds []string, revokedCode uint8, revokedReason string, revokedAt utils.FormatDate) error {
	if len(jwtIds) == 0 {
		return nil
	}
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Where("jwt_id IN ? AND deleted_at = 0 AND is_revoked = ?", jwtIds, IsRevokedNo).
		Updates(map[string]interface{}{
			"is_revoked":     IsRevokedYes,
			"revoked_code":   revokedCode,
			"revoked_reason": revokedReason,
			"revoked_at":     revokedAt,
		}).Error
}

// FindActiveTokensByUserId 查询用户未过期的活跃 token 列表。
func (m *AdminLoginLogs) FindActiveTokensByUserId(userId uint, now time.Time) ([]AdminLoginLogs, error) {
	db, err := m.GetDB()
	if err != nil {
		return nil, err
	}
	var loginLogs []AdminLoginLogs
	err = db.Where("uid = ? AND deleted_at = 0 AND is_revoked = ? AND login_status = ? AND token_expires IS NOT NULL AND token_expires > ?",
		userId, IsRevokedNo, LoginStatusSuccess, now).Find(&loginLogs).Error
	return loginLogs, err
}
