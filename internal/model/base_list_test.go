package model

import (
	"strings"
	"testing"
)

func TestNormalizeOrderByAcceptsValidFields(t *testing.T) {
	orderBy, err := normalizeOrderBy("sort desc, id asc", nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if orderBy != "sort DESC, id ASC" {
		t.Fatalf("unexpected normalized order by: %s", orderBy)
	}
}

func TestNormalizeOrderByRejectsInjection(t *testing.T) {
	_, err := normalizeOrderBy("id desc; drop table admin_user", nil)
	if err == nil {
		t.Fatal("expected invalid order by to return error")
	}
}

func TestNormalizeOrderByChecksAllowList(t *testing.T) {
	allowed := map[string]struct{}{
		"id": {},
	}
	_, err := normalizeOrderBy("created_at desc", allowed)
	if err == nil {
		t.Fatal("expected order field allow-list error")
	}
	if !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("expected not allowed error, got %v", err)
	}
}

func TestNormalizeSelectFieldsAcceptsValidFields(t *testing.T) {
	fields, err := normalizeSelectFields("id, created_at, admin_user.id")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if fields != "id, created_at, admin_user.id" {
		t.Fatalf("unexpected normalized fields: %s", fields)
	}
}

func TestNormalizeSelectFieldsRejectsInjection(t *testing.T) {
	_, err := normalizeSelectFields("id;drop table admin_user")
	if err == nil {
		t.Fatal("expected invalid select fields to return error")
	}
}
