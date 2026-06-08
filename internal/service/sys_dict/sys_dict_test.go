package sys_dict

import (
	"errors"
	"testing"

	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

func TestUpdateTypeRejectsImmutableTypeCodeBeforeDBLookup(t *testing.T) {
	typeCode := "new_type"
	err := NewSysDictService().UpdateType(&form.UpdateSysDictType{Id: 1, TypeCode: &typeCode})

	var businessErr *e.BusinessError
	if !errors.As(err, &businessErr) {
		t.Fatalf("expected business error, got %T: %v", err, err)
	}
	if businessErr.GetCode() != e.InvalidParameter {
		t.Fatalf("expected code %d, got %d", e.InvalidParameter, businessErr.GetCode())
	}
}
