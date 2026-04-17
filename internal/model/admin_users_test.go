package model

import (
	"errors"
	"testing"
)

func TestExistsWithLockExcludeIdRejectsUnknownField(t *testing.T) {
	adminUser := NewAdminUsers()
	_, err := adminUser.ExistsWithLockExcludeId("status", "1", 1)
	if err == nil {
		t.Fatal("expected unknown field to return error")
	}
}

func TestExistsWithLockExcludeIdAllowedFieldReturnsDBErrorWhenUninitialized(t *testing.T) {
	adminUser := NewAdminUsers()
	_, err := adminUser.ExistsWithLockExcludeId("username", "tester", 1)
	if !errors.Is(err, ErrDBUninitialized) {
		t.Fatalf("expected ErrDBUninitialized, got %v", err)
	}
}
