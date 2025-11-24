package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/model/modelDict"
	"github.com/wannanbigpig/gin-layout/internal/pkg/logger"
)

// Api 权限路由表
type Api struct {
	ContainsDeleteBaseModel
	Code        string `json:"code"`         // 权限唯一code
	GroupCode   string `json:"group_code"`   // 分组code
	Name        string `json:"name"`         // 权限名称
	Description string `json:"description"`  // 描述
	Method      string `json:"method"`       // 接口请求方法
	Route       string `json:"route"`        // 接口路由
	Func        string `json:"func"`         // 接口方法
	FuncPath    string `json:"func_path"`    // 接口方法路径
	IsAuth      uint8  `json:"is_auth"`      // 是否鉴权 0:否 1:是
	IsEffective uint8  `json:"is_effective"` // 是否有效 0:否 1:是
	Sort        int    `json:"sort"`         // 排序，数字越大优先级越高
}

func NewApi() *Api {
	return &Api{}
}

// TableName 获取表名
func (m *Api) TableName() string {
	return "a_api"
}

// InitRegisters 注册接口，写入到DB
func (m *Api) InitRegisters(data []map[string]any, date string) error {
	return m.DB(m).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "code"}},
			DoUpdates: clause.AssignmentColumns([]string{"func", "group_code", "func_path", "is_effective", "updated_at"}),
		}).Create(data).Error
		if err != nil {
			return err
		}
		return tx.Model(m).Where("updated_at != ?", date).Update("is_effective", 0).Error
	})
}

func (m *Api) AfterCreate(tx *gorm.DB) (err error) {
	return m.updateCache()
}

func (m *Api) AfterUpdate(tx *gorm.DB) (err error) {
	return m.updateCache()
}

// 缓存键名
const apiRedisKey = "api_info_map"

// apiCacheInfo API 缓存信息结构
type apiCacheInfo struct {
	IsAuth uint8  `json:"is_auth"` // 是否鉴权 0:否 1:是
	Name   string `json:"name"`    // 接口名称
}

// updateCache 更新缓存
func (m *Api) updateCache() error {
	if !config.Config.Redis.Enable {
		return nil
	}

	ctx := context.Background()

	// 1. 清空 Redis 旧缓存
	if err := data.RedisClient().Del(ctx, apiRedisKey).Err(); err != nil {
		logger.Error("Redis 清空失败: ", zap.Error(err))
		return err
	}

	// 2. 查询全部 API 数据
	apis := List(m, "", nil)
	if len(apis) == 0 {
		return nil
	}

	// 3. 构造批量 HSET 的 map[string]interface{}，使用 JSON 格式存储
	cacheData := make(map[string]interface{}, len(apis))
	for _, api := range apis {
		code := fmt.Sprintf("%s:%s", api.Method, api.Route)
		// 将 is_auth 和 name 合并为一个 JSON 对象
		cacheInfo := apiCacheInfo{
			IsAuth: api.IsAuth,
			Name:   api.Name,
		}
		if cacheInfoBytes, err := json.Marshal(cacheInfo); err == nil {
			cacheData[code] = string(cacheInfoBytes)
		}
	}

	// 4. 批量写入 Redis
	if err := data.RedisClient().HSet(ctx, apiRedisKey, cacheData).Err(); err != nil {
		logger.Error("Redis 批量写入失败: ", zap.Error(err))
		return err
	}

	return nil
}

// getApiCacheInfo 从缓存获取 API 信息（内部方法）
func (m *Api) getApiCacheInfo(route string, method string) (*apiCacheInfo, error) {
	code := fmt.Sprintf("%s:%s", method, route)

	// 1. 如果开启 Redis 缓存
	if config.Config.Redis.Enable {
		// 1.1 从 Redis 中读取
		val, err := data.RedisClient().HGet(context.Background(), apiRedisKey, code).Result()
		if err == nil {
			// 1.2 解析 JSON
			var cacheInfo apiCacheInfo
			if err := json.Unmarshal([]byte(val), &cacheInfo); err == nil {
				return &cacheInfo, nil
			}
		} else if !errors.Is(err, redis.Nil) {
			// Redis 查询出错（非缓存未命中），记录日志但继续查数据库
			logger.Error("api表Redis查询出错:", zap.Error(err))
		}
		// redis.Nil 是正常的缓存未命中，继续查询数据库
	}

	// 2. Redis 未命中或未开启，查数据库
	err := m.GetDetail(m, "route = ? AND method = ? AND deleted_at = 0", route, method)
	if err != nil {
		return nil, err
	}

	// 3. 构造缓存信息
	cacheInfo := &apiCacheInfo{
		IsAuth: m.IsAuth,
		Name:   m.Name,
	}

	// 4. 写入 Redis 缓存（便于下次使用）
	if config.Config.Redis.Enable {
		if cacheInfoBytes, err := json.Marshal(cacheInfo); err == nil {
			_ = data.RedisClient().HSet(context.Background(), apiRedisKey, code, string(cacheInfoBytes))
		}
	}

	return cacheInfo, nil
}

// CheckoutRouteIsAuth 检查接口是否需要授权
func (m *Api) CheckoutRouteIsAuth(route string, method string) bool {
	cacheInfo, err := m.getApiCacheInfo(route, method)
	if err != nil {
		// 如果是数据库查询错误或不存在，默认认为需要授权（保险做法）
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Error("api表数据库查询出错:", zap.Error(err))
		}
		return true
	}

	return cacheInfo.IsAuth == 1
}

// GetApiName 获取接口名称（带缓存优化）
func (m *Api) GetApiName(route string, method string) string {
	cacheInfo, err := m.getApiCacheInfo(route, method)
	if err != nil {
		// 如果是数据库查询错误或不存在，返回空字符串
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Error("api名称数据库查询出错:", zap.Error(err))
		}
		return ""
	}

	return cacheInfo.Name
}

// IsAuthMap 是否授权映射
func (m *Api) IsAuthMap() string {
	return modelDict.IsMap.Map(m.IsAuth)
}

// IsEffectiveMap 是否有效映射
func (m *Api) IsEffectiveMap() string {
	return modelDict.IsMap.Map(m.IsEffective)
}
