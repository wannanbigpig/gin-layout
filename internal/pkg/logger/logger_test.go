package logger

import (
	"testing"

	"go.uber.org/zap/zapcore"
)

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

func TestReloadLoggerCoreClosesPreviousWriter(t *testing.T) {
	oldCloser := &closeRecorder{}
	previousCore, previousCloser := swapLoggerCore(zapcore.NewNopCore(), oldCloser)
	t.Cleanup(func() {
		_, currentCloser := swapLoggerCore(previousCore, previousCloser)
		closeLoggerWriter(currentCloser)
	})

	reloadLoggerCore(zapcore.NewNopCore(), &closeRecorder{})

	if !oldCloser.closed {
		t.Fatal("expected previous writer to be closed")
	}
}

type closeRecorder struct {
	closed bool
}

func (c *closeRecorder) Close() error {
	c.closed = true
	return nil
}
