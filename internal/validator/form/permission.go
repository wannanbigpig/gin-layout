package form

type EditPermission struct {
	Id          uint   `form:"id" json:"id" binding:"required"`                                                                      // id
	Name        string `form:"name" json:"name" binding:"required,max=60"`                                                           // 权限名称
	Description string `form:"description" json:"desc" binding:"omitempty"`                                                          // 描述
	Method      string `form:"method" json:"method" binding:"omitempty,oneof=GET POST PUT DELETE OPTIONS HEAD PATCH" label:"接口请求方法"` // 接口请求方法
	Route       string `form:"route" json:"route" binding:"omitempty"`                                                               // 接口路由
	Func        string `form:"func" json:"func" binding:"omitempty"`                                                                 // 接口方法
	FuncPath    string `form:"func_path" json:"func_path" binding:"omitempty"`                                                       // 接口方法
	IsAuth      *int8  `form:"is_auth" json:"is_auth" binding:"required,oneof=0 1"`                                                  // 接口方法
	Sort        int32  `form:"sort" json:"sort" binding:"required"`                                                                  // 排序
}

func NewEditApiForm() *EditPermission {
	return &EditPermission{}
}

type ListPermission struct {
	Paginate
	Name        string `form:"name" json:"name" binding:"omitempty,max=60"`                                                          // 权限名称
	Method      string `form:"method" json:"method" binding:"omitempty,oneof=GET POST PUT DELETE OPTIONS HEAD PATCH" label:"接口请求方法"` // 接口请求方法
	Route       string `form:"route" json:"route" binding:"omitempty"`                                                               // 接口路由
	Keyword     string `form:"keyword" json:"keyword" binding:"omitempty"`                                                           // 关键字
	IsAuth      *int8  `form:"is_auth" json:"is_auth" binding:"omitempty,oneof=0 1"`                                                 // 是否授权
	IsEffective *int8  `form:"is_effective" json:"is_effective" binding:"omitempty,oneof=0 1"`                                       // 是否授权
}

func NewListApiQuery() *ListPermission {
	return &ListPermission{}
}
