package logger

import "testing"

func TestLoggerDefaultIsNotNil(t *testing.T) {
	if Logger == nil {
		t.Fatal("expected default logger to be non-nil")
	}
}

func TestLoggerWrappersDoNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("expected wrappers not to panic, got %v", r)
		}
	}()

	Info("info test")
	Warn("warn test")
	Error("error test")
}
