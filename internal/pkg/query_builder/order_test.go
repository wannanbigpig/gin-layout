package query_builder

import "testing"

func TestResolveListOrder(t *testing.T) {
	for _, tc := range []struct{ field, direction, want string }{
		{"", "", "sort DESC, id DESC"},
		{"created_at", "asc", "created_at ASC, id DESC"},
		{"id", "desc", "id DESC"},
		{"created_at; DROP TABLE users", "asc", "sort DESC, id DESC"},
		{"created_at", "sideways", "sort DESC, id DESC"},
	} {
		got := ResolveListOrder(tc.field, tc.direction, "sort DESC, id DESC", "id", "created_at")
		if got != tc.want {
			t.Fatalf("ResolveListOrder(%q, %q) = %q, want %q", tc.field, tc.direction, got, tc.want)
		}
	}
}
