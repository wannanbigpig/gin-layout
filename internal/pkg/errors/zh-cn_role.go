package errors

func init() {
	mergeZhCNText(map[int]string{
		RoleNotFound:     "角色不存在",
		RoleExists:       "角色已存在",
		RoleCannotDelete: "删除角色失败",
	})
}
