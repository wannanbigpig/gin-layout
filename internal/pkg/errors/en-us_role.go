package errors

func init() {
	mergeEnUSText(map[int]string{
		RoleNotFound:        "Role not found",
		RoleExists:          "Role already exists",
		RoleHasChildren:     "Cannot delete role with child roles",
		RoleCannotDelete:    "Failed to delete role",
		ParentRoleNotExists: "Parent role not exists",
		ParentRoleInvalid:   "Parent role cannot be itself or its child role",
		MaxRoleDepth:        "Can only create up to 2 levels of roles",
		MaxChildRoles:       "Each top-level role can have up to 5 child roles",
	})
}
