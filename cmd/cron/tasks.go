package cron

import (
	"context"
	"strings"

	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/config"
	taskcron "github.com/wannanbigpig/gin-layout/internal/cron"
	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
)

func defineSchedule(schedule *Scheduler) {
	cfg := config.GetConfig()
	for _, definition := range taskcron.BuiltinTaskDefinitions(cfg) {
		if definition.Kind != model.TaskKindCron || definition.Status != model.TaskStatusEnabled {
			continue
		}
		if strings.TrimSpace(definition.Code) == "" || strings.TrimSpace(definition.CronSpec) == "" {
			continue
		}

		taskName := taskNameFromCode(definition.Code)
		handler := definition.Handler
		if definition.IsHighRisk == model.TaskHighRisk {
			log.Logger.Warn("高风险定时任务已启用",
				zap.String("name", taskName),
				zap.String("schedule", definition.CronSpec),
			)
		}
		schedule.CallE(taskName, func() error {
			return taskcron.ExecuteHandler(context.Background(), handler, nil)
		}).
			Cron(definition.CronSpec).
			WithoutOverlapping()
	}
}

func taskNameFromCode(code string) string {
	code = strings.TrimSpace(code)
	if strings.HasPrefix(code, "cron:") {
		trimmed := strings.TrimPrefix(code, "cron:")
		if trimmed != "" {
			return trimmed
		}
	}
	return code
}
