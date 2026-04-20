package global

import "testing"

func TestApiAuthModeRequiresLogin(t *testing.T) {
	if ApiAuthModeNone.RequiresLogin() {
		t.Fatal("expected none mode to not require login")
	}
	if !ApiAuthModeLogin.RequiresLogin() {
		t.Fatal("expected login mode to require login")
	}
	if !ApiAuthModeAuthz.RequiresLogin() {
		t.Fatal("expected authz mode to require login")
	}
}

func TestApiAuthModeRequiresAPIPermission(t *testing.T) {
	if ApiAuthModeNone.RequiresAPIPermission() {
		t.Fatal("expected none mode to not require api permission")
	}
	if ApiAuthModeLogin.RequiresAPIPermission() {
		t.Fatal("expected login mode to not require api permission")
	}
	if !ApiAuthModeAuthz.RequiresAPIPermission() {
		t.Fatal("expected authz mode to require api permission")
	}
}

func TestApiAuthModeLabel(t *testing.T) {
	cases := map[ApiAuthMode]string{
		ApiAuthModeNone:  "无需登录",
		ApiAuthModeLogin: "需要登录",
		ApiAuthModeAuthz: "需要登录和API权限",
		ApiAuthMode(99):  "-",
	}

	for mode, expected := range cases {
		if got := mode.Label(); got != expected {
			t.Fatalf("unexpected label for mode %d: got %q want %q", mode, got, expected)
		}
	}
}
