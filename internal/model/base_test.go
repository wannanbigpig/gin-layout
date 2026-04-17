package model

import (
	"errors"
	"testing"
)

func TestGetDBReturnsErrorWhenUninitialized(t *testing.T) {
	_, err := GetDB()
	if err == nil {
		t.Fatal("expected GetDB to return error when database is uninitialized")
	}
}

func TestGetDBRejectsTypedNilModelArg(t *testing.T) {
	var role *Role
	_, err := GetDB(role)
	if !errors.Is(err, ErrInvalidModelArg) {
		t.Fatalf("expected ErrInvalidModelArg, got %v", err)
	}
}

func TestListEReturnsErrorWhenUninitialized(t *testing.T) {
	_, err := ListE(NewApi(), "", nil)
	if err == nil {
		t.Fatal("expected ListE to return error when database is uninitialized")
	}
}

func TestListPageEReturnsErrorWhenUninitialized(t *testing.T) {
	_, _, err := ListPageE(NewApi(), 1, 10, "", nil)
	if err == nil {
		t.Fatal("expected ListPageE to return error when database is uninitialized")
	}
}

func TestBaseModelSelfReturnsErrorWithoutBinding(t *testing.T) {
	var role Role
	_, err := role.self()
	if !errors.Is(err, ErrModelPtrNotImplemented) {
		t.Fatalf("expected ErrModelPtrNotImplemented, got %v", err)
	}
}

func TestNewModelBindsOwner(t *testing.T) {
	role := NewRole()
	self, err := role.self()
	if err != nil {
		t.Fatalf("expected bound owner, got error %v", err)
	}
	if self != role {
		t.Fatalf("expected owner to point to role itself")
	}
}

func TestInstanceMethodReturnsBindingErrorBeforeDBError(t *testing.T) {
	var role Role
	err := role.GetById(1)
	if !errors.Is(err, ErrModelPtrNotImplemented) {
		t.Fatalf("expected ErrModelPtrNotImplemented, got %v", err)
	}
}

func TestCountReturnsBindingErrorBeforeDBError(t *testing.T) {
	var role Role
	_, err := role.Count("pid = ?", 1)
	if !errors.Is(err, ErrModelPtrNotImplemented) {
		t.Fatalf("expected ErrModelPtrNotImplemented, got %v", err)
	}
}

func TestBoundCountReturnsDBErrorWhenUninitialized(t *testing.T) {
	role := NewRole()
	_, err := role.Count("pid = ?", 1)
	if !errors.Is(err, ErrDBUninitialized) {
		t.Fatalf("expected ErrDBUninitialized, got %v", err)
	}
}
