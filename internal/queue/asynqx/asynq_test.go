package asynqx

import (
	"testing"
	"time"

	"github.com/hibiken/asynq"

	"github.com/wannanbigpig/gin-layout/internal/queue"
)

func TestPrefixedQueueName(t *testing.T) {
	if got := prefixedQueueName("go_layout", "audit"); got != "go_layout:audit" {
		t.Fatalf("unexpected prefixed queue: %s", got)
	}
	if got := unprefixQueueName("go_layout", "go_layout:audit"); got != "audit" {
		t.Fatalf("unexpected unprefixed queue: %s", got)
	}
	if got := prefixedQueueName("", "audit"); got != "audit" {
		t.Fatalf("unexpected queue without namespace: %s", got)
	}
}

func TestMapOptions(t *testing.T) {
	options := mapOptions("go_layout", []queue.JobOption{
		queue.WithMaxRetry(3),
		queue.WithQueue("audit"),
		queue.WithTimeout(10 * time.Second),
		queue.WithTaskID("task-1"),
	})
	if len(options) != 4 {
		t.Fatalf("expected 4 options, got %d", len(options))
	}

	assertOption := func(index int, wantType asynq.OptionType, wantValue any) {
		if options[index].Type() != wantType {
			t.Fatalf("option %d type mismatch: got %v want %v", index, options[index].Type(), wantType)
		}
		if options[index].Value() != wantValue {
			t.Fatalf("option %d value mismatch: got %#v want %#v", index, options[index].Value(), wantValue)
		}
	}

	assertOption(0, asynq.MaxRetryOpt, 3)
	assertOption(1, asynq.QueueOpt, "go_layout:audit")
	assertOption(2, asynq.TimeoutOpt, 10*time.Second)
	assertOption(3, asynq.TaskIDOpt, "task-1")
}
