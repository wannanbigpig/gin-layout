package errors

// 菜单管理相关错误码（20020-20023, 20039-20045）。
const (
	MenuNotFound          = 20020
	MenuExists            = 20021
	MenuHasChildren       = 20022
	MenuCannotDelete      = 20023
	MaxMenuDepth          = 20039
	ParentMenuNotExists   = 20040
	ParentMenuTypeInvalid = 20041
	ParentMenuInvalid     = 20042
	MenuCodeExists        = 20043
	MenuRouteNameExists   = 20044
	MenuPathExists        = 20045
)
