package query_builder

import "testing"

var queryBuilderConditionSink string

func TestQueryBuilderBuildsExpectedCondition(t *testing.T) {
	status := int8(1)
	pid := uint(3)

	condition, args := New().
		AddKeywordLike("dashboard", "title", "path").
		AddEq("status", &status).
		AddEq("pid", &pid).
		Build()

	expected := "(title like ? OR path like ?) AND status = ? AND pid = ?"
	if condition != expected {
		t.Fatalf("unexpected condition: %s", condition)
	}
	if len(args) != 4 {
		t.Fatalf("unexpected args len: %d", len(args))
	}
}

func TestQueryBuilderSkipsEmptyValues(t *testing.T) {
	empty := ""

	condition, args := New().
		AddLike("name", "").
		AddEq("code", &empty).
		Build()

	if condition != "" {
		t.Fatalf("expected empty condition, got %s", condition)
	}
	if args != nil {
		t.Fatalf("expected nil args, got %#v", args)
	}
}

func TestQueryBuilderBuildSingleConditionDoesNotAllocate(t *testing.T) {
	query := New().AddCondition("status = ?", 1)
	allocs := testing.AllocsPerRun(1000, func() {
		queryBuilderConditionSink, _ = query.Build()
	})
	if allocs != 0 {
		t.Fatalf("single condition Build allocated %.0f times, want 0", allocs)
	}
}
