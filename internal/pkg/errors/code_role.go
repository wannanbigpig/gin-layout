package errors

// 角色管理相关错误码（20012-20019）。
const (
	RoleNotFound        = 20012
	RoleExists          = 20013
	RoleHasChildren     = 20014
	RoleCannotDelete    = 20015
	ParentRoleNotExists = 20016
	ParentRoleInvalid   = 20017
	MaxRoleDepth        = 20018
	MaxChildRoles       = 20019
)
