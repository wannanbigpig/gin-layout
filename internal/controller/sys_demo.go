package controller

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

// DemoController Demo控制器
type DemoController struct {
	Api
}

// NewDemoController 创建Demo控制器实例
func NewDemoController() *DemoController {
	return &DemoController{}
}

// HelloWorld Demo示例接口
func (api DemoController) HelloWorld(c *gin.Context) {
	name, ok := c.GetQuery("name")
	if !ok {
		name = "gin-layout"
	}

	id := c.Param("id")
	result := fmt.Sprintf("hello %s %s", name, id)
	api.Success(c, result)
}
