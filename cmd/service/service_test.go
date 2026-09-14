package service

import (
	"net/http"
	"testing"
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
