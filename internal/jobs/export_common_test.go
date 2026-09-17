package jobs

import "testing"

func TestIsFinalExportAttempt(t *testing.T) {
	tests := []struct {
		name       string
		retryCount int
		maxRetry   int
		hasMeta    bool
		want       bool
	}{
		{name: "direct invocation fails closed", hasMeta: false, want: true},
		{name: "initial attempt will retry", retryCount: 0, maxRetry: 2, hasMeta: true, want: false},
		{name: "first retry will retry", retryCount: 1, maxRetry: 2, hasMeta: true, want: false},
		{name: "last attempt is terminal", retryCount: 2, maxRetry: 2, hasMeta: true, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isFinalExportAttempt(tt.retryCount, tt.maxRetry, tt.hasMeta); got != tt.want {
				t.Fatalf("isFinalExportAttempt()=%v want=%v", got, tt.want)
			}
		})
	}
}
