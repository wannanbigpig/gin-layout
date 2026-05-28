package errors

func init() {
	mergeEnUSText(map[int]string{
		DepartmentNotFound:     "Department not found",
		DepartmentExists:       "Department already exists",
		DepartmentHasChildren:  "Cannot delete department with child departments",
		DepartmentCannotDelete: "Failed to delete department",
		ParentDeptNotExists:    "Parent department not exists",
		ParentDeptInvalid:      "Parent department cannot be itself or its child department",
		MaxDeptDepth:           "Department level exceeds limit",
	})
}
