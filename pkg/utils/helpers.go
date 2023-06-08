package utils

import "strings"

// MaskSensitiveInfo 对于字符串脱敏
// s 需要脱敏的字符串
// start 从第几位开始脱敏
// maskNumber 需要脱敏长度
// maskChars 掩饰字符串，替代需要脱敏处理的字符串
func MaskSensitiveInfo(s string, start int, maskNumber int, maskChars ...string) string {
	// 将字符串s的[start, end)区间用maskChar替换，并返回替换后的结果。
	maskChar := "*"
	if maskChars != nil {
		maskChar = maskChars[0]
	}
	// 处理起始位置超出边界的情况
	if start < 0 {
		start = 0
	}
	// 处理结束位置超出边界的情况
	end := start + maskNumber
	if end > len(s) {
		end = len(s)
	}
	return s[:start] + strings.Repeat(maskChar, end-start) + s[end:]
}
