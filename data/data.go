package data

import "sync"

var once sync.Once

func InitData() {
	once.Do(func() {
		// 初始化 mysql
		initMysql()
		// 初始化 redis
		initRedis()
	})
}
