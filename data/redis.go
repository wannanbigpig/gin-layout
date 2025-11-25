package data

import (
	"context"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"

	c "github.com/wannanbigpig/gin-layout/config"
)

var (
	redisDb        *redis.Client
	redisOnce      sync.Once
	redisInitError error
)

func initRedis() error {
	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	redisDb = redis.NewClient(&redis.Options{
		Addr:     c.Config.Redis.Host + ":" + c.Config.Redis.Port,
		Password: c.Config.Redis.Password,
		DB:       c.Config.Redis.Database,
	})

	_, err := redisDb.Ping(ctx).Result()
	if err != nil {
		_ = redisDb.Close() // 忽略关闭错误，但确保资源释放
		redisDb = nil
		return err
	}
	return nil
}

// RedisClient 返回 Redis 客户端和初始化错误
func RedisClient() *redis.Client {
	if redisDb == nil {
		redisOnce.Do(func() {
			redisInitError = initRedis()
		})
	}
	return redisDb
}

// GetRedisInitError 返回 Redis 初始化错误，供外部检查
func GetRedisInitError() error {
	return redisInitError
}
