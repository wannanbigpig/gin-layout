package main

import (
	"fmt"
	_ "l-admin.com/boot"
	c "l-admin.com/config"
	"l-admin.com/routers"
)

func main() {
	r := routers.SetRouters()
	err := r.Run(fmt.Sprintf("%s:%d", c.Config.Server.Host, c.Config.Server.Port))
	if err != nil {
		panic(err)
	}
}
