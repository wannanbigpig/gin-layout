package taskcron

import (
	"context"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/queue"
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
			return ExecuteHandler(ctx, handler, payload)
		})
		registered++
	}
	return registered
}
