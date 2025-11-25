package service

import "github.com/gin-gonic/gin"

type Base struct {
	ctx         *gin.Context
	adminUserId uint
}

// SetAdminUserId 设置管理员ID
func (b *Base) SetAdminUserId(userId uint) {
	b.adminUserId = userId
}

// GetAdminUserId 获取管理员ID
func (b *Base) GetAdminUserId() uint {
	return b.adminUserId
}

// SetCtx 设置上下文
func (b *Base) SetCtx(c *gin.Context) {
	b.ctx = c
}

// GetCtx 获取上下文
func (b *Base) GetCtx() *gin.Context {
	return b.ctx
}
