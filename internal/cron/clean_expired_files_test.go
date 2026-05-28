package taskcron

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCleanExpiredFilesHandlerUsesCursorAndDoesNotLoopOnFailure(t *testing.T) {
	db := newCleanExpiredFilesTestDB(t)
	now := time.Now()
	for i := 1; i <= 205; i++ {
		if err := db.Exec(`INSERT INTO upload_files (id, origin_name, expires_at) VALUES (?, ?, ?)`,
			i, "file-"+filepath.Base(filepath.Join("tmp", "f")), now.Add(-time.Minute)).Error; err != nil {
			t.Fatalf("insert expired file %d failed: %v", i, err)
		}
	}

	previousGetDB := cleanExpiredFilesGetDB
	previousDestroy := cleanExpiredFilesDestroy
	previousBatchSize := cleanExpiredFilesBatchSize
	defer func() {
		cleanExpiredFilesGetDB = previousGetDB
		cleanExpiredFilesDestroy = previousDestroy
		cleanExpiredFilesBatchSize = previousBatchSize
	}()

	cleanExpiredFilesGetDB = func(model ...any) (*gorm.DB, error) { return db, nil }
	cleanExpiredFilesBatchSize = 50

	visited := make(map[uint]int)
	cleanExpiredFilesDestroy = func(id uint) error {
		visited[id]++
		if id == 50 {
			return errors.New("delete failed")
		}
		return nil
	}

	if err := CleanExpiredFilesHandler(context.Background(), nil); err != nil {
		t.Fatalf("CleanExpiredFilesHandler returned error: %v", err)
	}

	if len(visited) != 205 {
		t.Fatalf("expected all expired files to be scanned once, got %d", len(visited))
	}
	for id, count := range visited {
		if count != 1 {
			t.Fatalf("expected file %d scanned once, got %d", id, count)
		}
	}
}

func newCleanExpiredFilesTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.Exec(`CREATE TABLE upload_files (
		id integer primary key,
		origin_name text,
		expires_at datetime
	)`).Error; err != nil {
		t.Fatalf("create upload_files failed: %v", err)
	}
	return db
}
