package testkit

import "strings"

// SecretKey 返回测试专用密钥，统一管理避免散落硬编码。
func SecretKey(scope string) string {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		scope = "default"
	}
	// 保持长度充足以满足生产级最小长度校验场景。
	return "unit-test-secret-key-" + scope + "-0123456789abcdef"
}
