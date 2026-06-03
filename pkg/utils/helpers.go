package utils

import (
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// passwordHashCost 为 bcrypt 工作因子，下限 12（密码哈希强度安全底线，勿低于此值）。
const passwordHashCost = 12

// MaskSensitiveInfo 对于字符串脱敏
// s 需要脱敏的字符串
// start 从第几位开始脱敏
// maskNumber 需要脱敏长度
// maskChars 掩饰字符串，替代需要脱敏处理的字符串
func MaskSensitiveInfo(s string, start int, maskNumber int, maskChars ...string) string {
	// 将字符串 s 的 [start, end) 区间用 maskChar 替换。按 rune 处理，避免中文/emoji 等多字节字符被截断。
	maskChar := "*"
	if len(maskChars) > 0 && maskChars[0] != "" {
		maskChar = maskChars[0]
	}
	if maskNumber <= 0 || len(s) == 0 {
		return s
	}
	runes := []rune(s)
	length := len(runes)
	// 处理起始位置超出边界的情况
	if start < 0 {
		start = 0
	}
	if start > length {
		start = length
	}
	// 处理结束位置超出边界的情况
	end := start + maskNumber
	if end > length {
		end = length
	}
	return string(runes[:start]) + strings.Repeat(maskChar, end-start) + string(runes[end:])
}

// PasswordHash 密码hash并自动加盐
func PasswordHash(pwd string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(pwd), passwordHashCost)
	return string(hash), err
}

// ComparePasswords 比对用户密码是否正确
func ComparePasswords(oldPassword, password string) bool {
	if err := bcrypt.CompareHashAndPassword([]byte(oldPassword), []byte(password)); err != nil {
		return false
	}
	return true
}

// SliceToAny 将切片转换为any类型切片
func SliceToAny[T any](data []T) []any {
	anyData := make([]any, len(data))
	for i, v := range data {
		anyData[i] = v
	}
	return anyData
}
