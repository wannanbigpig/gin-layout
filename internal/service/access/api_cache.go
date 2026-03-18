package access

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
)

const apiRedisKey = "api_info_map"

// ApiRouteInfo 描述接口路由的鉴权属性和展示名称。
type ApiRouteInfo struct {
	IsAuth uint8  `json:"is_auth"`
	Name   string `json:"name"`
}

// ApiRouteCacheService 负责缓存 API 路由元数据。
type ApiRouteCacheService struct{}

// NewApiRouteCacheService 创建 API 路由缓存服务实例。
func NewApiRouteCacheService() *ApiRouteCacheService {
	return &ApiRouteCacheService{}
}

func (s *ApiRouteCacheService) cacheKey(route string, method string) string {
	return fmt.Sprintf("%s:%s", method, route)
}

// RefreshCache 重建 Redis 中的 API 路由缓存。
func (s *ApiRouteCacheService) RefreshCache() error {
	cfg := config.GetConfig()
	if !cfg.Redis.Enable {
		return nil
	}

	apis, err := model.ListE(model.NewApi(), "", nil)
	if err != nil {
		return err
	}
	if len(apis) == 0 {
		return nil
	}

	ctx := context.Background()
	client := data.RedisClient()
	if client == nil {
		return nil
	}
	if err := client.Del(ctx, apiRedisKey).Err(); err != nil {
		return err
	}

	cacheData := make(map[string]any, len(apis))
	for _, api := range apis {
		cacheInfo := ApiRouteInfo{IsAuth: api.IsAuth, Name: api.Name}
		cacheInfoBytes, err := json.Marshal(cacheInfo)
		if err != nil {
			return err
		}
		cacheData[s.cacheKey(api.Route, api.Method)] = string(cacheInfoBytes)
	}
	return client.HSet(ctx, apiRedisKey, cacheData).Err()
}

// GetRouteInfo 返回指定路由的方法元数据。
func (s *ApiRouteCacheService) GetRouteInfo(route string, method string) (*ApiRouteInfo, error) {
	cfg := config.GetConfig()
	cacheKey := s.cacheKey(route, method)

	if cfg.Redis.Enable && data.RedisClient() != nil {
		val, err := data.RedisClient().HGet(context.Background(), apiRedisKey, cacheKey).Result()
		if err == nil {
			var cacheInfo ApiRouteInfo
			if unmarshalErr := json.Unmarshal([]byte(val), &cacheInfo); unmarshalErr == nil {
				return &cacheInfo, nil
			}
		} else if !errors.Is(err, redis.Nil) {
			logError("api表Redis查询出错", err, route, method)
		}
	}

	api := model.NewApi()
	if err := api.GetDetail("route = ? AND method = ? AND deleted_at = 0", route, method); err != nil {
		return nil, err
	}

	cacheInfo := &ApiRouteInfo{IsAuth: api.IsAuth, Name: api.Name}
	if cfg.Redis.Enable && data.RedisClient() != nil {
		if cacheInfoBytes, err := json.Marshal(cacheInfo); err == nil {
			_ = data.RedisClient().HSet(context.Background(), apiRedisKey, cacheKey, string(cacheInfoBytes)).Err()
		}
	}
	return cacheInfo, nil
}

// CheckoutRouteIsAuth 判断指定路由是否要求鉴权。
func (s *ApiRouteCacheService) CheckoutRouteIsAuth(route string, method string) bool {
	cacheInfo, err := s.GetRouteInfo(route, method)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logError("api表数据库查询出错", err, route, method)
		}
		return true
	}
	return cacheInfo.IsAuth == 1
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
