package main

import (
	"fmt"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/wannanbigpig/gin-layout/boot"
	"github.com/wannanbigpig/gin-layout/config"
	_ "github.com/wannanbigpig/gin-layout/docs"
	"github.com/wannanbigpig/gin-layout/internal/command"
	"github.com/wannanbigpig/gin-layout/internal/routers"
	"strings"
)

// @title           Swagger dbsgw-api 接口在线地址
// @version         1.0
// @description     dbsgw-api 接口在线地址

// @license.name  个人技术博客
// @license.url   https://blog.dbsgw.com/

// @host      127.0.0.1:9999
func main() {
	run()
}

func run() {
	script := strings.Split(boot.Run, ":")
	switch script[0] {
	case "http":
		r := routers.SetRouters()
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
		err := r.Run(fmt.Sprintf("%s:%d", config.Config.Server.Host, config.Config.Server.Port))
		if err != nil {
			panic(err)
		}
	case "command":
		if len(script) != 2 {
			panic("命令错误，缺少重要参数")
		}
		command.Run(script[1])
	default:
		panic("执行脚本错误")
	}
}
