package errors

import (
	"fmt"
	"strings"
)

// 错误码常量定义按模块拆分至 code_common.go / code_auth.go / code_file.go /
// code_role.go / code_menu.go / code_dept.go / code_user.go 等文件中，
// 数值与名称保持稳定，外部引用大量存在，请勿变更。

// ErrorText 根据语言返回业务错误文案。
type ErrorText struct {
	Language string
}

// NewErrorText 创建错误文案解析器。
func NewErrorText(language string) *ErrorText {
	return &ErrorText{
		Language: language,
	}
}

// Text 按错误码和语言返回错误消息。
func (et *ErrorText) Text(code int) (str string) {
	var ok bool
	switch et.Language {
	case "zh_CN":
		str, ok = zhCNText[code]
	case "en":
		str, ok = enUSText[code]
	default:
		str, ok = zhCNText[code]
	}
	if !ok {
		return "unknown error"
	}
	return
}

// TextByKey 按语言和文案 key 返回错误消息。
func (et *ErrorText) TextByKey(key string, args ...any) (string, bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", false
	}

	var (
		template string
		ok       bool
	)
	switch et.Language {
	case "zh_CN":
		template, ok = zhCNTextKey[key]
	case "en":
		template, ok = enUSTextKey[key]
	default:
		template, ok = zhCNTextKey[key]
	}
	if !ok {
		return "", false
	}
	if len(args) == 0 {
		return template, true
	}
	return fmt.Sprintf(template, args...), true
}
