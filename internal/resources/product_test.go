package resources

import (
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
)

func TestProductTransformerFillsStatusAndDepartmentName(t *testing.T) {
	product := &model.Product{
		Status: 1,
		DeptId: 2,
		Department: model.Department{
			ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{
				BaseModel: model.BaseModel{ID: 2},
			},
			Name: "研发部",
		},
	}

	resource := NewProductTransformer().ToStruct(product)
	if resource.StatusName != "启用" {
		t.Fatalf("expected status name 启用, got %q", resource.StatusName)
	}
	if resource.DeptName != "研发部" {
		t.Fatalf("expected dept name 研发部, got %q", resource.DeptName)
	}
}

func TestProductTransformerCollectionUsesResourceShape(t *testing.T) {
	collection := NewProductTransformer().ToCollection(1, 15, 1, []*model.Product{
		{
			Status: 0,
			DeptId: 1,
			Department: model.Department{
				ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{
					BaseModel: model.BaseModel{ID: 1},
				},
				Name: "默认部门",
			},
		},
	})

	if len(collection.Data) != 1 {
		t.Fatalf("expected one product resource, got %d", len(collection.Data))
	}
	item, ok := collection.Data[0].(*ProductResources)
	if !ok {
		t.Fatalf("expected ProductResources item, got %T", collection.Data[0])
	}
	if item.DeptName != "默认部门" {
		t.Fatalf("expected dept name 默认部门, got %q", item.DeptName)
	}
	if item.StatusName != "禁用" {
		t.Fatalf("expected status name 禁用, got %q", item.StatusName)
	}
}
