package data

import (
	"sync"

	c "github.com/wannanbigpig/gin-layout/config"
)

var once sync.Once

func InitData() {
	once.Do(func() {
		if c.Config.Mysql.Enable {
			// 初始化 mysql
			err := initMysql()
			if err != nil {
				panic("mysql init error: " + err.Error())
			}
		}

		if c.Config.Redis.Enable {
			// 初始化 redis
			err := initRedis()
			if err != nil {
				panic("redis init error: " + err.Error())
			}
		}
	})
}
