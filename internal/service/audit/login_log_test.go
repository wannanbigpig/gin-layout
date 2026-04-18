package audit

import (
	"testing"

	"github.com/wannanbigpig/gin-layout/pkg/utils/crypto"
)

func TestDecryptLoginTokenIfNeeded(t *testing.T) {
	const key = "test-secret-key"
	const plain = "header.payload.signature"

	encrypted, err := crypto.Encrypt(key, plain)
	if err != nil {
		t.Fatalf("encrypt token failed: %v", err)
	}

	if got := decryptLoginTokenIfNeeded(encrypted, key); got != plain {
		t.Fatalf("expected decrypted token %q, got %q", plain, got)
	}
}

func TestDecryptLoginTokenIfNeededFallbackOnDecryptError(t *testing.T) {
	const key = "test-secret-key"
	const raw = "not-encrypted-token"

	if got := decryptLoginTokenIfNeeded(raw, key); got != raw {
		t.Fatalf("expected fallback raw token %q, got %q", raw, got)
	}
}
