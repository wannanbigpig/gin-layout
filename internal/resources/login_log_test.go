package resources

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

func TestAdminLoginLogTransformerOnlyExposeTokenHashesInDetail(t *testing.T) {
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

	if resource.JwtID != "jwt-id" || resource.UserAgent != "ua" {
		t.Fatal("expected detail basic fields to be preserved")
	}
	if resource.TokenHash != "access-hash" || resource.RefreshTokenHash != "refresh-hash" {
		t.Fatal("expected token hashes to be preserved")
	}
	if resource.TokenExpires == nil || resource.RefreshExpires == nil {
		t.Fatal("expected token expiry fields to be preserved")
	}
	if !resource.TokenExpires.Time.Equal(now.Time) || !resource.RefreshExpires.Time.Equal(now.Time) {
		t.Fatal("expected token expiry values to be preserved")
	}

	payload, err := json.Marshal(resource)
	if err != nil {
		t.Fatalf("marshal resource failed: %v", err)
	}
	fields := map[string]any{}
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatalf("unmarshal resource payload failed: %v", err)
	}
	if _, ok := fields["access_token"]; ok {
		t.Fatal("expected access_token to be hidden from detail response")
	}
	if _, ok := fields["refresh_token"]; ok {
		t.Fatal("expected refresh_token to be hidden from detail response")
	}
}
