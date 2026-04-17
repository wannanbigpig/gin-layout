package jobs

import (
	"context"
	"errors"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/queue"
	auditsvc "github.com/wannanbigpig/gin-layout/internal/service/audit"
)

func TestNewAuditLogPayload(t *testing.T) {
	payload, err := NewAuditLogPayload(AuditLogKindRequest, &auditsvc.AuditLogSnapshot{RequestID: "req-1"})
	if err != nil {
		t.Fatalf("NewAuditLogPayload returned error: %v", err)
	}
	if payload.Kind != AuditLogKindRequest {
		t.Fatalf("expected kind %q, got %q", AuditLogKindRequest, payload.Kind)
	}
	if payload.Snapshot == nil || payload.Snapshot.RequestID != "req-1" {
		t.Fatalf("unexpected snapshot: %#v", payload.Snapshot)
	}
}

func TestAuditLogHandlerReturnsSkipRetryForInvalidPayload(t *testing.T) {
	registry := queue.NewRegistry()
	RegisterAll(registry)

	entries := registry.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 registry entry, got %d", len(entries))
	}

	err := entries[0].Handler(context.Background(), []byte(`{"kind":"invalid"}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, queue.ErrSkipRetry) {
		t.Fatalf("expected skip retry error, got %v", err)
	}
}

func TestAuditLogHandlerPersistsSnapshot(t *testing.T) {
	original := persistAuditLogFunc
	defer func() {
		persistAuditLogFunc = original
	}()

	called := false
	persistAuditLogFunc = func(snapshot *auditsvc.AuditLogSnapshot) error {
		called = true
		if snapshot == nil || snapshot.RequestID != "req-2" {
			t.Fatalf("unexpected snapshot: %#v", snapshot)
		}
		return nil
	}

	registry := queue.NewRegistry()
	RegisterAll(registry)
	entries := registry.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 registry entry, got %d", len(entries))
	}

	payload, err := NewAuditLogPayload(AuditLogKindPanic, &auditsvc.AuditLogSnapshot{RequestID: "req-2"})
	if err != nil {
		t.Fatalf("NewAuditLogPayload returned error: %v", err)
	}

	job := queue.NewJSONJob(AuditLogTaskType, AuditQueueName, payload)
	raw, err := job.Payload()
	if err != nil {
		t.Fatalf("Payload returned error: %v", err)
	}

	if err := entries[0].Handler(context.Background(), raw); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if !called {
		t.Fatal("expected persist function to be called")
	}
}
