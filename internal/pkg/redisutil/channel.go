package redisutil

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/wannanbigpig/gin-layout/config"
)

// Channel 隔离共享 Redis 实例中的应用、环境和逻辑库；Pub/Sub 本身不隔离 DB。
func Channel(name string) string {
	namespace, environment, database := "go_layout", "default", 0
	if cfg := config.GetConfig(); cfg != nil {
		if value := strings.TrimSpace(cfg.Queue.Namespace); value != "" {
			namespace = value
		}
		if value := strings.TrimSpace(cfg.AppEnv); value != "" {
			environment = value
		}
		database = cfg.Redis.Database
	}
	return fmt.Sprintf("%s:%s:db%d:%s", url.PathEscape(namespace), url.PathEscape(environment), database, name)
}
