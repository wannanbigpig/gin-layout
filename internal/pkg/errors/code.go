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

	// 业务错误码 20000-29999
	UserPasswordWrong       = 20001
	UserExists              = 20002
	PhoneNumberExists       = 20003
	EmailExists             = 20004
	UsernameRequired        = 20005
	NicknameRequired        = 20006
	PasswordProcessFailed   = 20007
	SuperAdminCannotModify  = 20008
	SuperAdminCannotDisable = 20009
	SuperAdminCannotDelete  = 20010
	SamePassword            = 20011
	RoleNotFound            = 20012
	RoleExists              = 20013
	RoleHasChildren         = 20014
	RoleCannotDelete        = 20015
	ParentRoleNotExists     = 20016
	ParentRoleInvalid       = 20017
	MaxRoleDepth            = 20018
	MaxChildRoles           = 20019
	MenuNotFound            = 20020
	MenuExists              = 20021
	MenuHasChildren         = 20022
	MenuCannotDelete        = 20023
	DepartmentNotFound      = 20024
	DepartmentExists        = 20025
	DepartmentHasChildren   = 20026
	DepartmentCannotDelete  = 20027
	ParentDeptNotExists     = 20028
	ParentDeptInvalid       = 20029
	MaxDeptDepth            = 20030
	CasbinInitFailed        = 20031
	TokenGenerateFailed     = 20032
	LoginFailed             = 20033
	CreateUserFailed        = 20034
	UpdateUserFailed        = 20035
	DeleteUserFailed        = 20036
	QueryUserDeptFailed     = 20037
	SuperAdminMustKeepRole  = 20038
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
