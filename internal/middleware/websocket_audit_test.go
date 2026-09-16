package middleware

import (
	"testing"
	"time"
)

func TestWebSocketAuditWaitsForHandlerCompletion(t *testing.T) {
	done := BeginWebSocketAudit()
	defer done()
	if WaitWebSocketAudits(time.Millisecond) {
		t.Fatal("audit drain must wait for the active handler")
	}
	done()
	done()
	if !WaitWebSocketAudits(time.Second) {
		t.Fatal("completed handler still blocks audit drain")
	}
}
