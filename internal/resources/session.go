package resources

import (
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

// SessionResources 在线会话响应结构。
type SessionResources struct {
	ID            uint              `json:"id"`
	UID           uint              `json:"uid"`
	Username      string            `json:"username"`
	JwtID         string            `json:"jwt_id"`
	IP            string            `json:"ip"`
	OS            string            `json:"os"`
	Browser       string            `json:"browser"`
	IsRevoked     uint8             `json:"is_revoked"`
	RevokedReason string            `json:"revoked_reason"`
	RevokedAt     *utils.FormatDate `json:"revoked_at"`
	TokenExpires  *utils.FormatDate `json:"token_expires"`
	CreatedAt     utils.FormatDate  `json:"created_at"`
}

// SessionTransformer 在线会话转换器。
type SessionTransformer struct {
	BaseResources[*model.AdminLoginLogs, *SessionResources]
}

func NewSessionTransformer() SessionTransformer {
	return SessionTransformer{
		BaseResources: BaseResources[*model.AdminLoginLogs, *SessionResources]{
			NewResource: func() *SessionResources {
				return &SessionResources{}
			},
		},
	}
}
