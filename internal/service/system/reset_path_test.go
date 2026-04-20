package system

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetMigrationsPathPrefersEnvPath(t *testing.T) {
	tempDir := t.TempDir()
	migrationsDir := filepath.Join(tempDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0o755); err != nil {
		t.Fatalf("create migrations dir failed: %v", err)
	}

	// 只要存在一个 *.up.sql 即视为有效迁移目录。
	upFile := filepath.Join(migrationsDir, "000001_init.up.sql")
	if err := os.WriteFile(upFile, []byte("SELECT 1;"), 0o644); err != nil {
		t.Fatalf("write migration file failed: %v", err)
	}

	t.Setenv(migrationsPathEnvKey, migrationsDir)

	got, err := getMigrationsPath()
	if err != nil {
		t.Fatalf("expected migrations path from env, got error: %v", err)
	}
	if got != migrationsDir {
		t.Fatalf("expected %s, got %s", migrationsDir, got)
	}
}
