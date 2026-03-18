package utils

import (
	"strings"
	"unicode/utf8"
)

// DesensitizeRule 描述字符串脱敏策略。
type DesensitizeRule struct {
	KeepPrefixLen   int  // 保留前缀长度
	KeepSuffixLen   int  // 保留后缀长度
	MaskChar        rune // 脱敏字符
	Separator       rune // 特殊分隔符(如邮箱的@)
	FixedMaskLength int  // 固定脱敏长度(0表示不固定)
}

// NewPhoneRule 构建手机号码脱敏规则
func NewPhoneRule() *DesensitizeRule {
	return &DesensitizeRule{KeepPrefixLen: 3, KeepSuffixLen: 4, MaskChar: '*', FixedMaskLength: 4}
}

// NewEmailRule 构建邮箱脱敏规则
func NewEmailRule() *DesensitizeRule {
	return &DesensitizeRule{KeepPrefixLen: 2, KeepSuffixLen: 0, MaskChar: '*', Separator: '@', FixedMaskLength: 3}
}

// Apply 按当前规则对输入字符串做脱敏。
func (r *DesensitizeRule) Apply(s string) string {
	if utf8.RuneCountInString(s) == 0 {
		return s
	}

	// 处理带分隔符的情况(如邮箱)
	if r.Separator != 0 {
		parts := strings.Split(s, string(r.Separator))
		if len(parts) == 2 {
			localPart := r.applyToPart(parts[0])
			return localPart + string(r.Separator) + parts[1]
		}
	}

	return r.applyToPart(s)
}

func (r *DesensitizeRule) applyToPart(s string) string {
	runes := []rune(s)
	length := len(runes)

	// 计算需要保留的前后部分
	keepPrefix := r.min(r.KeepPrefixLen, length)
	keepSuffix := r.min(r.KeepSuffixLen, length-keepPrefix)

	// 计算脱敏部分长度
	var maskLength int
	if r.FixedMaskLength > 0 {
		maskLength = r.FixedMaskLength // 使用固定长度
	} else {
		maskLength = length - keepPrefix - keepSuffix // 使用可变长度
	}

	// 构建结果
	var result strings.Builder
	if keepPrefix > 0 {
		result.WriteString(string(runes[:keepPrefix]))
	}
	if maskLength > 0 {
		result.WriteString(strings.Repeat(string(r.MaskChar), maskLength))
	}
	if keepSuffix > 0 {
		result.WriteString(string(runes[length-keepSuffix:]))
	}

	return result.String()
}

func (r *DesensitizeRule) min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
