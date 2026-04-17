package resources

import (
	"testing"
	"time"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

func TestAdminLoginLogTransformerDoesNotExposeTokens(t *testing.T) {
	now := utils.FormatDate{Time: time.Now()}
	resource := NewAdminLoginLogTransformer().ToStruct(&model.AdminLoginLogs{
		JwtID:            "jwt-id",
		UserAgent:        "ua",
		AccessToken:      "plain-access-token",
		RefreshToken:     "plain-refresh-token",
		TokenHash:        "access-hash",
		RefreshTokenHash: "refresh-hash",
		TokenExpires:     &now,
		RefreshExpires:   &now,
	})

	if resource.AccessToken != "" {
		t.Fatalf("expected access token to be hidden, got %q", resource.AccessToken)
	}
	if resource.RefreshToken != "" {
		t.Fatalf("expected refresh token to be hidden, got %q", resource.RefreshToken)
	}
	if resource.TokenHash != "access-hash" || resource.RefreshTokenHash != "refresh-hash" {
		t.Fatal("expected token hashes to be preserved")
	}
}
