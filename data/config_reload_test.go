package data

import (
	"errors"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/wannanbigpig/gin-layout/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func reloadTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closePreparedMysql(db) })
	return db
}

func TestPrepareReloadFailureKeepsLiveMysqlAndClosesCandidate(t *testing.T) {
	restore := backupMysqlState()
	defer restore()
	old, next := reloadTestDB(t), reloadTestDB(t)
	mysqlDB = old
	mysqlValue.Store(mysqlSlot{db: old})
	oldOpenMysql, oldOpenRedis := openMysqlFunc, openRedisFunc
	defer func() { openMysqlFunc, openRedisFunc = oldOpenMysql, oldOpenRedis }()
	openMysqlFunc = func(*config.Conf) (*gorm.DB, error) { return next, nil }
	openRedisFunc = func(*config.Conf) (*redis.Client, error) { return nil, errors.New("unavailable") }
	if _, err := PrepareReload(&config.Conf{}, config.ConfigDiff{MysqlChanged: true, RedisChanged: true}); err == nil {
		t.Fatal("expected Redis preparation error")
	}
	if currentMysql() != old {
		t.Fatal("failed preparation published candidate MySQL")
	}
	if err := getSQLDB(old).Ping(); err != nil {
		t.Fatal("previous MySQL was closed", err)
	}
	if err := getSQLDB(next).Ping(); err == nil {
		t.Fatal("failed reload leaked candidate MySQL")
	}
}

func TestPreparedReloadRetainsOldConnectionUntilFinalized(t *testing.T) {
	restore := backupMysqlState()
	defer restore()
	old, next := reloadTestDB(t), reloadTestDB(t)
	mysqlDB = old
	mysqlValue.Store(mysqlSlot{db: old})
	previous := openMysqlFunc
	defer func() { openMysqlFunc = previous }()
	openMysqlFunc = func(*config.Conf) (*gorm.DB, error) { return next, nil }
	p, err := PrepareReload(&config.Conf{}, config.ConfigDiff{MysqlChanged: true})
	if err != nil {
		t.Fatal(err)
	}
	if currentMysql() != old {
		t.Fatal("preparation changed live MySQL")
	}
	p.Commit()
	if currentMysql() != next {
		t.Fatal("commit did not publish candidate")
	}
	if err := getSQLDB(old).Ping(); err != nil {
		t.Fatal("old resource closed before finalize")
	}
	p.Finalize()
	if err := getSQLDB(old).Ping(); err == nil {
		t.Fatal("old resource not closed after finalize")
	}
}

func TestDisabledReloadBlocksLazyInitializationWithOldConfig(t *testing.T) {
	for _, mode := range []string{"prepared", "direct"} {
		t.Run(mode, func(t *testing.T) {
			defer backupMysqlState()()
			defer backupRedisState()()
			defer config.UpdateConfigForTesting(func(cfg *config.Conf) { cfg.Mysql.Enable = true; cfg.Redis.Enable = true })()
			oldConfig := config.GetConfig()
			oldDB := reloadTestDB(t)
			oldRedis := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
			defer oldRedis.Close()
			mysqlDB = oldDB
			mysqlValue.Store(mysqlSlot{db: oldDB})
			redisDb = oldRedis
			redisValue.Store(redisSlot{client: oldRedis})
			previousMysql, previousRedis := openMysqlFunc, openRedisFunc
			defer func() { openMysqlFunc, openRedisFunc = previousMysql, previousRedis }()
			mysqlOpens, redisOpens := 0, 0
			openMysqlFunc = func(cfg *config.Conf) (*gorm.DB, error) {
				if cfg.Mysql.Enable {
					mysqlOpens++
					return oldDB, nil
				}
				return nil, nil
			}
			openRedisFunc = func(cfg *config.Conf) (*redis.Client, error) {
				if cfg.Redis.Enable {
					redisOpens++
					return oldRedis, nil
				}
				return nil, nil
			}
			disabled := &config.Conf{}
			if mode == "prepared" {
				p, err := PrepareReload(disabled, config.ConfigDiff{MysqlChanged: true, RedisChanged: true})
				if err != nil {
					t.Fatal(err)
				}
				p.Commit()
				defer p.Finalize()
			} else {
				if err := ReloadMysql(disabled); err != nil {
					t.Fatal(err)
				}
				if err := ReloadRedis(disabled); err != nil {
					t.Fatal(err)
				}
			}
			// The config snapshot intentionally still says enabled. A caller may also
			// have captured this snapshot before the resource commit.
			if MysqlDB() != nil || RedisClient() != nil {
				t.Error("disabled resource was revived by lazy getter")
			}
			if err := ensureMysqlInitialized(oldConfig); err != nil {
				t.Fatal(err)
			}
			if err := ensureRedisInitialized(oldConfig); err != nil {
				t.Fatal(err)
			}
			if mysqlOpens != 0 || redisOpens != 0 {
				t.Fatalf("old config reopened resources: mysql=%d redis=%d", mysqlOpens, redisOpens)
			}
			if currentMysql() != nil || currentRedis() != nil {
				t.Fatal("disabled resources must remain nil")
			}
		})
	}
}
