package api_permission

import (
	"errors"
	"strings"
	"testing"

	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func TestValidateAuthModeUpdateAllowsOmittedAndSameValue(t *testing.T) {
	same := int8(2)
	if err := validateAuthModeUpdate(2, nil); err != nil {
		t.Fatalf("omitted is_auth should be allowed: %v", err)
	}
	if err := validateAuthModeUpdate(2, &same); err != nil {
		t.Fatalf("same is_auth should be compatible: %v", err)
	}
}

func TestValidateAuthModeUpdateRejectsChange(t *testing.T) {
	requested := int8(1)
	err := validateAuthModeUpdate(2, &requested)
	var businessErr *e.BusinessError
	if !errors.As(err, &businessErr) {
		t.Fatalf("expected business error, got %T: %v", err, err)
	}
	if businessErr.GetCode() != e.InvalidParameter || !strings.Contains(businessErr.GetMessage(), "路由声明") {
		t.Fatalf("unexpected error: %v", businessErr)
	}
}
