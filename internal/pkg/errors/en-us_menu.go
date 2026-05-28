package errors

func init() {
	mergeEnUSText(map[int]string{
		MenuNotFound:          "Menu not found",
		MenuExists:            "Menu already exists",
		MenuHasChildren:       "Cannot delete menu with child menus",
		MenuCannotDelete:      "Failed to delete menu",
		MaxMenuDepth:          "Can only create up to 4 levels of menu",
		ParentMenuNotExists:   "Parent menu not exists",
		ParentMenuTypeInvalid: "Parent menu cannot be a button",
		ParentMenuInvalid:     "Parent menu cannot be itself or its child menu",
		MenuCodeExists:        "Permission code already exists",
		MenuRouteNameExists:   "Route name already exists",
		MenuPathExists:        "Route already exists",
	})
}
