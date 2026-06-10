package asynqx

import (
	"errors"
	"testing"
	"time"

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

func TestConvertQueueInfo(t *testing.T) {
	info := &asynq.QueueInfo{
		Queue:     "go_layout:default",
		Size:      20,
		Pending:   5,
		Active:    3,
		Scheduled: 4,
		Retry:     2,
		Archived:  1,
		Completed: 10,
		Processed: 100,
		Failed:    5,
		Paused:    false,
		Timestamp: time.Now(),
	}

	stats := convertQueueInfo(info, "go_layout")
	if stats.Queue != "default" {
		t.Fatalf("expected queue name 'default', got %q", stats.Queue)
	}
	if stats.Pending != 5 || stats.Active != 3 || stats.Retry != 2 {
		t.Fatalf("unexpected stats: pending=%d active=%d retry=%d", stats.Pending, stats.Active, stats.Retry)
	}
	if stats.Size != 20 || stats.Completed != 10 || stats.Processed != 100 || stats.Failed != 5 {
		t.Fatalf("unexpected stats: size=%d completed=%d processed=%d failed=%d", stats.Size, stats.Completed, stats.Processed, stats.Failed)
	}
	if stats.Paused {
		t.Fatal("expected paused=false")
	}
}

func TestConvertQueueInfoWithoutNamespace(t *testing.T) {
	info := &asynq.QueueInfo{
		Queue: "default",
		Size:  1,
	}

	stats := convertQueueInfo(info, "")
	if stats.Queue != "default" {
		t.Fatalf("expected queue name 'default', got %q", stats.Queue)
	}
}

func TestInspectorGetQueueStatsReturnsUnavailableWhenNil(t *testing.T) {
	var i *inspector
	_, err := i.GetQueueStats(nil, "default")
	if !errors.Is(err, queue.ErrInspectorUnavailable) {
		t.Fatalf("expected ErrInspectorUnavailable, got %v", err)
	}
}

func TestInspectorGetAllQueueStatsReturnsUnavailableWhenNil(t *testing.T) {
	var i *inspector
	_, err := i.GetAllQueueStats(nil)
	if !errors.Is(err, queue.ErrInspectorUnavailable) {
		t.Fatalf("expected ErrInspectorUnavailable, got %v", err)
	}
}
