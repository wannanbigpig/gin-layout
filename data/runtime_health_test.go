package data

import (
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	c "github.com/wannanbigpig/gin-layout/config"
)

func TestMysqlRuntimeStatusCachesProbeResult(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}

	restore := backupMysqlState()
	defer restore()

	mysqlDB = db
	mysqlValue.Store(mysqlSlot{db: db})
	mysqlInitError = nil
	mysqlHealth = newRuntimeHealthCache(time.Hour)

	probeCount := 0
	originalProbe := mysqlRuntimeProbe
	mysqlRuntimeProbe = func(current *gorm.DB) error {
		if current != db {
			t.Fatalf("unexpected db pointer")
		}
		probeCount++
		return nil
	}
	defer func() {
		mysqlRuntimeProbe = originalProbe
	}()

	status1 := MysqlRuntimeStatus()
	status2 := MysqlRuntimeStatus()

	if !status1.Ready || !status2.Ready {
		t.Fatalf("expected mysql runtime status to stay ready, got %+v %+v", status1, status2)
	}
	if probeCount != 1 {
		t.Fatalf("expected mysql probe to run once, got %d", probeCount)
	}
}

func TestMysqlRuntimeStatusReturnsProbeFailure(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}

	restore := backupMysqlState()
	defer restore()

	mysqlDB = db
	mysqlValue.Store(mysqlSlot{db: db})
	mysqlInitError = nil
	mysqlHealth = newRuntimeHealthCache(time.Hour)

	wantErr := errors.New("mysql down")
	originalProbe := mysqlRuntimeProbe
	mysqlRuntimeProbe = func(*gorm.DB) error { return wantErr }
	defer func() {
		mysqlRuntimeProbe = originalProbe
	}()

	status := MysqlRuntimeStatus()
	if status.Ready {
		t.Fatal("expected mysql runtime status to be not ready")
	}
	if !errors.Is(status.Error, wantErr) {
		t.Fatalf("expected error %v, got %v", wantErr, status.Error)
	}
	if MysqlReady() {
		t.Fatal("expected MysqlReady to be false")
	}
}

func TestRedisRuntimeStatusCachesProbeResult(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	defer client.Close()

	restore := backupRedisState()
	defer restore()

	redisDb = client
	redisValue.Store(redisSlot{client: client})
	redisInitError = nil
	redisHealth = newRuntimeHealthCache(time.Hour)

	probeCount := 0
	originalProbe := redisRuntimeProbe
	redisRuntimeProbe = func(current *redis.Client) error {
		if current != client {
			t.Fatalf("unexpected redis client pointer")
		}
		probeCount++
		return nil
	}
	defer func() {
		redisRuntimeProbe = originalProbe
	}()

	status1 := RedisRuntimeStatus()
	status2 := RedisRuntimeStatus()

	if !status1.Ready || !status2.Ready {
		t.Fatalf("expected redis runtime status to stay ready, got %+v %+v", status1, status2)
	}
	if probeCount != 1 {
		t.Fatalf("expected redis probe to run once, got %d", probeCount)
	}
}

func TestRedisRuntimeStatusReturnsProbeFailure(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	defer client.Close()

	restore := backupRedisState()
	defer restore()

	redisDb = client
	redisValue.Store(redisSlot{client: client})
	redisInitError = nil
	redisHealth = newRuntimeHealthCache(time.Hour)

	wantErr := errors.New("redis down")
	originalProbe := redisRuntimeProbe
	redisRuntimeProbe = func(*redis.Client) error { return wantErr }
	defer func() {
		redisRuntimeProbe = originalProbe
	}()

	status := RedisRuntimeStatus()
	if status.Ready {
		t.Fatal("expected redis runtime status to be not ready")
	}
	if !errors.Is(status.Error, wantErr) {
		t.Fatalf("expected error %v, got %v", wantErr, status.Error)
	}
	if RedisReady() {
		t.Fatal("expected RedisReady to be false")
	}
}

func TestMysqlDBRetriesAfterInitFailure(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}

	restore := backupMysqlState()
	defer restore()

	mysqlDB = nil
	mysqlValue.Store(mysqlSlot{})
	mysqlInitError = nil
	mysqlHealth = newRuntimeHealthCache(time.Hour)

	wantErr := errors.New("mysql init failed")
	attempts := 0
	openMysqlFunc = func(_ *c.Conf) (*gorm.DB, error) {
		attempts++
		if attempts == 1 {
			return nil, wantErr
		}
		return db, nil
	}

	if got := MysqlDB(); got != nil {
		t.Fatalf("expected first init to fail, got db %#v", got)
	}
	if !errors.Is(MysqlInitError(), wantErr) {
		t.Fatalf("expected stored init error %v, got %v", wantErr, MysqlInitError())
	}

	if got := MysqlDB(); got != db {
		t.Fatalf("expected retry to store db, got %#v", got)
	}
	if MysqlInitError() != nil {
		t.Fatalf("expected init error to be cleared, got %v", MysqlInitError())
	}
	if attempts != 2 {
		t.Fatalf("expected 2 init attempts, got %d", attempts)
	}
}

func TestRedisClientRetriesAfterInitFailure(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"})
	defer client.Close()

	restore := backupRedisState()
	defer restore()

	redisDb = nil
	redisValue.Store(redisSlot{})
	redisInitError = nil
	redisHealth = newRuntimeHealthCache(time.Hour)

	wantErr := errors.New("redis init failed")
	attempts := 0
	openRedisFunc = func(_ *c.Conf) (*redis.Client, error) {
		attempts++
		if attempts == 1 {
			return nil, wantErr
		}
		return client, nil
	}

	if got := RedisClient(); got != nil {
		t.Fatalf("expected first init to fail, got client %#v", got)
	}
	if !errors.Is(GetRedisInitError(), wantErr) {
		t.Fatalf("expected stored init error %v, got %v", wantErr, GetRedisInitError())
	}

	if got := RedisClient(); got != client {
		t.Fatalf("expected retry to store redis client, got %#v", got)
	}
	if GetRedisInitError() != nil {
		t.Fatalf("expected init error to be cleared, got %v", GetRedisInitError())
	}
	if attempts != 2 {
		t.Fatalf("expected 2 init attempts, got %d", attempts)
	}
}

func backupMysqlState() func() {
	previousDB := mysqlDB
	previousInitErr := mysqlInitError
	previousHealth := mysqlHealth
	previousOpen := openMysqlFunc
	var previousSlot mysqlSlot
	if slot, ok := mysqlValue.Load().(mysqlSlot); ok {
		previousSlot = slot
	}
	return func() {
		mysqlDB = previousDB
		mysqlInitError = previousInitErr
		mysqlHealth = previousHealth
		openMysqlFunc = previousOpen
		mysqlValue.Store(previousSlot)
	}
}

func backupRedisState() func() {
	previousClient := redisDb
	previousInitErr := redisInitError
	previousHealth := redisHealth
	previousOpen := openRedisFunc
	var previousSlot redisSlot
	if slot, ok := redisValue.Load().(redisSlot); ok {
		previousSlot = slot
	}
	return func() {
		redisDb = previousClient
		redisInitError = previousInitErr
		redisHealth = previousHealth
		openRedisFunc = previousOpen
		redisValue.Store(previousSlot)
	}
}
