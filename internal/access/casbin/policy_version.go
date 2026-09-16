package casbinx

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/casbin/casbin/v3"
	casbinmodel "github.com/casbin/casbin/v3/model"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/pkg/redisutil"
)

const (
	policyVersionRowID         = 1
	policyVersionCacheTTL      = 30 * time.Second
	policyVersionDirtyTTL      = 5 * time.Minute
	policyVersionRedisTimeout  = 100 * time.Millisecond
	policyVersionRedisCooldown = time.Second
)

type permissionPolicyVersion struct {
	ID        uint8     `gorm:"column:id;primaryKey"`
	Version   uint64    `gorm:"column:version"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (permissionPolicyVersion) TableName() string { return "permission_policy_versions" }

type policyVersionAccess interface {
	Current(context.Context) (uint64, error)
	LoadSnapshot(context.Context, casbinmodel.Model) (*casbin.Enforcer, uint64, error)
	RecordLoaded(context.Context, uint64) error
}

type policyVersionStore struct {
	dbProvider    func() *gorm.DB
	redisProvider func() *redis.Client
	cacheTTL      time.Duration
	dirtyTTL      time.Duration
	dbReads       singleflight.Group
	redisRetryAt  atomic.Int64
}

var defaultPolicyVersionStore = &policyVersionStore{
	dbProvider:    data.MysqlDB,
	redisProvider: data.RedisClient,
	cacheTTL:      policyVersionCacheTTL,
	dirtyTTL:      policyVersionDirtyTTL,
}

var readPolicyVersionScript = redis.NewScript(`
local dirty = redis.call('GET', KEYS[1])
if dirty then
  return {dirty, ''}
end
local version = redis.call('GET', KEYS[2])
if version then
  return {'', version}
end
return {'', ''}
`)

var recordPolicyVersionScript = redis.NewScript(`
local incoming = tonumber(ARGV[1])
local current = tonumber(redis.call('GET', KEYS[2]) or '0')
if incoming >= current then
  redis.call('SET', KEYS[2], ARGV[1], 'PX', ARGV[2])
end
local dirty = tonumber(redis.call('GET', KEYS[1]) or '0')
if dirty > 0 and dirty <= incoming then
  redis.call('DEL', KEYS[1])
end
return 1
`)

var reconcileDirtyPolicyVersionScript = redis.NewScript(`
local dirty = redis.call('GET', KEYS[1])
if dirty == ARGV[1] then
  redis.call('SET', KEYS[2], ARGV[2], 'PX', ARGV[3])
  redis.call('DEL', KEYS[1])
  return 1
end
return 0
`)

func policyVersionKeys() (dirty, version string) {
	return redisutil.Channel("casbin:policy:version:dirty"), redisutil.Channel("casbin:policy:version")
}

func (s *policyVersionStore) Current(ctx context.Context) (uint64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if client := s.redisProvider(); client != nil && s.shouldTryRedis() {
		redisCtx, cancel := context.WithTimeout(ctx, policyVersionRedisTimeout)
		dirtyKey, versionKey := policyVersionKeys()
		result, err := readPolicyVersionScript.Run(redisCtx, client, []string{dirtyKey, versionKey}).Slice()
		cancel()
		if err == nil && len(result) == 2 {
			s.markRedisHealthy()
			dirty, dirtyErr := parsePolicyVersion(result[0])
			version, versionErr := parsePolicyVersion(result[1])
			if dirtyErr == nil && versionErr == nil && dirty == 0 && version > 0 {
				return version, nil
			}
			if dirtyErr == nil && dirty > 0 {
				return s.currentFromDB(ctx, dirty)
			}
		} else if err != nil {
			s.markRedisFailure()
		}
	}
	return s.currentFromDB(ctx, 0)
}

func (s *policyVersionStore) currentFromDB(ctx context.Context, observedDirty uint64) (uint64, error) {
	groupKey := "permission-policy-version"
	if observedDirty > 0 {
		groupKey += ":dirty:" + strconv.FormatUint(observedDirty, 10)
	}
	value, err, _ := s.dbReads.Do(groupKey, func() (any, error) {
		db := s.dbProvider()
		if db == nil {
			return uint64(0), errors.New("mysql not initialized")
		}
		row, err := s.readDBVersion(ctx, db, observedDirty)
		if err != nil {
			return uint64(0), err
		}
		if row.Version == 0 {
			return uint64(0), errors.New("permission policy version is invalid")
		}
		_ = s.recordCached(ctx, row.Version)
		return row.Version, nil
	})
	if err != nil {
		return 0, err
	}
	return value.(uint64), nil
}

func (s *policyVersionStore) readDBVersion(ctx context.Context, db *gorm.DB, observedDirty uint64) (permissionPolicyVersion, error) {
	var row permissionPolicyVersion
	if observedDirty == 0 {
		err := db.WithContext(ctx).First(&row, policyVersionRowID).Error
		return row, err
	}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).First(&row, policyVersionRowID).Error; err != nil {
			return err
		}
		// SHARE 锁会等待持 UPDATE 锁的写事务提交或回滚；锁内条件清理不会误删后续写事务的 dirty。
		if client := s.redisProvider(); client != nil {
			redisCtx, cancel := context.WithTimeout(ctx, policyVersionRedisTimeout)
			dirtyKey, versionKey := policyVersionKeys()
			reconcileErr := reconcileDirtyPolicyVersionScript.Run(redisCtx, client, []string{dirtyKey, versionKey},
				strconv.FormatUint(observedDirty, 10), strconv.FormatUint(row.Version, 10), s.cacheTTL.Milliseconds()).Err()
			cancel()
			if reconcileErr != nil {
				s.markRedisFailure()
			} else {
				s.markRedisHealthy()
			}
		}
		return nil
	})
	return row, err
}

func (s *policyVersionStore) LoadSnapshot(ctx context.Context, m casbinmodel.Model) (*casbin.Enforcer, uint64, error) {
	db := s.dbProvider()
	if db == nil {
		return nil, 0, errors.New("mysql not initialized")
	}
	var snapshot *casbin.Enforcer
	var version uint64
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row permissionPolicyVersion
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).First(&row, policyVersionRowID).Error; err != nil {
			return err
		}
		loaded, err := newEnforcerFromDBFunc(m, tx)
		if err != nil {
			return err
		}
		snapshot, version = loaded, row.Version
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	adapter, err := newPolicyAdapter(db)
	if err != nil {
		return nil, 0, err
	}
	snapshot.SetAdapter(adapter)
	return snapshot, version, nil
}

func (s *policyVersionStore) RecordLoaded(ctx context.Context, version uint64) error {
	return s.recordVersion(ctx, version)
}

func (s *policyVersionStore) recordCached(ctx context.Context, version uint64) error {
	if !s.shouldTryRedis() {
		return errors.New("redis retry is cooling down")
	}
	return s.recordVersion(ctx, version)
}

func (s *policyVersionStore) recordVersion(ctx context.Context, version uint64) error {
	client := s.redisProvider()
	if client == nil {
		return errors.New("redis not initialized")
	}
	redisCtx, cancel := context.WithTimeout(ctx, policyVersionRedisTimeout)
	defer cancel()
	dirtyKey, versionKey := policyVersionKeys()
	err := recordPolicyVersionScript.Run(redisCtx, client, []string{dirtyKey, versionKey},
		strconv.FormatUint(version, 10), s.cacheTTL.Milliseconds()).Err()
	if err != nil {
		s.markRedisFailure()
	} else {
		s.markRedisHealthy()
	}
	return err
}

func (s *policyVersionStore) beginMutation(tx *gorm.DB) (uint64, error) {
	var row permissionPolicyVersion
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, policyVersionRowID).Error; err != nil {
		return 0, err
	}
	target := row.Version + 1
	if client := s.redisProvider(); client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), policyVersionRedisTimeout)
		dirtyKey, _ := policyVersionKeys()
		err := client.Set(ctx, dirtyKey, target, s.dirtyTTL).Err()
		cancel()
		if err != nil {
			s.markRedisFailure()
			// Redis 失败时仍允许数据库事务推进；有限 TTL 版本缓存把旧值窗口限制在明确上限。
			return target, nil
		}
		s.markRedisHealthy()
	}
	return target, nil
}

func (s *policyVersionStore) shouldTryRedis() bool {
	return time.Now().UnixNano() >= s.redisRetryAt.Load()
}

func (s *policyVersionStore) markRedisFailure() {
	s.redisRetryAt.Store(time.Now().Add(policyVersionRedisCooldown).UnixNano())
}

func (s *policyVersionStore) markRedisHealthy() {
	s.redisRetryAt.Store(0)
}

func (s *policyVersionStore) finishMutation(tx *gorm.DB, target uint64) error {
	result := tx.Model(&permissionPolicyVersion{}).
		Where("id = ? AND version < ?", policyVersionRowID, target).
		Update("version", target)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("permission policy version advance to %d affected %d rows", target, result.RowsAffected)
	}
	return nil
}

func parsePolicyVersion(value any) (uint64, error) {
	if value == nil || value == "" {
		return 0, nil
	}
	switch typed := value.(type) {
	case string:
		return strconv.ParseUint(typed, 10, 64)
	case []byte:
		return strconv.ParseUint(string(typed), 10, 64)
	default:
		return 0, fmt.Errorf("unexpected permission policy version type %T", value)
	}
}
