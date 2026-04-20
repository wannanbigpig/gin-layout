package access

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
)

const (
	apiRedisKey                 = "api_info_map"
	apiCacheRedisTimeout        = 3 * time.Second
	apiCacheRefreshTotalTimeout = 15 * time.Second
	apiCacheWriteBatch          = 500
)

// ApiRouteInfo 描述接口路由的鉴权模式和展示名称。
type ApiRouteInfo struct {
	IsAuth uint8  `json:"is_auth"`
	Name   string `json:"name"`
}

type apiRouteCacheMetrics struct {
	requestTotal       atomic.Uint64
	cacheHitTotal      atomic.Uint64
	cacheMissTotal     atomic.Uint64
	sourceLoadTotal    atomic.Uint64
	singleflightShared atomic.Uint64
	refreshBatchTotal  atomic.Uint64
	refreshWriteTotal  atomic.Uint64
}

type apiRouteCacheEntry struct {
	field string
	value string
}

// ApiRouteCacheMetricsSnapshot 用于观测 API 路由缓存命中与回源情况。
type ApiRouteCacheMetricsSnapshot struct {
	RequestTotal       uint64  `json:"request_total"`
	CacheHitTotal      uint64  `json:"cache_hit_total"`
	CacheMissTotal     uint64  `json:"cache_miss_total"`
	HitRate            float64 `json:"hit_rate"`
	SourceLoadTotal    uint64  `json:"source_load_total"`
	SingleflightShared uint64  `json:"singleflight_shared_total"`
	RefreshBatchTotal  uint64  `json:"refresh_batch_total"`
	RefreshWriteTotal  uint64  `json:"refresh_write_total"`
}

// ApiRouteCacheService 负责缓存 API 路由元数据。
type ApiRouteCacheService struct {
	loadRouteInfo     func(route string, method string) (*ApiRouteInfo, error)
	singleflightGroup *singleflight.Group
	metrics           *apiRouteCacheMetrics
	configProvider    func() *config.Conf
}

// NewApiRouteCacheService 创建 API 路由缓存服务实例。
func NewApiRouteCacheService() *ApiRouteCacheService {
	return &ApiRouteCacheService{
		singleflightGroup: &singleflight.Group{},
		metrics:           &apiRouteCacheMetrics{},
		configProvider:    config.GetConfig,
	}
}

func (s *ApiRouteCacheService) ensureRuntimeDeps() {
	if s.singleflightGroup == nil {
		s.singleflightGroup = &singleflight.Group{}
	}
	if s.metrics == nil {
		s.metrics = &apiRouteCacheMetrics{}
	}
	if s.configProvider == nil {
		s.configProvider = config.GetConfig
	}
}

func (s *ApiRouteCacheService) currentConfig() *config.Conf {
	s.ensureRuntimeDeps()
	return config.GetConfigFrom(s.configProvider)
}

// MetricsSnapshot 返回当前 API 路由缓存指标快照。
func (s *ApiRouteCacheService) MetricsSnapshot() ApiRouteCacheMetricsSnapshot {
	s.ensureRuntimeDeps()

	requestTotal := s.metrics.requestTotal.Load()
	cacheHitTotal := s.metrics.cacheHitTotal.Load()
	cacheMissTotal := s.metrics.cacheMissTotal.Load()

	hitRate := 0.0
	if requestTotal > 0 {
		hitRate = float64(cacheHitTotal) / float64(requestTotal)
	}

	return ApiRouteCacheMetricsSnapshot{
		RequestTotal:       requestTotal,
		CacheHitTotal:      cacheHitTotal,
		CacheMissTotal:     cacheMissTotal,
		HitRate:            hitRate,
		SourceLoadTotal:    s.metrics.sourceLoadTotal.Load(),
		SingleflightShared: s.metrics.singleflightShared.Load(),
		RefreshBatchTotal:  s.metrics.refreshBatchTotal.Load(),
		RefreshWriteTotal:  s.metrics.refreshWriteTotal.Load(),
	}
}

// ResetMetrics 清空 API 路由缓存指标。
func (s *ApiRouteCacheService) ResetMetrics() {
	s.ensureRuntimeDeps()

	s.metrics.requestTotal.Store(0)
	s.metrics.cacheHitTotal.Store(0)
	s.metrics.cacheMissTotal.Store(0)
	s.metrics.sourceLoadTotal.Store(0)
	s.metrics.singleflightShared.Store(0)
	s.metrics.refreshBatchTotal.Store(0)
	s.metrics.refreshWriteTotal.Store(0)
}

func (s *ApiRouteCacheService) cacheKey(route string, method string) string {
	return fmt.Sprintf("%s:%s", method, route)
}

func redisContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), apiCacheRedisTimeout)
}

func redisContextWithTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if parent == nil {
		return context.WithTimeout(context.Background(), timeout)
	}
	if deadline, ok := parent.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return context.WithCancel(parent)
		}
		if remaining < timeout {
			return context.WithTimeout(parent, remaining)
		}
	}
	return context.WithTimeout(parent, timeout)
}

func (s *ApiRouteCacheService) refreshTempKey() string {
	return fmt.Sprintf("%s:refresh:%d", apiRedisKey, time.Now().UnixNano())
}

func (s *ApiRouteCacheService) writeRouteCacheBatch(parent context.Context, client *redis.Client, redisKey string, batch []apiRouteCacheEntry) error {
	if len(batch) == 0 {
		return nil
	}

	ctx, cancel := redisContextWithTimeout(parent, apiCacheRedisTimeout)
	defer cancel()

	pipe := client.Pipeline()
	for _, entry := range batch {
		pipe.HSet(ctx, redisKey, entry.field, entry.value)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}
	return nil
}

// RefreshCache 重建 Redis 中的 API 路由缓存。
func (s *ApiRouteCacheService) RefreshCache() error {
	s.ensureRuntimeDeps()

	cfg := s.currentConfig()
	if !cfg.Redis.Enable {
		return nil
	}

	apis, err := model.ListE(model.NewApi(), "", nil)
	if err != nil {
		return err
	}
	client := data.RedisClient()
	if client == nil {
		return nil
	}
	if len(apis) == 0 {
		ctx, cancel := redisContext()
		defer cancel()
		if err := client.Del(ctx, apiRedisKey).Err(); err != nil {
			return fmt.Errorf("clear empty api route cache failed: %w", err)
		}
		return nil
	}

	totalCtx, totalCancel := context.WithTimeout(context.Background(), apiCacheRefreshTotalTimeout)
	defer totalCancel()

	tempKey := s.refreshTempKey()
	shouldCleanupTempKey := true
	defer func() {
		if !shouldCleanupTempKey {
			return
		}
		ctx, cancel := redisContextWithTimeout(context.Background(), apiCacheRedisTimeout)
		defer cancel()
		if err := client.Del(ctx, tempKey).Err(); err != nil && !errors.Is(err, redis.Nil) {
			log.Logger.Warn("清理 API 路由缓存临时 key 失败",
				zap.String("key", tempKey),
				zap.Error(err))
		}
	}()

	batch := make([]apiRouteCacheEntry, 0, apiCacheWriteBatch)
	batchCount := 0
	writeCount := 0

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := s.writeRouteCacheBatch(totalCtx, client, tempKey, batch); err != nil {
			return fmt.Errorf("write api route cache batch failed: %w", err)
		}
		batchCount++
		writeCount += len(batch)
		batch = batch[:0]
		return nil
	}

	for _, api := range apis {
		cacheInfo := ApiRouteInfo{IsAuth: api.IsAuth, Name: api.Name}
		cacheInfoBytes, err := json.Marshal(cacheInfo)
		if err != nil {
			return err
		}
		batch = append(batch, apiRouteCacheEntry{
			field: s.cacheKey(api.Route, api.Method),
			value: string(cacheInfoBytes),
		})
		if len(batch) >= apiCacheWriteBatch {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := flush(); err != nil {
		return err
	}

	renameCtx, renameCancel := redisContextWithTimeout(totalCtx, apiCacheRedisTimeout)
	defer renameCancel()
	if err := client.Rename(renameCtx, tempKey, apiRedisKey).Err(); err != nil {
		return fmt.Errorf("swap api route cache key failed: %w", err)
	}
	shouldCleanupTempKey = false

	s.metrics.refreshBatchTotal.Add(uint64(batchCount))
	s.metrics.refreshWriteTotal.Add(uint64(writeCount))
	return nil
}

// GetRouteInfo 返回指定路由的方法元数据。
func (s *ApiRouteCacheService) GetRouteInfo(route string, method string) (*ApiRouteInfo, error) {
	s.ensureRuntimeDeps()
	s.metrics.requestTotal.Add(1)

	cfg := s.currentConfig()
	cacheKey := s.cacheKey(route, method)

	client := data.RedisClient()
	if cfg.Redis.Enable && client != nil {
		ctx, cancel := redisContext()
		defer cancel()
		val, err := client.HGet(ctx, apiRedisKey, cacheKey).Result()
		if err == nil {
			var cacheInfo ApiRouteInfo
			unmarshalErr := json.Unmarshal([]byte(val), &cacheInfo)
			if unmarshalErr == nil {
				s.metrics.cacheHitTotal.Add(1)
				return &cacheInfo, nil
			}
			logError("api 路由缓存反序列化失败", unmarshalErr, route, method)
			if delErr := client.HDel(ctx, apiRedisKey, cacheKey).Err(); delErr != nil {
				logError("api 路由缓存删除损坏值失败", delErr, route, method)
			}
		} else if !errors.Is(err, redis.Nil) {
			logError("api表Redis查询出错", err, route, method)
		}
	}

	s.metrics.cacheMissTotal.Add(1)
	value, err, shared := s.singleflightGroup.Do(cacheKey, func() (interface{}, error) {
		return s.loadRouteInfoFromSource(route, method)
	})
	if shared {
		s.metrics.singleflightShared.Add(1)
	}
	if err != nil {
		return nil, err
	}
	cacheInfo, ok := value.(*ApiRouteInfo)
	if !ok || cacheInfo == nil {
		return nil, fmt.Errorf("invalid api route info type")
	}
	return cacheInfo, nil
}

func (s *ApiRouteCacheService) loadRouteInfoFromSource(route string, method string) (*ApiRouteInfo, error) {
	s.ensureRuntimeDeps()
	s.metrics.sourceLoadTotal.Add(1)

	if s.loadRouteInfo != nil {
		return s.loadRouteInfo(route, method)
	}

	api := model.NewApi()
	if err := api.GetDetail("route = ? AND method = ? AND deleted_at = 0", route, method); err != nil {
		return nil, err
	}

	cacheInfo := &ApiRouteInfo{IsAuth: api.IsAuth, Name: api.Name}
	cfg := s.currentConfig()
	client := data.RedisClient()
	if cfg.Redis.Enable && client != nil {
		if cacheInfoBytes, err := json.Marshal(cacheInfo); err == nil {
			ctx, cancel := redisContext()
			defer cancel()
			cacheKey := s.cacheKey(route, method)
			if err := client.HSet(ctx, apiRedisKey, cacheKey, string(cacheInfoBytes)).Err(); err != nil {
				logError("api 路由缓存写入 Redis 失败", err, route, method)
			}
		}
	}

	return cacheInfo, nil
}

// CheckoutRouteIsAuth 判断指定路由是否要求 API 权限校验。
func (s *ApiRouteCacheService) CheckoutRouteIsAuth(route string, method string) bool {
	cacheInfo, err := s.GetRouteInfo(route, method)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logError("api表数据库查询出错", err, route, method)
		}
		return true
	}
	return global.ApiAuthMode(cacheInfo.IsAuth).RequiresAPIPermission()
}

// GetApiName 返回指定路由的人类可读名称。
func (s *ApiRouteCacheService) GetApiName(route string, method string) string {
	cacheInfo, err := s.GetRouteInfo(route, method)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logError("api名称数据库查询出错", err, route, method)
		}
		return ""
	}
	return cacheInfo.Name
}

func logError(message string, err error, route string, method string) {
	if log.Logger == nil {
		return
	}
	log.Logger.Error(message, zap.Error(err), zap.String("route", route), zap.String("method", method))
}
