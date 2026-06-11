package taskcenter

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	ErrTaskLockHeld      = errors.New("task lock already held by another instance")
	ErrTaskLockRedisDown = errors.New("redis unavailable, cannot acquire task lock for high-risk task")
)

const (
	taskLockPrefix = "go-layout:task-lock:"
	taskLockTTL    = 30 * time.Minute
)

// TaskLock 分布式锁，用于保护高风险任务的跨实例互斥执行。
type TaskLock struct {
	client *redis.Client
	key    string
	token  string
}

// NewTaskLock 创建任务分布式锁。
func NewTaskLock(client *redis.Client, taskCode string) *TaskLock {
	return &TaskLock{
		client: client,
		key:    taskLockPrefix + taskCode,
		token:  uuid.New().String(),
	}
}

// Acquire 获取锁。Redis 不可用时返回 ErrTaskLockRedisDown。
func (l *TaskLock) Acquire(ctx context.Context) error {
	ok, err := l.client.SetNX(ctx, l.key, l.token, taskLockTTL).Result()
	if err != nil {
		return ErrTaskLockRedisDown
	}
	if !ok {
		return ErrTaskLockHeld
	}
	return nil
}

// Release 释放锁，仅当持有者匹配时。失败仅静默忽略，不影响业务。
func (l *TaskLock) Release(ctx context.Context) {
	script := redis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		end
		return 0
	`)
	_ = script.Run(ctx, l.client, []string{l.key}, l.token)
}
