package error_code

import (
	"github.com/wannanbigpig/gin-layout/config"
)

const (
	SUCCESS            = 0
	FAILURE            = 1
	NotFound           = 404
	ParamBindError     = 10000
	ServerError        = 10101
	TooManyRequests    = 10102
	AuthorizationError = 10103
	RBACError          = 10104
)

func Text(code int) (str string) {
	lang := config.Config.Language

	var ok bool
	switch lang {
	case "zh_CN":
		str, ok = zhCNText[code]
		break
	case "en":
		str, ok = enUSText[code]
		break
	}
	if !ok {
		return "unknown error"
	}
	return
}
