package v1

import (
	"fmt"
	"github.com/gin-gonic/gin"
	r "github.com/wannanbigpig/gin-layout/internal/pkg/response"
)

// HelloWorld hello world
func HelloWorld(c *gin.Context) {
	str, ok := c.GetQuery("name")
	if !ok {
		str = "gin-layout"
	}
	panic("2")
	r.Success(c, fmt.Sprintf("hello %s", str))
}
