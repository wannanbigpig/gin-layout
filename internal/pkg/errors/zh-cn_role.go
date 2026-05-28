package errors

func init() {
	mergeZhCNText(map[int]string{
		RoleNotFound:        "角色不存在",
		RoleExists:          "角色已存在",
		RoleHasChildren:     "该角色有子角色，无法删除",
		RoleCannotDelete:    "删除角色失败",
		ParentRoleNotExists: "上级角色不存在",
		ParentRoleInvalid:   "上级角色不能是当前角色自身或其子角色",
		MaxRoleDepth:        "最多只能创建2层角色",
		MaxChildRoles:       "每个顶级角色下最多只能创建5个子角色",
	})
}
