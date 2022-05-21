package data

import (
	"context"
	"github.com/go-redis/redis/v8"
	c "github.com/wannanbigpig/gin-layout/config"
)

var Rdb *redis.Client

func initRedis() {
	if c.Config.Redis.Enable == true {
		Rdb = redis.NewClient(&redis.Options{
			Addr:     c.Config.Redis.Host + ":" + c.Config.Redis.Port,
			Password: c.Config.Redis.Password,
			DB:       c.Config.Redis.Database,
		})
		var ctx = context.Background()
		_, err := Rdb.Ping(ctx).Result()

		if err != nil {
			panic("redis 链接错误：" + err.Error())
		}
	}
	return
}
