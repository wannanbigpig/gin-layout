package service

import (
	"reflect"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

func TestExtractFileUUID(t *testing.T) {
	uuid := "1234567890abcdef1234567890abcdef"
	cases := []string{
		"/admin/v1/file/" + uuid,
		"https://example.com/admin/v1/file/" + uuid,
	}
	for _, input := range cases {
		if got := ExtractFileUUID(input); got != uuid {
			t.Fatalf("ExtractFileUUID(%q) = %q", input, got)
		}
	}
	if got := ExtractFileUUID("/admin/v1/file/not-found"); got != "" {
		t.Fatalf("expected invalid uuid to be empty, got %q", got)
	}
}

func TestReferenceListFileIDUsesIDAlias(t *testing.T) {
	got := referenceListFileID(&form.FileReferenceList{ID: 12})
	if got != 12 {
		t.Fatalf("expected id alias to be used as file id, got %d", got)
	}
}

func TestReferenceListFileIDPrefersFileID(t *testing.T) {
	got := referenceListFileID(&form.FileReferenceList{ID: 12, FileID: 34})
	if got != 34 {
		t.Fatalf("expected file_id to take precedence, got %d", got)
	}
}

func TestBuildFileReferenceListQuerySkipsZeroOwnerID(t *testing.T) {
	condition, args := buildFileReferenceListQuery(&form.FileReferenceList{ID: 1}).Build()
	if condition != "file_id = ?" {
		t.Fatalf("expected only file_id condition, got %q", condition)
	}
	expectedArgs := []any{uint(1)}
	if !reflect.DeepEqual(args, expectedArgs) {
		t.Fatalf("expected args %#v, got %#v", expectedArgs, args)
	}
}

func TestBuildFileReferenceListQueryAddsPositiveOwnerID(t *testing.T) {
	condition, args := buildFileReferenceListQuery(&form.FileReferenceList{ID: 1, OwnerID: 2}).Build()
	if condition != "file_id = ? AND owner_id = ?" {
		t.Fatalf("expected file_id and owner_id conditions, got %q", condition)
	}
	expectedArgs := []any{uint(1), uint(2)}
	if !reflect.DeepEqual(args, expectedArgs) {
		t.Fatalf("expected args %#v, got %#v", expectedArgs, args)
	}
}
