package data

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"

	c "github.com/wannanbigpig/gin-layout/config"
)

var (
	redisDb        *redis.Client
	redisOnce      sync.Once
	redisInitError error
	redisValue     atomic.Value
	redisMu        sync.Mutex
	redisHealth    = newRuntimeHealthCache(defaultRuntimeHealthTTL)
)

type redisSlot struct {
	client *redis.Client
}

const redisProbeTimeout = 2 * time.Second

var redisRuntimeProbe = func(client *redis.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), redisProbeTimeout)
	defer cancel()
	return client.Ping(ctx).Err()
}

func initRedis() error {
	return reloadRedis(c.GetConfig())
}

func reloadRedis(cfg *c.Conf) error {
	redisMu.Lock()
	defer redisMu.Unlock()

	next, err := openRedis(cfg)
	if err != nil {
		return err
	}
	old := currentRedis()
	redisDb = next
	redisValue.Store(redisSlot{client: next})
	redisInitError = nil
	if next != nil {
		redisHealth.SeedReady()
	} else {
		redisHealth.Reset()
	}
	if old != nil {
		_ = old.Close()
	}
	return nil
}

func openRedis(cfg *c.Conf) (*redis.Client, error) {
	if cfg == nil || !cfg.Redis.Enable {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := &redis.Options{
		Addr:            cfg.Redis.Host + ":" + cfg.Redis.Port,
		Password:        cfg.Redis.Password,
		DB:              cfg.Redis.Database,
		PoolSize:        cfg.Redis.PoolSize,
		MinIdleConns:    cfg.Redis.MinIdleConns,
		ConnMaxLifetime: cfg.Redis.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.Redis.ConnMaxIdle,
		ReadTimeout:     cfg.Redis.ReadTimeout,
		WriteTimeout:    cfg.Redis.WriteTimeout,
	}

	client := redis.NewClient(opts)

	_, err := client.Ping(ctx).Result()
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

// RedisClient 返回 Redis 客户端和初始化错误
func RedisClient() *redis.Client {
	if client := currentRedis(); client != nil {
		return client
	}
	if redisDb == nil {
		redisOnce.Do(func() {
			redisInitError = initRedis()
		})
	}
	return currentRedis()
}

// GetRedisInitError 返回 Redis 初始化错误，供外部检查
func GetRedisInitError() error {
	return redisInitError
}

// RedisRuntimeStatus 返回带缓存的 Redis 运行时健康探测结果。
func RedisRuntimeStatus() RuntimeHealthStatus {
	client := RedisClient()
	if client == nil {
		redisHealth.Reset()
		return RuntimeHealthStatus{
			Ready:     false,
			Error:     redisUnavailableError(),
			CheckedAt: time.Now(),
		}
	}
	status := redisHealth.Check(func() error {
		return redisRuntimeProbe(client)
	})
	if !status.Ready && status.Error == nil {
		status.Error = redisUnavailableError()
	}
	return status
}

// RedisReady 判断 Redis 当前是否可用。
func RedisReady() bool {
	return RedisRuntimeStatus().Ready
}

// ReloadRedis 重新加载 Redis 客户端。
func ReloadRedis(cfg *c.Conf) error {
	return reloadRedis(cfg)
}

// CloseRedis 关闭当前 Redis 客户端。
func CloseRedis() error {
	redisMu.Lock()
	defer redisMu.Unlock()

	current := currentRedis()
	redisDb = nil
	redisValue.Store(redisSlot{})
	redisInitError = nil
	redisHealth.Reset()
	if current == nil {
		return nil
	}
	return current.Close()
}

func currentRedis() *redis.Client {
	if slot, ok := redisValue.Load().(redisSlot); ok {
		return slot.client
	}
	return redisDb
}

func redisUnavailableError() error {
	if redisInitError != nil {
		return redisInitError
	}
	return ErrRedisUnavailable
}

// ErrRedisUnavailable 表示 Redis 客户端当前不可用。
var ErrRedisUnavailable = errors.New("redis client is unavailable")
