package errors

func init() {
	mergeEnUSText(map[int]string{
		RoleNotFound:     "Role not found",
		RoleExists:       "Role already exists",
		RoleCannotDelete: "Failed to delete role",
	})
}
