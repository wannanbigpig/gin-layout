package role

import "testing"

func TestGenerateRoleCodeUsesUniqueDefaultPrefix(t *testing.T) {
	service := NewRoleService()

	first := service.generateRoleCode()
	second := service.generateRoleCode()

	if first == "" || second == "" {
		t.Fatal("expected generated role code")
	}
	if first == second {
		t.Fatalf("expected different role codes, got %s", first)
	}
	if first[:5] != "role_" || second[:5] != "role_" {
		t.Fatalf("expected role_ prefix, got %s and %s", first, second)
	}
}
