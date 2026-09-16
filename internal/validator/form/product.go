package form

type ProductList struct {
	Paginate
	Status *int8  `form:"status" json:"status" binding:"omitempty,oneof=0 1"`
	Name   string `form:"name" json:"name" binding:"omitempty,max=200"`
}

func NewProductListQuery() *ProductList {
	return &ProductList{}
}

type productMutablePayload struct {
	Name        string `form:"name" json:"name" binding:"required,max=200"`
	Description string `form:"description" json:"description" binding:"omitempty"`
	Price       int64  `form:"price" json:"price" binding:"omitempty,gte=0"`
	Status      uint8  `form:"status" json:"status" binding:"omitempty,oneof=0 1"`
	DeptId      uint   `form:"dept_id" json:"dept_id" binding:"omitempty"` // 部门ID，可选，不传则使用用户默认部门
}

type CreateProduct struct {
	productMutablePayload
}

func NewCreateProductForm() *CreateProduct {
	return &CreateProduct{}
}

type UpdateProduct struct {
	Id uint `form:"id" json:"id" binding:"required"`
	productMutablePayload
}

func NewUpdateProductForm() *UpdateProduct {
	return &UpdateProduct{}
}

func (f *UpdateProduct) GetIDPointer() *uint {
	return &f.Id
}
