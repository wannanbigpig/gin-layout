package error_code

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

type ErrorText struct {
	Language string
}

func (et *ErrorText) Text(code int) (str string) {
	var ok bool
	switch et.Language {
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
