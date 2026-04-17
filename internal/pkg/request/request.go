package request

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
)

// GetQueryParams 提取当前请求的查询参数。
func GetQueryParams(c *gin.Context) map[string]any {
	query := c.Request.URL.Query()
	var queryMap = make(map[string]any, len(query))
	for k := range query {
		queryMap[k] = c.Query(k)
	}
	return queryMap
}

// GetAccessToken 从 Authorization 请求头提取 access token。
func GetAccessToken(c *gin.Context) (string, error) {
	if c == nil {
		return "", errors.New("gin context is nil")
	}
	authorization := c.GetHeader("Authorization")
	return token.GetAccessToken(authorization)
}
