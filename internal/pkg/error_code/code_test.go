package error_code

import "testing"

func TestName(t *testing.T) {
	if "OK" != Text(0) {
		t.Error("error")
	}

	if "unknown error" != Text(1032323) {
		t.Error("error")
	}
}
