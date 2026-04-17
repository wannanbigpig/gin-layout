package data

import (
	"errors"
	"fmt"
	"sync"

	c "github.com/wannanbigpig/gin-layout/config"
)

var once sync.Once
var initErr error

// InitData 按配置初始化 MySQL 和 Redis 数据源。
func InitData() error {
	once.Do(func() {
		cfg := c.GetConfig()
		var errs []error

		if cfg.Mysql.Enable {
			if err := initMysql(); err != nil {
				errs = append(errs, fmt.Errorf("mysql init error: %w", err))
			}
		}

		if cfg.Redis.Enable {
			if err := initRedis(); err != nil {
				errs = append(errs, fmt.Errorf("redis init error: %w", err))
			}
		}

		if len(errs) > 0 {
			initErr = errors.Join(errs...)
		}
	})

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
