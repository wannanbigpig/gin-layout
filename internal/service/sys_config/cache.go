package sys_config

import (
	"sync"

	"github.com/wannanbigpig/gin-layout/internal/model"
)

type ConfigCacheItem struct {
	ConfigKey   string `json:"config_key"`
	ConfigValue string `json:"config_value"`
	ValueType   string `json:"value_type"`
	GroupCode   string `json:"group_code"`
}

var runtimeCache = struct {
	sync.RWMutex
	loaded bool
	items  map[string]ConfigCacheItem
}{
	items: make(map[string]ConfigCacheItem),
}

func replaceCache(configs []model.SysConfig) {
	next := make(map[string]ConfigCacheItem, len(configs))
	for _, config := range configs {
		next[config.ConfigKey] = ConfigCacheItem{
			ConfigKey:   config.ConfigKey,
			ConfigValue: config.ConfigValue,
			ValueType:   config.ValueType,
			GroupCode:   config.GroupCode,
		}
	}

	runtimeCache.Lock()
	defer runtimeCache.Unlock()
	runtimeCache.items = next
	runtimeCache.loaded = true
}

func getCachedValue(key string) (ConfigCacheItem, bool) {
	runtimeCache.RLock()
	defer runtimeCache.RUnlock()
	item, ok := runtimeCache.items[key]
	return item, ok
}

func cacheLoaded() bool {
	runtimeCache.RLock()
	defer runtimeCache.RUnlock()
	return runtimeCache.loaded
}
