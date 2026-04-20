package testkit

import "testing"

func TestSecretKey(t *testing.T) {
	secret := SecretKey("auth")
	if secret == "" {
		t.Fatal("expected non-empty secret")
	}
	if len(secret) < 16 {
		t.Fatalf("expected secret length >=16, got %d", len(secret))
	}

	fallback := SecretKey("")
	if fallback == "" {
		t.Fatal("expected fallback secret to be non-empty")
	}
	if fallback == secret {
		t.Fatal("expected scoped and fallback secrets to differ")
	}
}
