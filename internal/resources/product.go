package resources

import (
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

type ProductResources struct {
	ID          uint             `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Price       int64            `json:"price"`
	Status      uint8            `json:"status"`
	StatusName  string           `json:"status_name"`
	DeptId      uint             `json:"dept_id"`
	CreatedBy   uint             `json:"created_by"`
	CreatedAt   utils.FormatDate `json:"created_at"`
	UpdatedAt   utils.FormatDate `json:"updated_at"`
}

func (r *ProductResources) SetCustomFields(data *model.Product) {
	if data == nil {
		return
	}
	r.StatusName = model.ProductStatusDict.Map(data.Status)
}

type ProductTransformer struct {
	BaseResources[*model.Product, *ProductResources]
}

func NewProductTransformer() ProductTransformer {
	return ProductTransformer{
		BaseResources: BaseResources[*model.Product, *ProductResources]{
			NewResource: func() *ProductResources {
				return &ProductResources{}
			},
		},
	}
}
