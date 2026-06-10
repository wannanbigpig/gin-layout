package cron

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/config"
	taskcron "github.com/wannanbigpig/gin-layout/internal/cron"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/cronexpr"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
)

var loadCronTaskStatus = loadCronTaskStatusFromDB
var loadCronTaskDefinitions = loadCronTaskDefinitionsFromDB

const cronDefinitionSyncInterval = 5 * time.Second

type cronTaskDefinition struct {
	TaskCode   string
	TaskName   string
	CronSpec   string
	Handler    string
	Status     uint8
	IsHighRisk uint8
}

type runtimeCronTask struct {
	definition cronTaskDefinition
	entryID    cron.EntryID
}

type runtimeCronRegistry struct {
	crontab *cron.Cron
	ctx     context.Context
	logger  *cronLogger
	tasks   map[string]runtimeCronTask
}

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

func startRuntimeDefinitionSync(ctx context.Context, crontab *cron.Cron) {
	registry := newRuntimeCronRegistry(ctx, crontab, &cronLogger{})
	registry.reconcileOnce(ctx)
	go registry.loop(ctx, cronDefinitionSyncInterval)
}

func newRuntimeCronRegistry(ctx context.Context, crontab *cron.Cron, logger *cronLogger) *runtimeCronRegistry {
	if ctx == nil {
		ctx = context.Background()
	}
	return &runtimeCronRegistry{
		crontab: crontab,
		ctx:     ctx,
		logger:  logger,
		tasks:   make(map[string]runtimeCronTask),
	}
}

func (r *runtimeCronRegistry) loop(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = cronDefinitionSyncInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.reconcileOnce(ctx)
		}
	}
}

func (r *runtimeCronRegistry) reconcileOnce(ctx context.Context) {
	if r == nil || r.crontab == nil {
		return
	}
	definitions, err := loadCronTaskDefinitions(ctx)
	if err != nil {
		log.Logger.Warn("同步定时任务定义失败", zap.Error(err))
		return
	}

	next := make(map[string]cronTaskDefinition, len(definitions))
	for _, definition := range definitions {
		code := strings.TrimSpace(definition.TaskCode)
		if code == "" {
			continue
		}
		definition.TaskCode = code
		if definition.TaskName == "" {
			definition.TaskName = taskNameFromCode(code)
		}
		next[code] = definition
	}

	for code, current := range r.tasks {
		definition, ok := next[code]
		if !ok || definition.Status != model.TaskStatusEnabled || strings.TrimSpace(definition.CronSpec) == "" {
			r.crontab.Remove(current.entryID)
			delete(r.tasks, code)
			log.Logger.Info("定时任务已从运行调度移除", zap.String("task_code", code))
		}
	}

	for code, definition := range next {
		if definition.Status != model.TaskStatusEnabled || strings.TrimSpace(definition.CronSpec) == "" {
			continue
		}
		current, exists := r.tasks[code]
		if exists && current.definition.CronSpec == definition.CronSpec && current.definition.Handler == definition.Handler && current.definition.IsHighRisk == definition.IsHighRisk {
			continue
		}
		if exists {
			r.crontab.Remove(current.entryID)
			delete(r.tasks, code)
		}
		entryID, err := r.addTask(definition)
		if err != nil {
			log.Logger.Warn("定时任务运行配置注册失败",
				zap.String("task_code", code),
				zap.String("schedule", definition.CronSpec),
				zap.Error(err))
			continue
		}
		r.tasks[code] = runtimeCronTask{definition: definition, entryID: entryID}
	}
}

func (r *runtimeCronRegistry) addTask(definition cronTaskDefinition) (cron.EntryID, error) {
	if err := cronexpr.Validate(definition.CronSpec); err != nil {
		return 0, err
	}
	taskName := strings.TrimSpace(definition.TaskName)
	if taskName == "" {
		taskName = taskNameFromCode(definition.TaskCode)
	}
	taskCode := strings.TrimSpace(definition.TaskCode)
	cronSpec := strings.TrimSpace(definition.CronSpec)
	handler := strings.TrimSpace(definition.Handler)
	builder := NewScheduleWithContext(r.ctx, r.logger).
		CallContextE(taskName, func(ctx context.Context) error {
			return taskcron.ExecuteHandler(ctx, handler, nil)
		}).
		EnabledWhen(func(context.Context) bool {
			if definition.IsHighRisk == model.TaskHighRisk {
				log.Logger.Warn("高风险定时任务即将执行",
					zap.String("name", taskName),
					zap.String("schedule", cronSpec),
				)
			}
			return true
		}).
		Cron(cronSpec).
		WithoutOverlapping()
	entryID, err := registerScheduledTask(r.crontab, r.logger, r.ctx, builder.task)
	if err != nil {
		return 0, err
	}
	log.Logger.Info("定时任务运行配置已生效",
		zap.String("task_code", taskCode),
		zap.String("schedule", cronSpec),
		zap.String("description", cronexpr.Describe(cronSpec)))
	return entryID, nil
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

func loadCronTaskDefinitionsFromDB(ctx context.Context) ([]cronTaskDefinition, error) {
	db, err := model.GetDB()
	if err != nil {
		return nil, err
	}
	if !db.Migrator().HasTable(model.NewTaskDefinition().TableName()) {
		return nil, nil
	}

	rows := make([]struct {
		Code       string
		CronSpec   string
		Handler    string
		Status     uint8
		IsHighRisk uint8
	}, 0)
	if err := db.WithContext(ctx).Model(&model.TaskDefinition{}).
		Select("code", "cron_spec", "handler", "status", "is_high_risk").
		Where("kind = ? AND deleted_at = 0", model.TaskKindCron).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	definitions := make([]cronTaskDefinition, 0, len(rows))
	for _, row := range rows {
		definitions = append(definitions, cronTaskDefinition{
			TaskCode:   strings.TrimSpace(row.Code),
			TaskName:   taskNameFromCode(row.Code),
			CronSpec:   strings.TrimSpace(row.CronSpec),
			Handler:    strings.TrimSpace(row.Handler),
			Status:     row.Status,
			IsHighRisk: row.IsHighRisk,
		})
	}
	return definitions, nil
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
