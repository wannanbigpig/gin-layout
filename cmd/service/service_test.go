package service

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewHTTPServerUsesDefensiveDefaults(t *testing.T) {
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	server := newHTTPServer("127.0.0.1:9001", handler)

	if server.Addr != "127.0.0.1:9001" {
		t.Fatalf("unexpected server address: %q", server.Addr)
	}
	if server.Handler == nil {
		t.Fatal("expected server handler")
	}
	if server.ReadHeaderTimeout != defaultReadHeaderTimeout {
		t.Fatalf("unexpected read header timeout: %s", server.ReadHeaderTimeout)
	}
	if server.IdleTimeout != defaultIdleTimeout {
		t.Fatalf("unexpected idle timeout: %s", server.IdleTimeout)
	}
	if server.MaxHeaderBytes != defaultMaxHeaderBytes {
		t.Fatalf("unexpected max header bytes: %d", server.MaxHeaderBytes)
	}
	if server.MaxHeaderValueCount != defaultMaxHeaderValueCount {
		t.Fatalf("unexpected max header value count: %d", server.MaxHeaderValueCount)
	}
}

func TestHTTPRequestLifecycleWaitsForActiveHandlerAndRejectsLateRequest(t *testing.T) {
	lifecycle := newHTTPRequestLifecycle()
	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	var calls atomic.Int32
	handler := lifecycle.wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
		close(started)
		<-release
	}))

	go func() {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/slow", nil))
		close(finished)
	}()
	<-started
	if drained, remaining := lifecycle.drainAndWait(10 * time.Millisecond); drained || remaining != 1 {
		t.Fatalf("expected one active handler after timeout, drained=%v remaining=%d", drained, remaining)
	}

	late := httptest.NewRecorder()
	handler.ServeHTTP(late, httptest.NewRequest(http.MethodGet, "/late", nil))
	if late.Code != http.StatusServiceUnavailable || calls.Load() != 1 {
		t.Fatalf("late request was not rejected: status=%d calls=%d", late.Code, calls.Load())
	}

	close(release)
	<-finished
	if drained, remaining := lifecycle.drainAndWait(time.Second); !drained || remaining != 0 {
		t.Fatalf("completed handler still blocks drain: drained=%v remaining=%d", drained, remaining)
	}
}
