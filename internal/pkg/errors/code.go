package errors

const (
	SUCCESS            = 0
	FAILURE            = 1
	AuthorizationError = 403
	NotFound           = 404
	NotLogin           = 401
	InvalidParameter   = 10000
	UserDoesNotExist   = 10001
	ServerError        = 10101
	TooManyRequests    = 10102
)

type ErrorText struct {
	Language string
}

func NewErrorText(language string) *ErrorText {
	return &ErrorText{
		Language: language,
	}
}

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
