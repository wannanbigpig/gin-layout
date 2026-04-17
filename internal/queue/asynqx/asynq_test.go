package asynqx

import (
	"testing"
	"time"

	"github.com/hibiken/asynq"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/config/autoload"
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

func TestNewRedisConnOptUsesDefaultRedis(t *testing.T) {
	cfg := &config.Conf{
		Redis: autoload.RedisConfig{
			Enable:   true,
			Host:     "127.0.0.1",
			Port:     "6380",
			Password: "default-pass",
			Database: 3,
		},
		Queue: autoload.QueueConfig{
			Enable:          true,
			UseDefaultRedis: true,
		},
	}

	opt, err := newRedisConnOpt(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if opt.Addr != "127.0.0.1:6380" {
		t.Fatalf("unexpected addr: %s", opt.Addr)
	}
	if opt.Password != "default-pass" || opt.DB != 3 {
		t.Fatalf("unexpected redis option: %+v", opt)
	}
}

func TestNewRedisConnOptUsesQueueRedis(t *testing.T) {
	cfg := &config.Conf{
		Redis: autoload.RedisConfig{
			Enable: false,
		},
		Queue: autoload.QueueConfig{
			Enable:          true,
			UseDefaultRedis: false,
			Redis: autoload.QueueRedisConfig{
				Host:     "10.0.0.8",
				Port:     "6381",
				Password: "queue-pass",
				Database: 6,
			},
		},
	}

	opt, err := newRedisConnOpt(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if opt.Addr != "10.0.0.8:6381" {
		t.Fatalf("unexpected addr: %s", opt.Addr)
	}
	if opt.Password != "queue-pass" || opt.DB != 6 {
		t.Fatalf("unexpected redis option: %+v", opt)
	}
}

func TestNewRedisConnOptReturnsErrorWhenDefaultRedisDisabled(t *testing.T) {
	cfg := &config.Conf{
		Redis: autoload.RedisConfig{
			Enable: false,
			Host:   "127.0.0.1",
			Port:   "6379",
		},
		Queue: autoload.QueueConfig{
			Enable:          true,
			UseDefaultRedis: true,
		},
	}

	if _, err := newRedisConnOpt(cfg); err == nil {
		t.Fatal("expected error when default redis is disabled")
	}
}
