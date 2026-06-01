// Package corsutil 提供唯一的 Origin 白名单匹配实现，供 HTTP CORS 中间件与
// WebSocket 握手共用，避免两处各写一套通配规则导致语义漂移与安全缺口。
package corsutil

import (
	"path"
	"strings"
)

// IsOriginAllowed 判断 origin 是否命中白名单。
//
// 匹配规则（与历史 HTTP CORS 行为保持一致）：
//   - 精确相等命中；
//   - 含通配符的条目用 path.Match 按整串匹配（如 "https://*.example.com"）。
//
// allowWildcardAll 控制字面 "*" 是否表示放行任意 Origin：
//   - HTTP CORS 在无凭证场景下允许 "*"（公共接口），传 true；
//   - WebSocket 握手始终携带身份（ticket/token），放行任意 Origin 即 CSWSH 风险，
//     必须传 false，使字面 "*" 不开 WS 的 Origin 闸门。
func IsOriginAllowed(origin string, allowedOrigins []string, allowWildcardAll bool) bool {
	if origin == "" || len(allowedOrigins) == 0 {
		return false
	}
	for _, allowed := range allowedOrigins {
		if allowed == "*" {
			if allowWildcardAll {
				return true
			}
			continue
		}
		if origin == allowed {
			return true
		}
		if strings.Contains(allowed, "*") {
			if matched, _ := path.Match(allowed, origin); matched {
				return true
			}
		}
	}
	return false
}
