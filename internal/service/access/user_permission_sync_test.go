package access

import (
	"errors"
	"reflect"
	"testing"
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
