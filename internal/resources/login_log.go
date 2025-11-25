package resources

import (
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

// AdminLoginLogBaseResources 管理员登录日志基础资源（公共字段）
type AdminLoginLogBaseResources struct {
	ID              uint              `json:"id"`
	UID             uint              `json:"uid"`               // 用户ID（登录失败时为0）
	Username        string            `json:"username"`          // 登录账号
	IP              string            `json:"ip"`                // 登录IP(支持IPv6)
	OS              string            `json:"os"`                // 操作系统
	Browser         string            `json:"browser"`           // 浏览器
	ExecutionTime   int               `json:"execution_time"`    // 登录耗时（毫秒）
	LoginStatus     uint8             `json:"login_status"`      // 登录状态：1=成功, 0=失败
	LoginStatusName string            `json:"login_status_name"` // 登录状态名称
	LoginFailReason string            `json:"login_fail_reason"` // 登录失败原因
	Type            uint8             `json:"type"`              // 操作类型：1=登录操作, 2=刷新token
	TypeName        string            `json:"type_name"`         // 操作类型名称
	IsRevoked       uint8             `json:"is_revoked"`        // 是否被撤销：0=否, 1=是
	IsRevokedName   string            `json:"is_revoked_name"`   // 是否被撤销名称
	RevokedCode     uint8             `json:"revoked_code"`      // 撤销原因码
	RevokedCodeName string            `json:"revoked_code_name"` // 撤销原因码名称
	RevokedReason   string            `json:"revoked_reason"`    // 撤销原因
	RevokedAt       *utils.FormatDate `json:"revoked_at"`        // 撤销时间
	CreatedAt       utils.FormatDate  `json:"created_at"`        // 创建时间
}

// AdminLoginLogListResources 管理员登录日志列表资源（简化版，不包含大字段）
type AdminLoginLogListResources struct {
	AdminLoginLogBaseResources
}

// AdminLoginLogResources 管理员登录日志详情资源
type AdminLoginLogResources struct {
	AdminLoginLogBaseResources
	JwtID          string            `json:"jwt_id"`          // JWT唯一标识(jti claim)
	UserAgent      string            `json:"user_agent"`      // 用户代理（浏览器/设备信息）
	TokenExpires   *utils.FormatDate `json:"token_expires"`   // Token过期时间
	RefreshExpires *utils.FormatDate `json:"refresh_expires"` // Refresh Token过期时间
	UpdatedAt      utils.FormatDate  `json:"updated_at"`      // 更新时间
	// 注意：不返回敏感信息 access_token、refresh_token、token_hash、refresh_token_hash
}

// AdminLoginLogTransformer 管理员登录日志资源转换器
type AdminLoginLogTransformer struct {
	BaseResources[*model.AdminLoginLogs, *AdminLoginLogResources]
}

// NewAdminLoginLogTransformer 实例化管理员登录日志资源转换器
func NewAdminLoginLogTransformer() AdminLoginLogTransformer {
	return AdminLoginLogTransformer{
		BaseResources: BaseResources[*model.AdminLoginLogs, *AdminLoginLogResources]{
			NewResource: func() *AdminLoginLogResources {
				return &AdminLoginLogResources{}
			},
		},
	}
}

// buildAdminLoginLogBaseResources 构建基础资源（公共字段）
func buildAdminLoginLogBaseResources(data *model.AdminLoginLogs) AdminLoginLogBaseResources {
	return AdminLoginLogBaseResources{
		ID:              data.ID,
		UID:             data.UID,
		Username:        data.Username,
		IP:              data.IP,
		OS:              data.OS,
		Browser:         data.Browser,
		ExecutionTime:   data.ExecutionTime,
		LoginStatus:     data.LoginStatus,
		LoginStatusName: data.LoginStatusMap(),
		LoginFailReason: data.LoginFailReason,
		Type:            data.Type,
		TypeName:        data.TypeMap(),
		IsRevoked:       data.IsRevoked,
		IsRevokedName:   data.IsRevokedMap(),
		RevokedCode:     data.RevokedCode,
		RevokedCodeName: data.RevokedCodeMap(),
		RevokedReason:   data.RevokedReason,
		RevokedAt:       data.RevokedAt,
		CreatedAt:       data.CreatedAt,
	}
}

// ToStruct 转换为单个资源（详情）
func (r AdminLoginLogTransformer) ToStruct(data *model.AdminLoginLogs) *AdminLoginLogResources {
	base := buildAdminLoginLogBaseResources(data)
	return &AdminLoginLogResources{
		AdminLoginLogBaseResources: base,
		JwtID:                      data.JwtID,
		UserAgent:                  data.UserAgent,
		TokenExpires:               data.TokenExpires,
		RefreshExpires:             data.RefreshExpires,
		UpdatedAt:                  data.UpdatedAt,
		// 注意：不返回敏感信息 access_token、refresh_token、token_hash、refresh_token_hash
	}
}

// ToCollection 转换为集合资源（列表，不包含大字段）
func (r AdminLoginLogTransformer) ToCollection(page, perPage int, total int64, data []*model.AdminLoginLogs) *Collection {
	response := make([]any, 0, len(data))
	for _, v := range data {
		base := buildAdminLoginLogBaseResources(v)
		response = append(response, &AdminLoginLogListResources{
			AdminLoginLogBaseResources: base,
		})
	}
	return NewCollection().SetPaginate(page, perPage, total).ToCollection(response)
}
