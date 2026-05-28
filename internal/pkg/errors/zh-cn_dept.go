package errors

func init() {
	mergeZhCNText(map[int]string{
		DepartmentNotFound:     "部门不存在",
		DepartmentExists:       "部门已存在",
		DepartmentHasChildren:  "该部门有子部门，无法删除",
		DepartmentCannotDelete: "删除部门失败",
		ParentDeptNotExists:    "上级部门不存在",
		ParentDeptInvalid:      "上级部门不能是当前部门自身或其子部门",
		MaxDeptDepth:           "部门层级超出限制",
	})
}
