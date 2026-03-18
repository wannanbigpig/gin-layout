package resources

import "testing"

type baseTestModel struct {
	ID uint
}

type baseTestResource struct {
	ID       uint
	Computed string
}

func (r *baseTestResource) SetCustomFields(data baseTestModel) {
	r.Computed = "ok"
}

func TestBaseResourcesToCollectionTransformsItems(t *testing.T) {
	transformer := BaseResources[baseTestModel, *baseTestResource]{
		NewResource: func() *baseTestResource {
			return &baseTestResource{}
		},
	}

	collection := transformer.ToCollection(1, 10, 2, []baseTestModel{{ID: 1}, {ID: 2}})
	if len(collection.Data) != 2 {
		t.Fatalf("unexpected data len: %d", len(collection.Data))
	}

	item, ok := collection.Data[0].(*baseTestResource)
	if !ok {
		t.Fatalf("expected transformed resource, got %#v", collection.Data[0])
	}
	if item.Computed != "ok" {
		t.Fatalf("expected custom field to be applied, got %#v", item)
	}
}

func TestPaginateCalculateLastPageUsesIntegerCeil(t *testing.T) {
	collection := NewCollection().SetPaginate(1, 10, 21)
	if collection.LastPage != 3 {
		t.Fatalf("expected last page 3, got %d", collection.LastPage)
	}
}
