package middleware

import (
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/wannanbigpig/gin-layout/internal/global"
)

// SetAuditChangeDiff 设置本次请求的关键变更前后差异。
func SetAuditChangeDiff(c *gin.Context, before any, after any) {
	if c == nil {
		return
	}
	raw, err := json.Marshal(map[string]any{
		"before": before,
		"after":  after,
	})
	if err != nil {
		return
	}
	c.Set(global.ContextKeyAuditChangeDiff, string(raw))
}

// SetAuditChangeDiffRaw 直接写入变更差异 JSON 字符串。
func SetAuditChangeDiffRaw(c *gin.Context, rawJSON string) {
	if c == nil {
		return
	}
	c.Set(global.ContextKeyAuditChangeDiff, rawJSON)
}

// SetAuditHighRisk 设置本次请求是否按高危操作记录。
func SetAuditHighRisk(c *gin.Context, highRisk bool) {
	if c == nil {
		return
	}
	c.Set(global.ContextKeyAuditHighRisk, highRisk)
}

// SetAuditRequestBodyRaw 覆盖本次请求日志中的请求体快照。
func SetAuditRequestBodyRaw(c *gin.Context, rawJSON string) {
	if c == nil || strings.TrimSpace(rawJSON) == "" {
		return
	}
	c.Set(global.ContextKeyAuditRequestBody, rawJSON)
}
