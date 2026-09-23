package taskcron

import (
	"context"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/queue"
	"go.uber.org/zap"
)

// RegisterQueueFallbackHandlers registers non-high-risk cron handlers for
// historical Asynq tasks that were enqueued before cron/worker boundaries were split.
func RegisterQueueFallbackHandlers(registry queue.Registry, cfg *config.Conf) int {
	if registry == nil {
		return 0
	}

	registered := 0
	for _, definition := range BuiltinTaskDefinitions(cfg) {
		if definition.Kind != model.TaskKindCron || definition.IsHighRisk == model.TaskHighRisk {
			continue
		}
		taskCode := definition.Code
		handler := definition.Handler
		if taskCode == "" || handler == "" {
			continue
		}

		queue.RegisterJSON(registry, taskCode, func(ctx context.Context, payload map[string]any) error {
			if !isTaskEnabled(ctx, taskCode) {
				log.Logger.Info("跳过已禁用的 cron fallback 任务", zap.String("code", taskCode))
				return nil
			}
			return ExecuteHandler(ctx, handler, payload)
		})
		registered++
	}
	return registered
}

func isTaskEnabled(ctx context.Context, taskCode string) bool {
	db := data.MysqlDB()
	if db == nil {
		return true
	}
	var def model.TaskDefinition
	err := db.WithContext(ctx).Where("code = ?", taskCode).Select("status").First(&def).Error
	if err != nil {
		return true
	}
	return def.Status == model.TaskStatusEnabled
}
