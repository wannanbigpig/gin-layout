package api_permission

import "testing"

func TestAPIPermissionDiffExcludesGeneratedAuthMode(t *testing.T) {
	for _, rule := range apiPermissionDiffRules {
		if rule.Field == "is_auth" {
			t.Fatal("generated is_auth must not be part of editable audit fields")
		}
	}
}
