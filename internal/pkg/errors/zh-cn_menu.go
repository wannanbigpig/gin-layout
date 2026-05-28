package errors

func init() {
	mergeZhCNText(map[int]string{
		MenuNotFound:          "菜单不存在",
		MenuExists:            "菜单已存在",
		MenuHasChildren:       "该菜单有子菜单，无法删除",
		MenuCannotDelete:      "删除菜单失败",
		MaxMenuDepth:          "最多只能创建 4 层菜单",
		ParentMenuNotExists:   "上级菜单不存在",
		ParentMenuTypeInvalid: "上级菜单不能是按钮类型",
		ParentMenuInvalid:     "上级菜单不能是当前菜单自身或其子菜单",
		MenuCodeExists:        "权限标识已存在",
		MenuRouteNameExists:   "路由名称已存在",
		MenuPathExists:        "路由已存在",
	})
}
