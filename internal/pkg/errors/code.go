package errors

const (
	SUCCESS          = 0
	FAILURE          = 1
	AuthorizationErr = 403
	NotFound         = 404
	CaptchaErr       = 400
	NotLogin         = 401
	ServerErr        = 500
	InvalidParameter = 10000
	UserDoesNotExist = 10001
	UserDisable      = 10002
	TooManyRequests  = 10102
)

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
