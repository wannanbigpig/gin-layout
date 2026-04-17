package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPingRoute(t *testing.T) {
	router := SetupRouter()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", recorder.Code)
	}
	if body := recorder.Body.String(); body != "pong" {
		t.Fatalf("unexpected response body: %s", body)
	}
}
