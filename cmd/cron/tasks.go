package cron

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/wannanbigpig/gin-layout/internal/pkg/redisutil"

	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"gorm.io/gorm/clause"

	"github.com/wannanbigpig/gin-layout/data"
	taskcron "github.com/wannanbigpig/gin-layout/internal/cron"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/cronexpr"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/service/taskcenter"
)

var loadCronTaskDefinitions = loadCronTaskDefinitionsFromDB

const cronDefinitionSyncInterval = 5 * time.Second
const cronRuntimeCacheMaxAge = cronDefinitionSyncInterval * 3

const (
	cronRuntimeSyncSuccess = "success"
	cronRuntimeSyncFailed  = "failed"
)

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
	loadedAt   time.Time
	syncErr    string
}

type runtimeStateUpdate struct {
	definition   cronTaskDefinition
	activeStatus uint8
	syncStatus   string
	syncErr      error
}

type runtimeCronRegistry struct {
	mu        sync.RWMutex
	crontab   *cron.Cron
	ctx       context.Context
	logger    *cronLogger
	tasks     map[string]runtimeCronTask
	stateKeys map[string]string
	instance  string
}

func startRuntimeDefinitionSync(ctx context.Context, crontab *cron.Cron) {
	registry := newRuntimeCronRegistry(ctx, crontab, &cronLogger{})
	registry.reconcileOnce(ctx)
	go registry.loop(ctx, cronDefinitionSyncInterval)
	go registry.subscribeTaskDefinitionChanges(ctx)
}

func newRuntimeCronRegistry(ctx context.Context, crontab *cron.Cron, logger *cronLogger) *runtimeCronRegistry {
	if ctx == nil {
		ctx = context.Background()
	}
	return &runtimeCronRegistry{
		crontab:   crontab,
		ctx:       ctx,
		logger:    logger,
		tasks:     make(map[string]runtimeCronTask),
		stateKeys: make(map[string]string),
		instance:  cronSchedulerInstanceID(),
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
		r.markSyncFailure(ctx, err)
		return
	}

	now := time.Now()
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

	updates := make([]runtimeStateUpdate, 0, len(next))

	r.mu.Lock()
	for code, current := range r.tasks {
		definition, ok := next[code]
		if !ok || definition.Status != model.TaskStatusEnabled || strings.TrimSpace(definition.CronSpec) == "" {
			r.crontab.Remove(current.entryID)
			delete(r.tasks, code)
			if ok {
				updates = r.appendRuntimeStateUpdateIfChanged(updates, definition, model.TaskStatusDisabled, cronRuntimeSyncSuccess, nil)
			} else {
				updates = r.appendRuntimeStateUpdateIfChanged(updates, current.definition, model.TaskStatusDisabled, cronRuntimeSyncSuccess, nil)
			}
			log.Logger.Info("定时任务已从运行调度移除", zap.String("task_code", code))
		}
	}

	for code, definition := range next {
		if definition.Status != model.TaskStatusEnabled || strings.TrimSpace(definition.CronSpec) == "" {
			updates = r.appendRuntimeStateUpdateIfChanged(updates, definition, model.TaskStatusDisabled, cronRuntimeSyncSuccess, nil)
			continue
		}
		current, exists := r.tasks[code]
		if exists && current.definition.CronSpec == definition.CronSpec && current.definition.Handler == definition.Handler && current.definition.IsHighRisk == definition.IsHighRisk {
			current.definition = definition
			current.loadedAt = now
			current.syncErr = ""
			updates = r.appendRuntimeStateUpdateIfChanged(updates, definition, model.TaskStatusEnabled, cronRuntimeSyncSuccess, nil)
			r.tasks[code] = current
			continue
		}
		entryID, err := r.addTask(definition)
		if err != nil {
			log.Logger.Warn("定时任务运行配置注册失败",
				zap.String("task_code", code),
				zap.String("schedule", definition.CronSpec),
				zap.Error(err))
			if exists {
				current.syncErr = err.Error()
				r.tasks[code] = current
				updates = r.appendRuntimeStateUpdateIfChanged(updates, definition, model.TaskStatusEnabled, cronRuntimeSyncFailed, err)
			} else {
				updates = r.appendRuntimeStateUpdateIfChanged(updates, definition, model.TaskStatusDisabled, cronRuntimeSyncFailed, err)
			}
			continue
		}
		if exists {
			r.crontab.Remove(current.entryID)
		}
		r.tasks[code] = runtimeCronTask{
			definition: definition,
			entryID:    entryID,
			loadedAt:   now,
		}
		updates = r.appendRuntimeStateUpdateIfChanged(updates, definition, model.TaskStatusEnabled, cronRuntimeSyncSuccess, nil)
	}
	r.mu.Unlock()

	recorded := make([]runtimeStateUpdate, 0, len(updates))
	for _, update := range updates {
		if r.recordRuntimeState(ctx, update.definition, update.activeStatus, update.syncStatus, update.syncErr) {
			recorded = append(recorded, update)
		}
	}
	r.markRuntimeStatesRecorded(recorded)
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
			if !r.taskCanRun(taskCode, cronSpec) {
				return false
			}
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
	builder.task.taskCode = taskCode
	builder.task.isHighRisk = definition.IsHighRisk == model.TaskHighRisk
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

func (r *runtimeCronRegistry) taskCanRun(taskCode, cronSpec string) bool {
	taskCode = strings.TrimSpace(taskCode)
	cronSpec = strings.TrimSpace(cronSpec)
	if taskCode == "" {
		return false
	}

	r.mu.RLock()
	current, ok := r.tasks[taskCode]
	r.mu.RUnlock()
	if !ok {
		return false
	}
	definition := current.definition
	if definition.Status != model.TaskStatusEnabled || strings.TrimSpace(definition.CronSpec) != cronSpec {
		return false
	}
	if definition.IsHighRisk == model.TaskHighRisk {
		if current.syncErr != "" {
			log.Logger.Warn("高风险定时任务同步状态异常，跳过执行",
				zap.String("task_code", taskCode),
				zap.String("sync_error", current.syncErr))
			return false
		}
		if current.loadedAt.IsZero() || time.Since(current.loadedAt) > cronRuntimeCacheMaxAge {
			log.Logger.Warn("高风险定时任务运行缓存过期，跳过执行",
				zap.String("task_code", taskCode),
				zap.Time("loaded_at", current.loadedAt))
			return false
		}
	}
	return true
}

func (r *runtimeCronRegistry) markSyncFailure(ctx context.Context, syncErr error) {
	if r == nil || syncErr == nil {
		return
	}
	updates := make([]runtimeStateUpdate, 0)
	r.mu.Lock()
	for code, current := range r.tasks {
		current.syncErr = syncErr.Error()
		updates = r.appendRuntimeStateUpdateIfChanged(updates, current.definition, current.definition.Status, cronRuntimeSyncFailed, syncErr)
		r.tasks[code] = current
	}
	r.mu.Unlock()
	recorded := make([]runtimeStateUpdate, 0, len(updates))
	for _, update := range updates {
		if r.recordRuntimeState(ctx, update.definition, update.activeStatus, update.syncStatus, update.syncErr) {
			recorded = append(recorded, update)
		}
	}
	r.markRuntimeStatesRecorded(recorded)
}

func (r *runtimeCronRegistry) subscribeTaskDefinitionChanges(ctx context.Context) {
	var subscription redisutil.DynamicSubscription
	defer subscription.Close()
	refresh := func() <-chan *redis.Message {
		return subscription.Refresh(ctx, data.RedisClient(), redisutil.Channel(taskcenter.TaskDefinitionChangeChannel))
	}
	messages := refresh()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			messages = refresh()
		case message, ok := <-messages:
			if !ok {
				subscription.Reset()
				messages = nil
				continue
			}
			if message == nil {
				continue
			}
			payload, valid := taskcenter.DecodeTaskDefinitionChangePayload(message.Payload)
			if !valid {
				continue
			}
			log.Logger.Info("收到任务定义变更通知", zap.String("task_code", payload.TaskCode))
			r.reconcileOnce(ctx)
		}
	}
}

func (r *runtimeCronRegistry) appendRuntimeStateUpdateIfChanged(updates []runtimeStateUpdate, definition cronTaskDefinition, activeStatus uint8, syncStatus string, syncErr error) []runtimeStateUpdate {
	update := runtimeStateUpdate{
		definition:   definition,
		activeStatus: activeStatus,
		syncStatus:   syncStatus,
		syncErr:      syncErr,
	}
	code := strings.TrimSpace(definition.TaskCode)
	if code == "" || r.stateKeys[code] == runtimeStateKey(update) {
		return updates
	}
	return append(updates, update)
}

func (r *runtimeCronRegistry) markRuntimeStatesRecorded(updates []runtimeStateUpdate) {
	if len(updates) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, update := range updates {
		code := strings.TrimSpace(update.definition.TaskCode)
		if code == "" {
			continue
		}
		r.stateKeys[code] = runtimeStateKey(update)
	}
}

func runtimeStateKey(update runtimeStateUpdate) string {
	errText := ""
	if update.syncErr != nil {
		errText = truncateRuntimeSyncError(update.syncErr.Error())
	}
	activeCronSpec := ""
	if update.activeStatus == model.TaskStatusEnabled {
		activeCronSpec = strings.TrimSpace(update.definition.CronSpec)
	}
	return fmt.Sprintf("%s|%s|%d|%s|%s|%s",
		strings.TrimSpace(update.definition.TaskCode),
		strings.TrimSpace(update.definition.CronSpec),
		update.activeStatus,
		activeCronSpec,
		update.syncStatus,
		errText)
}

func (r *runtimeCronRegistry) recordRuntimeState(ctx context.Context, definition cronTaskDefinition, activeStatus uint8, syncStatus string, syncErr error) bool {
	if r == nil || strings.TrimSpace(definition.TaskCode) == "" {
		return false
	}
	db, err := model.GetDB()
	if err != nil {
		return false
	}
	if !db.Migrator().HasTable(model.NewCronTaskState().TableName()) {
		return false
	}
	if !db.Migrator().HasColumn(model.NewCronTaskState(), "active_status") {
		return false
	}

	now := utils.FormatDate{Time: time.Now()}
	state := model.NewCronTaskState()
	state.TaskCode = strings.TrimSpace(definition.TaskCode)
	state.CronSpec = strings.TrimSpace(definition.CronSpec)
	state.ActiveStatus = activeStatus
	if activeStatus == model.TaskStatusEnabled {
		state.ActiveCronSpec = strings.TrimSpace(definition.CronSpec)
	}
	state.LastSyncAt = &now
	state.LastSyncStatus = syncStatus
	state.SchedulerInstance = r.instance
	if syncErr != nil {
		state.LastSyncError = truncateRuntimeSyncError(syncErr.Error())
	}

	assignments := []string{
		"cron_spec",
		"active_status",
		"active_cron_spec",
		"last_sync_at",
		"last_sync_status",
		"last_sync_error",
		"scheduler_instance",
		"updated_at",
	}
	if err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "task_code"}},
		DoUpdates: clause.AssignmentColumns(assignments),
	}).Create(state).Error; err != nil {
		log.Logger.Warn("记录定时任务运行同步状态失败",
			zap.String("task_code", definition.TaskCode),
			zap.Error(err))
		return false
	}
	return true
}

func truncateRuntimeSyncError(raw string) string {
	const limit = 8 * 1024
	if len(raw) <= limit {
		return raw
	}
	return raw[:limit] + "...(truncated)"
}

func cronSchedulerInstanceID() string {
	host, err := os.Hostname()
	if err != nil {
		host = "unknown"
	}
	return fmt.Sprintf("%s:%d:%d", host, os.Getpid(), time.Now().UnixNano())
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
