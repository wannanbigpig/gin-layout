package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/queue"
	auditsvc "github.com/wannanbigpig/gin-layout/internal/service/audit"
)

func TestNewAuditLogJobPayload(t *testing.T) {
	job, err := NewAuditLogJob(AuditLogKindRequest, &auditsvc.AuditLogSnapshot{RequestID: "req-1"})
	if err != nil {
		t.Fatalf("NewAuditLogJob returned error: %v", err)
	}

	payloadBytes, err := job.Payload()
	if err != nil {
		t.Fatalf("Payload returned error: %v", err)
	}

	var payload AuditLogPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if payload.Kind != AuditLogKindRequest {
		t.Fatalf("expected kind %q, got %q", AuditLogKindRequest, payload.Kind)
	}
	if payload.Snapshot == nil || payload.Snapshot.RequestID != "req-1" {
		t.Fatalf("unexpected snapshot: %#v", payload.Snapshot)
	}
}

func TestAuditLogHandlerReturnsSkipRetryForInvalidPayload(t *testing.T) {
	handler := AuditLogHandler{}
	err := handler.Handle(context.Background(), []byte(`{"kind":"invalid"}`))
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

	payload, err := json.Marshal(AuditLogPayload{
		Kind:     AuditLogKindPanic,
		Snapshot: &auditsvc.AuditLogSnapshot{RequestID: "req-2"},
	})
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}

	if err := (AuditLogHandler{}).Handle(context.Background(), payload); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if !called {
		t.Fatal("expected persist function to be called")
	}
}
