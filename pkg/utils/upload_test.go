package utils

import (
	"bytes"
	"testing"
)

func TestIsAllowedImageHandlesShortNonImageFile(t *testing.T) {
	file := bytes.NewReader([]byte("x"))

	ext, allowed, err := IsAllowedImage(file)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if allowed {
		t.Fatalf("expected non-image short file to be rejected")
	}
	if ext != "" {
		t.Fatalf("expected empty extension, got %q", ext)
	}
}
