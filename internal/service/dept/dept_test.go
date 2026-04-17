package dept

import "testing"

func TestGenerateDeptCodeUsesUniqueDefaultPrefix(t *testing.T) {
	service := NewDeptService()

	first := service.generateDeptCode()
	second := service.generateDeptCode()

	if first == "" || second == "" {
		t.Fatal("expected generated dept code")
	}
	if first == second {
		t.Fatalf("expected different dept codes, got %s", first)
	}
	if first[:5] != "dept_" || second[:5] != "dept_" {
		t.Fatalf("expected dept_ prefix, got %s and %s", first, second)
	}
}
