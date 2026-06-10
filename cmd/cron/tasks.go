package cron

import (
	"context"
	"errors"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/config"
	taskcron "github.com/wannanbigpig/gin-layout/internal/cron"
	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
)

var loadCronTaskStatus = loadCronTaskStatusFromDB

func defineSchedule(schedule *Scheduler) {
	cfg := config.GetConfig()
	for _, definition := range taskcron.BuiltinTaskDefinitions(cfg) {
		code := strings.TrimSpace(definition.Code)
		if definition.Kind != model.TaskKindCron {
			continue
		}
		if code == "" || strings.TrimSpace(definition.CronSpec) == "" {
			continue
		}

		taskName := taskNameFromCode(code)
		taskCode := code
		cronSpec := definition.CronSpec
		handler := definition.Handler
		defaultStatus := definition.Status
		isHighRisk := definition.IsHighRisk
		schedule.CallContextE(taskName, func(ctx context.Context) error {
			return taskcron.ExecuteHandler(ctx, handler, nil)
		}).
			EnabledWhen(func(ctx context.Context) bool {
				enabled := cronTaskEnabled(ctx, taskCode, defaultStatus)
				if enabled && isHighRisk == model.TaskHighRisk {
					log.Logger.Warn("高风险定时任务即将执行",
						zap.String("name", taskName),
						zap.String("schedule", cronSpec),
					)
				}
				return enabled
			}).
			Cron(cronSpec).
			WithoutOverlapping()
	}
}

func cronTaskEnabled(ctx context.Context, taskCode string, fallbackStatus uint8) bool {
	status, ok, err := loadCronTaskStatus(ctx, taskCode)
	if err != nil {
		log.Logger.Warn("读取定时任务运营状态失败",
			zap.String("task_code", taskCode),
			zap.Error(err))
		return false
	}
	if !ok {
		return fallbackStatus == model.TaskStatusEnabled
	}
	return status == model.TaskStatusEnabled
}

func loadCronTaskStatusFromDB(ctx context.Context, taskCode string) (uint8, bool, error) {
	db, err := model.GetDB()
	if err != nil {
		return 0, false, err
	}
	if !db.Migrator().HasTable(model.NewTaskDefinition().TableName()) {
		return 0, false, nil
	}

	var definition model.TaskDefinition
	err = db.WithContext(ctx).Model(&model.TaskDefinition{}).
		Select("status").
		Where("code = ? AND kind = ? AND deleted_at = 0", strings.TrimSpace(taskCode), model.TaskKindCron).
		First(&definition).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return definition.Status, true, nil
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
