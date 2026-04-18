package resources

import (
	"testing"
	"time"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

func TestAdminLoginLogTransformerExposeTokensInDetail(t *testing.T) {
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

	if resource.AccessToken != "plain-access-token" {
		t.Fatalf("expected access token to be exposed in detail, got %q", resource.AccessToken)
	}
	if resource.RefreshToken != "plain-refresh-token" {
		t.Fatalf("expected refresh token to be exposed in detail, got %q", resource.RefreshToken)
	}
	if resource.TokenHash != "access-hash" || resource.RefreshTokenHash != "refresh-hash" {
		t.Fatal("expected token hashes to be preserved")
	}
}
