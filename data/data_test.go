package data

import "testing"

func TestShutdownWithoutInitializedResources(t *testing.T) {
	if err := Shutdown(); err != nil {
		t.Fatalf("shutdown should be safe without initialized resources: %v", err)
	}
	if err := Shutdown(); err != nil {
		t.Fatalf("shutdown should be idempotent: %v", err)
	}
}
