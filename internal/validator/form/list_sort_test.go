package form

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestListSortQueryBinding(t *testing.T) {
	for _, newForm := range []struct {
		name string
		new  func() any
		read func(any) ListSort
	}{
		{"role", func() any { return NewRoleListQuery() }, func(v any) ListSort { return v.(*RoleList).ListSort }},
		{"task", func() any { return NewTaskRunListQuery() }, func(v any) ListSort { return v.(*TaskRunList).ListSort }},
		{"todo", func() any { return NewUserTodoListQuery() }, func(v any) ListSort { return v.(*UserTodoList).ListSort }},
	} {
		t.Run(newForm.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodGet, "/?sort_by=created_at&sort_order=asc", nil)
			payload := newForm.new()
			if err := ctx.ShouldBindQuery(payload); err != nil {
				t.Fatalf("bind list sort: %v", err)
			}
			if got := newForm.read(payload); got.SortBy != "created_at" || got.SortOrder != "asc" {
				t.Fatalf("unexpected sort query: %+v", got)
			}
		})
	}
}
