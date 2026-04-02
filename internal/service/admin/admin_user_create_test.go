package admin

import (
	"errors"
	"testing"

	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

func TestAdminUserCreateRequiresUsername(t *testing.T) {
	service := NewAdminUserService()
	nickname := "nick"
	params := form.NewCreateAdminUser()
	params.Nickname = &nickname

	err := service.Create(params)

	assertBusinessErrorMessage(t, err, e.UsernameRequired, "用户名必填")
}

func TestAdminUserCreateRequiresNickname(t *testing.T) {
	service := NewAdminUserService()
	username := "admin"
	params := form.NewCreateAdminUser()
	params.Username = &username

	err := service.Create(params)

	assertBusinessErrorMessage(t, err, e.NicknameRequired, "昵称必填")
}

func assertBusinessErrorMessage(t *testing.T, err error, code int, message string) {
	t.Helper()

	var businessErr *e.BusinessError
	if !errors.As(err, &businessErr) {
		t.Fatalf("expected business error, got %v", err)
	}
	if businessErr.GetCode() != code {
		t.Fatalf("expected code %d, got %d", code, businessErr.GetCode())
	}
	if businessErr.GetMessage() != message {
		t.Fatalf("expected message %q, got %q", message, businessErr.GetMessage())
	}
}
