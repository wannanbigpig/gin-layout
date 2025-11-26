package crypto

// Algorithm 加密算法类型
type Algorithm string

const (
	// AlgorithmAES256GCM AES-256-GCM 加密算法（默认）
	AlgorithmAES256GCM Algorithm = "aes-256-gcm"
)

// String 返回算法名称
func (a Algorithm) String() string {
	return string(a)
}

// IsValid 检查算法是否有效
func (a Algorithm) IsValid() bool {
	switch a {
	case AlgorithmAES256GCM:
		return true
	default:
		return false
	}
}

