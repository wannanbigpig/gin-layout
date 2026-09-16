package redisutil

import (
	"testing"

	"github.com/wannanbigpig/gin-layout/config"
)

func TestChannelIsolatesLogicalDatabasesAndEnvironments(t *testing.T) {
	restore := config.UpdateConfigForTesting(func(cfg *config.Conf) { cfg.Queue.Namespace = "app"; cfg.AppEnv = "test"; cfg.Redis.Database = 1 })
	first := Channel("events")
	restore()
	restore = config.UpdateConfigForTesting(func(cfg *config.Conf) { cfg.Queue.Namespace = "app"; cfg.AppEnv = "test"; cfg.Redis.Database = 2 })
	second := Channel("events")
	restore()
	restore = config.UpdateConfigForTesting(func(cfg *config.Conf) { cfg.Queue.Namespace = "app"; cfg.AppEnv = "production"; cfg.Redis.Database = 1 })
	defer restore()
	if first == second || first == Channel("events") {
		t.Fatal("Pub/Sub scopes collided")
	}
}
