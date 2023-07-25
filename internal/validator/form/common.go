package form

type Page struct {
	Page    int `form:"page" json:"page" binding:"min=1"`      // 必填，页面值>=1
	PerPage int `form:"limit" json:"per_page" binding:"min=1"` // 必填，每页条数值>=1
}

func PageForm() *ID {
	return &ID{}
}

type ID struct {
	ID uint `form:"id" json:"id" binding:"required"`
}

func IDForm() *ID {
	return &ID{}
}
