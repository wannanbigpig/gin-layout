package filestorage

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestLocalDriverPutExistsOpenDelete(t *testing.T) {
	publicDir := t.TempDir()
	privateDir := t.TempDir()
	driver := NewLocalDriver(LocalConfig{}, publicDir, privateDir)
	ctx := context.Background()

	result, err := driver.Put(ctx, PutInput{Bucket: "public", ObjectKey: "avatars/a.txt", Reader: strings.NewReader("ok"), Size: 2, ContentType: "text/plain"})
	if err != nil {
		t.Fatalf("put failed: %v", err)
	}
	if result.ObjectKey != "avatars/a.txt" {
		t.Fatalf("unexpected object key: %s", result.ObjectKey)
	}
	exists, err := driver.Exists(ctx, "public", "avatars/a.txt")
	if err != nil || !exists {
		t.Fatalf("expected object to exist, exists=%v err=%v", exists, err)
	}
	body, err := driver.Open(ctx, "public", "avatars/a.txt")
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	defer body.Close()
	content, _ := io.ReadAll(body)
	if string(content) != "ok" {
		t.Fatalf("unexpected content: %s", string(content))
	}
	if _, err := driver.SignedURL(ctx, "public", "avatars/a.txt", time.Minute); err != nil {
		t.Fatalf("signed url failed: %v", err)
	}
	if err := driver.Delete(ctx, "public", "avatars/a.txt"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	exists, err = driver.Exists(ctx, "public", "avatars/a.txt")
	if err != nil || exists {
		t.Fatalf("expected object to be deleted, exists=%v err=%v", exists, err)
	}
}
