package request

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/pkg/utils/token"
)

// GetQueryParams 提取当前请求的查询参数。
// 说明：
//   - 不做字段白名单过滤，调用方需自行约束参数使用范围；
//   - 仅保留每个 key 的首个值（与 c.Query 行为一致）。
func GetQueryParams(c *gin.Context) map[string]any {
	if c == nil {
		return map[string]any{}
	}
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
