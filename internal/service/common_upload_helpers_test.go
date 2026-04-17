package service

import "testing"

func TestNormalizeUploadPathRejectsTraversal(t *testing.T) {
	invalidPaths := []string{"../secret", "../../tmp", "/tmp/uploads", `..\\escape`}
	for _, input := range invalidPaths {
		if _, err := normalizeUploadPath(input); err == nil {
			t.Fatalf("expected path %q to be rejected", input)
		}
	}
}

func TestNormalizeUploadPathKeepsRelativeSubdirs(t *testing.T) {
	path, err := normalizeUploadPath("avatars/admin")
	if err != nil {
		t.Fatalf("expected valid relative path, got %v", err)
	}
	if path != "avatars/admin" {
		t.Fatalf("unexpected normalized path: %q", path)
	}
}
