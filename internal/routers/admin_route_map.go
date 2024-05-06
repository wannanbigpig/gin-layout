package routers

type routeMap struct {
	Name   string
	Desc   string
	IsAuth int
}

var AdminRouteMap = map[string]routeMap{
	"/ping":                  {Name: "心跳检测", Desc: "检测服务是否存活", IsAuth: 0},
	"/api/v1/demo":           {Name: "Demo", Desc: "一个请求示例", IsAuth: 0},
	"/api/v1/admin/login":    {Name: "管理员登录", Desc: "管理员登录接口", IsAuth: 0},
	"/api/v1/admin/logout":   {Name: "管理员登出", Desc: "管理员登出接口", IsAuth: 0},
	"/api/v1/admin-user/get": {Name: "获取登录用户信息", Desc: "获取当前登录管理员信息", IsAuth: 0},
	// 权限
	"/api/v1/permission/edit": {Name: "权限编辑", Desc: "权限的增加和修改", IsAuth: 1},
	"/api/v1/permission/list": {Name: "权限列表", Desc: "获取权限列表", IsAuth: 1},
}
