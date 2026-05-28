package auth

import "context"

// 会话控制 Close code，与前端约定保持一致。
const (
	// SessionControlCodeExpired 会话失效/被撤销（心跳反查或 token 过期触发），前端应停止重连并跳登录。
	SessionControlCodeExpired = 4001
	// SessionControlCodeKicked 被管理员强制下线或主动登出，前端应停止重连并跳登录。
	SessionControlCodeKicked = 4002
)

// SessionControl 描述一次会话强踢指令。
// SessionID 为空表示踢掉该用户全部在线连接；非空表示仅踢指定会话（JWT jti）。
type SessionControl struct {
	UserID    uint
	SessionID string
	Code      int
	Reason    string
}

// SessionControlPublisher 由 notification 包在启动时注入，用于把强踢指令派发到
// 本地 WebSocket 连接池及跨实例广播。auth 不直接依赖 notification，借此规避包循环依赖。
// 未注入（如纯 API 部署、单测）时为 nil，调用方需走 nil 安全封装。
var SessionControlPublisher func(context.Context, SessionControl)

// publishSessionControl 在 publisher 已注入时派发强踢指令，否则静默跳过。
func publishSessionControl(ctx context.Context, ctrl SessionControl) {
	publisher := SessionControlPublisher
	if publisher == nil || ctrl.UserID == 0 {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	publisher(ctx, ctrl)
}
