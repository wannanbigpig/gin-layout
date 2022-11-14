package v1

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/wannanbigpig/gin-layout/internal/controller"
)

type DemoController struct {
	controller.Api
}

func NewDemoController() *DemoController {
	return &DemoController{}
}

// HelloWorld 测试
// @Summary  测试 的 hello world
// @Schemes
// @Description 简单测试的hello world
// @Tags ops 标签 比如同类功能使用同一个标签
// @Param Id query int true "Id"     参数 ：@Param 参数名 位置（query 或者 path或者 body） 类型 是否必需 注释
// @Accept json
// @Produce json
// @Success 200 {object} model.AdminUsers  返回结果 200 类型（object就是结构体） 类型 注释
// @Router /user [get]
func (api *DemoController) HelloWorld(c *gin.Context) {
	str, ok := c.GetQuery("name")
	if !ok {
		str = "gin-layout"
	}

	api.Success(c, fmt.Sprintf("hello %s", str))
}
