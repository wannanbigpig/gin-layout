package errors

// 部门管理相关错误码（20024-20030）。
const (
	DepartmentNotFound     = 20024
	DepartmentExists       = 20025
	DepartmentHasChildren  = 20026
	DepartmentCannotDelete = 20027
	ParentDeptNotExists    = 20028
	ParentDeptInvalid      = 20029
	MaxDeptDepth           = 20030
)
