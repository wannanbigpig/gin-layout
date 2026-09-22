package errors

// 通用错误码（0-10999）。
const (
	SUCCESS                   = 0
	FAILURE                   = 1
	AuthorizationErr          = 403
	NotFound                  = 404
	CaptchaErr                = 400
	NotLogin                  = 401
	ServerErr                 = 500
	InvalidParameter          = 10000
	UserDoesNotExist          = 10001
	UserDisable               = 10002
	ServiceDependencyNotReady = 10003
	NoExportData              = 10004
	TooManyRequests           = 10102
)
