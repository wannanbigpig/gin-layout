package form

type Page struct {
	Page  float64 `form:"page" json:"page" binding:"min=1"`   // 必填，页面值>=1
	Limit float64 `form:"limit" json:"limit" binding:"min=1"` // 必填，每页条数值>=1
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
