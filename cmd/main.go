package main

import (
	"fmt"
	_ "github.com/wannanbigpig/gin-layout/boot"
	c "github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/routers"
)

func main() {
	r := routers.SetRouters()
	err := r.Run(fmt.Sprintf("%s:%d", c.Config.Server.Host, c.Config.Server.Port))
	if err != nil {
		panic(err)
	}
}
