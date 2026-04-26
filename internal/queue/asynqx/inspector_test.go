package asynqx

import (
	"errors"
	"testing"

	"github.com/hibiken/asynq"

	"github.com/wannanbigpig/gin-layout/internal/queue"
)

func TestNormalizeInspectorError(t *testing.T) {
	cases := []struct {
		name string
		in   error
		want error
	}{
		{name: "queue not found", in: asynq.ErrQueueNotFound, want: queue.ErrQueueNotFound},
		{name: "task not found", in: asynq.ErrTaskNotFound, want: queue.ErrTaskNotFound},
		{name: "other error", in: errors.New("boom"), want: errors.New("boom")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeInspectorError(tc.in)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			if tc.name == "other error" {
				if got == nil || got.Error() != "boom" {
					t.Fatalf("unexpected error: %v", got)
				}
				return
			}
			if !errors.Is(got, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}
