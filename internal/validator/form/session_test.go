package form

import "testing"

func TestSessionListRejectsInvalidIsRevoked(t *testing.T) {
	err := bindJSONBody(t, `{"is_revoked":2}`, NewSessionListQuery())
	if err == nil {
		t.Fatal("expected invalid is_revoked to fail validation")
	}
}

func TestSessionRevokeRejectsZeroID(t *testing.T) {
	err := bindJSONBody(t, `{"id":0}`, NewSessionRevokeForm())
	if err == nil {
		t.Fatal("expected zero id to fail validation")
	}
}
