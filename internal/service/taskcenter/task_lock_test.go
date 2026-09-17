package taskcenter

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestTaskLockReleaseReturnsRedisError(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:0",
		DialTimeout:  10 * time.Millisecond,
		ReadTimeout:  10 * time.Millisecond,
		WriteTimeout: 10 * time.Millisecond,
		MaxRetries:   -1,
	})
	defer client.Close()

	lock := NewTaskLock(client, "cron:test")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := lock.Release(ctx); err == nil {
		t.Fatal("expected Redis release error to be returned")
	}
}
