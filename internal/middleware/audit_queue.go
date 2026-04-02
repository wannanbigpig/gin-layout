package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/jobs"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	audit "github.com/wannanbigpig/gin-layout/internal/service/audit"
)

var enqueueAuditLogFunc = jobs.EnqueueAuditLog

func enqueueAuditLog(c *gin.Context, kind string, snapshot *audit.AuditLogSnapshot) {
	if snapshot == nil {
		return
	}
	if cfg := config.GetConfig(); cfg == nil || !cfg.Queue.Enable {
		return
	}

	ctx := context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
	}

	if err := enqueueAuditLogFunc(ctx, kind, snapshot); err != nil {
		log.Warn("Enqueue audit log failed",
			zap.String("kind", kind),
			zap.String("request_id", snapshot.RequestID),
			zap.Error(err))
	}
}
