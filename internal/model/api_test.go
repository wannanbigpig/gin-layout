package model

import (
	"testing"
)

func TestApiRegisterConflictUpdateColumnsRefreshRouteAuthMode(t *testing.T) {
	columns := make(map[string]struct{}, len(apiRegisterConflictUpdateColumns))
	for _, column := range apiRegisterConflictUpdateColumns {
		columns[column] = struct{}{}
	}

	required := []string{"name", "name_i18n", "description", "description_i18n", "method", "route", "func", "group_code", "func_path", "is_auth", "is_effective", "updated_at"}
	for _, column := range required {
		if _, ok := columns[column]; !ok {
			t.Fatalf("expected api register conflict updates to include %q", column)
		}
	}

	manualColumns := []string{"code", "created_at", "sort"}
	for _, column := range manualColumns {
		if _, ok := columns[column]; ok {
			t.Fatalf("expected api register conflict updates not to overwrite %q", column)
		}
	}
}
