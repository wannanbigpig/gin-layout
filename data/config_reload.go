package data

import (
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/wannanbigpig/gin-layout/config"
	"gorm.io/gorm"
)

// PreparedReload 持有尚未发布的连接；准备失败不能影响当前实例。
type PreparedReload struct {
	mysqlChanged, redisChanged   bool
	mysqlDisabled, redisDisabled bool
	mysql, previousMysql         *gorm.DB
	redis, previousRedis         *redis.Client
}

func PrepareReload(cfg *config.Conf, diff config.ConfigDiff) (*PreparedReload, error) {
	p := &PreparedReload{
		mysqlChanged: diff.MysqlChanged, redisChanged: diff.RedisChanged,
		mysqlDisabled: cfg == nil || !cfg.Mysql.Enable,
		redisDisabled: cfg == nil || !cfg.Redis.Enable,
	}
	var err error
	if p.mysqlChanged {
		p.mysql, err = openMysqlFunc(cfg)
		if err != nil {
			p.Abort()
			return nil, fmt.Errorf("mysql prepare failed: %w", err)
		}
	}
	if p.redisChanged {
		p.redis, err = openRedisFunc(cfg)
		if err != nil {
			p.Abort()
			return nil, fmt.Errorf("redis prepare failed: %w", err)
		}
	}
	return p, nil
}

// MysqlDB 返回候选连接，供权限快照在提交前完成校验。
func (p *PreparedReload) MysqlDB() *gorm.DB {
	if p.mysqlChanged {
		return p.mysql
	}
	return currentMysql()
}

func (p *PreparedReload) Commit() {
	if p.mysqlChanged {
		mysqlMu.Lock()
		p.previousMysql = currentMysql()
		mysqlDB = p.mysql
		mysqlValue.Store(mysqlSlot{db: p.mysql, disabled: p.mysqlDisabled})
		mysqlInitError = nil
		if p.mysql != nil {
			mysqlHealth.SeedReady()
		} else {
			mysqlHealth.Reset()
		}
		mysqlMu.Unlock()
	}
	if p.redisChanged {
		redisMu.Lock()
		p.previousRedis = currentRedis()
		redisDb = p.redis
		redisValue.Store(redisSlot{client: p.redis, disabled: p.redisDisabled})
		redisInitError = nil
		if p.redis != nil {
			redisHealth.SeedReady()
		} else {
			redisHealth.Reset()
		}
		redisMu.Unlock()
	}
}

func (p *PreparedReload) Abort() {
	closePreparedMysql(p.mysql)
	if p.redis != nil {
		_ = p.redis.Close()
	}
}

// Finalize 只在配置与所有依赖均已发布后关闭旧连接。
func (p *PreparedReload) Finalize() {
	if p.previousMysql != p.mysql {
		closePreparedMysql(p.previousMysql)
	}
	if p.previousRedis != nil && p.previousRedis != p.redis {
		_ = p.previousRedis.Close()
	}
}

func closePreparedMysql(db *gorm.DB) {
	if conn := getSQLDB(db); conn != nil {
		_ = conn.Close()
	}
}
