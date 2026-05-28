package errors

// 鉴权与登录相关错误码（20001-20011, 20031-20038, 20046-20048）。
const (
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

	CasbinInitFailed       = 20031
	TokenGenerateFailed    = 20032
	LoginFailed            = 20033
	CreateUserFailed       = 20034
	UpdateUserFailed       = 20035
	DeleteUserFailed       = 20036
	QueryUserDeptFailed    = 20037
	SuperAdminMustKeepRole = 20038

	LoginAccountLocked  = 20046
	PasswordRequired    = 20047
	RefreshTokenInvalid = 20048
)

// 鉴权相关文案 key。
const (
	MsgKeyAuthSessionExpired        = "auth.session.expired"
	MsgKeyAuthPermissionInitFailed  = "auth.permission.init_failed"
	MsgKeyAuthPermissionCheckFailed = "auth.permission.check_failed"
	MsgKeyAuthAPIOperationDenied    = "auth.api.operation_denied"
	MsgKeyAuthAccountLocked         = "auth.account.locked"
)
