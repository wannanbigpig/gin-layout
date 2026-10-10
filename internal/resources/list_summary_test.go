package resources

import (
	"encoding/json"
	"testing"
)

func TestListSummaryResourcesKeepPaginationFields(t *testing.T) {
	collection := NewCollection().SetPaginate(1, 10, 1).ToCollection([]any{map[string]string{"name": "item"}})
	for name, value := range map[string]any{
		"todo":   &UserTodoListResources{Collection: collection, Summary: &UserTodoSummaryResources{Total: 3}},
		"export": &MyExportRecordListResources{Collection: collection, Summary: &MyExportSummaryResources{Total: 5}},
	} {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var result struct {
			Total       int64 `json:"total"`
			CurrentPage int   `json:"current_page"`
			Data        []any `json:"data"`
			Summary     struct {
				Total int64 `json:"total"`
			} `json:"summary"`
		}
		if err := json.Unmarshal(encoded, &result); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if result.Total != 1 || result.CurrentPage != 1 || len(result.Data) != 1 || result.Summary.Total < 3 {
			t.Fatalf("%s: unexpected response %s", name, encoded)
		}
	}
}
