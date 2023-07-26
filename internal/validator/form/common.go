package form

type Paginate struct {
	Page    int `form:"page" json:"page" binding:"omitempty,gt=0"`         // 必填，页面值>=1
	PerPage int `form:"per_page" json:"per_page" binding:"omitempty,gt=0"` // 必填，每页条数值>=1
}

type ID struct {
	ID uint `form:"id" json:"id" binding:"required"`
}

func NewIDForm() *ID {
	return &ID{}
}
