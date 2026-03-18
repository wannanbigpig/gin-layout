package data

import (
	"fmt"
	"sync"

	c "github.com/wannanbigpig/gin-layout/config"
)

var once sync.Once

// InitData 按配置初始化 MySQL 和 Redis 数据源。
func InitData() {
	once.Do(func() {
		cfg := c.GetConfig()
		if cfg.Mysql.Enable {
			if err := initMysql(); err != nil {
				panic(fmt.Sprintf("mysql init error: %v", err))
			}
		}

		if cfg.Redis.Enable {
			if err := initRedis(); err != nil {
				panic(fmt.Sprintf("redis init error: %v", err))
			}
		}
	})
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
