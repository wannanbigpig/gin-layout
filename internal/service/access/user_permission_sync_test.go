package access

import (
	"errors"
	"reflect"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUserPermissionSyncForEachUserDeduplicatesInOrder(t *testing.T) {
	service := NewUserPermissionSyncService()
	var visited []uint

	err := service.forEachUser([]uint{2, 5, 2, 0, 5}, func(userID uint) error {
		visited = append(visited, userID)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []uint{2, 5, 0}
	if !reflect.DeepEqual(visited, want) {
		t.Fatalf("unexpected visit order: got %v want %v", visited, want)
	}
}

func TestUserPermissionSyncForEachUserStopsOnError(t *testing.T) {
	service := NewUserPermissionSyncService()
	wantErr := errors.New("stop")

	err := service.forEachUser([]uint{1, 2, 3}, func(userID uint) error {
		if userID == 2 {
			return wantErr
		}
		return nil
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestExpandRoleAncestorsSkipsDisabledRoles(t *testing.T) {
	roleMap := map[uint]roleStatusInfo{
		1: {ID: 1, Status: 1},
		2: {ID: 2, Pids: "1", Status: 1},
		3: {ID: 3, Pids: "1,2", Status: 0},
	}

	got := expandRoleAncestors([]uint{2, 3}, roleMap)
	want := []uint{2, 1}
	if !reflect.DeepEqual(got, want) && !reflect.DeepEqual(got, []uint{1, 2}) {
		t.Fatalf("unexpected expanded roles: got %v want %v", got, want)
	}
}

func TestPermissionSyncCoordinatorRunAfterCommitReloadsOnce(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}

	coordinator := NewPermissionSyncCoordinator()
	reloadCount := 0
	originalReloadPolicy := reloadPolicy
	t.Cleanup(func() {
		reloadPolicy = originalReloadPolicy
	})
	reloadPolicy = func() error {
		reloadCount++
		return nil
	}

	err = coordinator.RunAfterCommit(db, "reload failed", func(tx *gorm.DB) error {
		if tx == nil {
			t.Fatal("expected transaction")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reloadCount != 1 {
		t.Fatalf("expected reload once, got %d", reloadCount)
	}
}
