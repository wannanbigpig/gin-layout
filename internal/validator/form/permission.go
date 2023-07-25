package form

type EditPermission struct {
	Id       uint   `from:"id" json:"id" binding:"omitempty"`                                                                          // id
	Name     string `from:"name" json:"name" binding:"required,max=60"`                                                                // 权限名称
	Desc     string `from:"desc" json:"desc" binding:"omitempty"`                                                                      // 描述
	Method   string `from:"method" json:"method" binding:"required,oneof=GET POST PUT DELETE OPTIONS HEAD PATCH" label:"接口请求方法"` // 接口请求方法
	Route    string `from:"route" json:"route" binding:"required"`                                                                     // 接口路由
	Func     string `from:"func" json:"func" binding:"required"`                                                                       // 接口方法
	FuncPath string `from:"func_path" json:"func_path" binding:"required"`                                                             // 接口方法
	IsAuth   int8   `from:"is_auth" json:"is_auth" binding:"required"`                                                                 // 接口方法
	Sort     int32  `from:"sort" json:"sort" binding:"required"`                                                                       // 排序
}

type ListPermission struct {
	Page
	Name   string `from:"name" json:"name" binding:"omitempty,max=60"` // 权限名称
	Method string `from:"method" json:"method" binding:"omitempty"`    // 接口请求方法
	Route  string `from:"route" json:"route" binding:"omitempty"`      // 接口路由
}

func EditPermissionForm() *EditPermission {
	return &EditPermission{}
}
