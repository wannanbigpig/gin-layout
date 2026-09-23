package limiter

import (
	"context"
)

const (
	DriverMemory = "memory"
	DriverRedis  = "redis"
)

// Driver 定义通用速率限制驱动接口。
type Driver interface {
	// Allow 判定指定 key 是否允许消耗 1 个令牌。
	Allow(ctx context.Context, key string, rps float64, burst int) (bool, error)
}
