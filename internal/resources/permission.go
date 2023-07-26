package resources

type PermissionResources struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`      // 权限名称
	Desc     string `json:"desc"`      // 描述
	Method   string `json:"method"`    // 接口请求方法
	Route    string `json:"route"`     // 接口路由
	Func     string `json:"func"`      // 接口方法
	FuncPath string `json:"func_path"` // 接口方法
	IsAuth   int8   `json:"is_auth"`   // 接口方法
	Sort     int32  `json:"sort"`      // 排序
}

func NewPermissionResources() *PermissionResources {
	return &PermissionResources{}
}

type PermissionCollection struct {
	Paginate
	Data []*PermissionResources
}

func NewPermissionCollection() *PermissionCollection {
	return &PermissionCollection{}
}

func (p *PermissionCollection) ToCollection() *Collection {
	data := make([]any, 0, len(p.Data))
	for _, v := range p.Data {
		data = append(data, &PermissionResources{
			ID:       v.ID,
			Name:     v.Name,
			Desc:     v.Desc,
			Method:   v.Method,
			Route:    v.Route,
			Func:     v.Func,
			FuncPath: v.FuncPath,
			IsAuth:   v.IsAuth,
			Sort:     v.Sort,
		})
	}
	return newResponseCollection(p.Paginate, data)
}
