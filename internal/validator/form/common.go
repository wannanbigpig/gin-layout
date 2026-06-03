package form

type Paginate struct {
	Page    int `form:"page" json:"page" binding:"omitempty,gt=0"`                // 页码，>=1
	PerPage int `form:"per_page" json:"per_page" binding:"omitempty,gt=0,max=200"` // 每页条数，1~200
}

// NewPaginate 创建一个新的分页查询
func NewPaginate() *Paginate {
	return &Paginate{}
}

type ID struct {
	ID uint `form:"id" json:"id" binding:"required"`
}

// NewIdForm ID表单
func NewIdForm() *ID {
	return &ID{}
}
