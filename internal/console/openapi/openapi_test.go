package openapi

import (
	"strings"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/routers"
)

func TestBuildYAMLRendersCredentialSecuritySchemes(t *testing.T) {
	endpoints := []endpoint{
		{Path: "/public", Method: "get", Summary: "public", Tag: "test", Security: routers.SecurityPublic},
		{Path: "/refresh", Method: "post", Summary: "refresh", Tag: "test", Security: routers.SecurityRefreshCookie},
		{Path: "/ws", Method: "get", Summary: "ws", Tag: "test", Security: routers.SecurityWebSocketTicket},
		{Path: "/auth", Method: "get", Summary: "auth", Tag: "test", Auth: routers.AuthModeAuth, Security: routers.SecurityAccessTokenPermission},
	}
	yaml := buildYAML(endpoints)
	for _, expected := range []string{
		"RefreshCookie:\n      type: apiKey",
		"WebSocketTicket:\n      type: apiKey",
		"\"/public\":\n    get:",
		"\"/refresh\":\n    post:",
		"- RefreshCookie: []",
		"- WebSocketTicket: []",
		"- BearerAuth: []",
		"x-security-policy: \"public\"",
	} {
		if !strings.Contains(yaml, expected) {
			t.Errorf("generated YAML missing %q", expected)
		}
	}
}
