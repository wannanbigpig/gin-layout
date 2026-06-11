package csvutil

import "strings"

// SanitizeCell 防止 CSV 公式注入。
// 当单元格值以 =、+、-、@、Tab、CR 开头时，前加单引号 ' 使其在 Excel 中被视为文本。
func SanitizeCell(value string) string {
	if strings.HasPrefix(value, "\t") || strings.HasPrefix(value, "\r") {
		return "'" + value
	}
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if trimmed == "" {
		return value
	}
	switch trimmed[0] {
	case '=', '+', '-', '@':
		return "'" + value
	}
	return value
}
