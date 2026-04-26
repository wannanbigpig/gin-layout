package errors

import (
	"fmt"
	"strings"
)

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
	TooManyRequests           = 10102

	// 文件相关错误码 11000-11999
	FileIdentifierInvalid = 11001
	FilePrivateAuthNeeded = 11002
	FileAccessDenied      = 11003
	FileUploadPartialFail = 11004

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
	MaxMenuDepth            = 20039
	ParentMenuNotExists     = 20040
	ParentMenuTypeInvalid   = 20041
	ParentMenuInvalid       = 20042
	MenuCodeExists          = 20043
	MenuRouteNameExists     = 20044
	MenuPathExists          = 20045
	LoginAccountLocked      = 20046
)

const (
	MsgKeyAuthSessionExpired        = "auth.session.expired"
	MsgKeyAuthPermissionInitFailed  = "auth.permission.init_failed"
	MsgKeyAuthPermissionCheckFailed = "auth.permission.check_failed"
	MsgKeyAuthAPIOperationDenied    = "auth.api.operation_denied"
	MsgKeyAuthAccountLocked         = "auth.account.locked"
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
