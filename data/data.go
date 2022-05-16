package data

import "sync"

var once sync.Once

func InitData() {
	once.Do(func() { initMysql() })
}
