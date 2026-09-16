package form

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProductNameLengthMatchesDatabaseColumn(t *testing.T) {
	for _, test := range []struct {
		name    string
		length  int
		wantErr bool
	}{
		{name: "accepts 200 characters", length: 200},
		{name: "rejects 201 characters", length: 201, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			body, err := json.Marshal(map[string]any{"name": strings.Repeat("产", test.length)})
			if err != nil {
				t.Fatalf("marshal request: %v", err)
			}
			err = bindJSONBody(t, string(body), NewCreateProductForm())
			if (err != nil) != test.wantErr {
				t.Fatalf("validation error = %v, wantErr = %v", err, test.wantErr)
			}
		})
	}
}
