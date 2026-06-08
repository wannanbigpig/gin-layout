package data

import (
	"errors"
	"fmt"
	"sync"

	c "github.com/wannanbigpig/gin-layout/config"
)

var (
	initMu  sync.Mutex
	initErr error
)

// InitData 按配置初始化 MySQL 和 Redis 数据源。
func InitData() error {
	initMu.Lock()
	defer initMu.Unlock()

	cfg := c.GetConfig()
	var errs []error

	if cfg.Mysql.Enable {
		if err := ensureMysqlInitialized(cfg); err != nil {
			errs = append(errs, fmt.Errorf("mysql init error: %w", err))
		}
	}

	if cfg.Redis.Enable {
		if err := ensureRedisInitialized(cfg); err != nil {
			errs = append(errs, fmt.Errorf("redis init error: %w", err))
		}
	}

	if len(errs) > 0 {
		initErr = errors.Join(errs...)
		return initErr
	}

	initErr = nil
	return initErr
}

// Shutdown 关闭当前已初始化的数据源。
func Shutdown() error {
	var firstErr error

	if err := CloseRedis(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := CloseMysql(); err != nil && firstErr == nil {
		firstErr = err
	}

	return firstErr
}
