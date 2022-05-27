package data

import (
	c "github.com/wannanbigpig/gin-layout/config"
	"sync"
)

var once sync.Once

func InitData() {
	once.Do(func() {
		if c.Config.Mysql.Enable == true {
			// 初始化 mysql
			initMysql()
		}

		if c.Config.Redis.Enable == true {
			// 初始化 redis
			initRedis()
		}
	})
}
