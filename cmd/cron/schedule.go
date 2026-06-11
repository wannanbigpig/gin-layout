package cron

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/cronexpr"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/service/taskcenter"
)

// Scheduler 提供链式的任务注册方式。
type Scheduler struct {
	logger *cronLogger
	ctx    context.Context
	tasks  []*scheduledTask
}

type scheduledTask struct {
	taskCode    string
	name        string
	spec        string
	specErr     error
	run         func(context.Context) error
	enabledWhen func(context.Context) bool
	skipIfRun   bool
	isHighRisk  bool
}

// TaskBuilder 用于链式声明任务调度规则。
type TaskBuilder struct {
	task *scheduledTask
}

// NewSchedule 创建任务声明器。
func NewSchedule(logger *cronLogger) *Scheduler {
	return NewScheduleWithContext(context.Background(), logger)
}

// NewScheduleWithContext 创建带运行上下文的任务声明器。
func NewScheduleWithContext(ctx context.Context, logger *cronLogger) *Scheduler {
	if ctx == nil {
		ctx = context.Background()
	}
	return &Scheduler{
		logger: logger,
		ctx:    ctx,
		tasks:  make([]*scheduledTask, 0, 4),
	}
}

// Call 注册一个函数任务，默认启用防重入。
func (s *Scheduler) Call(name string, fn func()) *TaskBuilder {
	task := &scheduledTask{
		name:      name,
		run:       func(context.Context) error { fn(); return nil },
		skipIfRun: true,
	}
	s.tasks = append(s.tasks, task)
	return &TaskBuilder{task: task}
}

// CallE 注册一个返回 error 的函数任务。
func (s *Scheduler) CallE(name string, fn func() error) *TaskBuilder {
	task := &scheduledTask{
		name:      name,
		run:       func(context.Context) error { return fn() },
		skipIfRun: true,
	}
	s.tasks = append(s.tasks, task)
	return &TaskBuilder{task: task}
}

// CallContextE 注册一个接收运行上下文并返回 error 的函数任务。
func (s *Scheduler) CallContextE(name string, fn func(context.Context) error) *TaskBuilder {
	task := &scheduledTask{
		name:      name,
		run:       fn,
		skipIfRun: true,
	}
	s.tasks = append(s.tasks, task)
	return &TaskBuilder{task: task}
}

// Cron 直接使用 cron 表达式。
func (b *TaskBuilder) Cron(spec string) *TaskBuilder {
	b.task.spec = spec
	return b
}

// EnabledWhen 在任务触发时动态判断是否执行，适合读取后台运营开关。
func (b *TaskBuilder) EnabledWhen(fn func(context.Context) bool) *TaskBuilder {
	b.task.enabledWhen = fn
	return b
}

// EveryFiveSeconds 每 5 秒执行一次，适合本地测试任务。
func (b *TaskBuilder) EveryFiveSeconds() *TaskBuilder {
	return b.Cron("0/5 * * * * *")
}

// DailyAt 每天固定时间执行，支持 HH:MM 或 HH:MM:SS。
func (b *TaskBuilder) DailyAt(value string) *TaskBuilder {
	spec, err := dailyAtSpec(value)
	if err != nil {
		b.task.specErr = err
		return b
	}
	b.task.spec = spec
	return b
}

// WithoutOverlapping 表示任务执行期间跳过重入。
func (b *TaskBuilder) WithoutOverlapping() *TaskBuilder {
	b.task.skipIfRun = true
	return b
}

// AllowOverlap 允许任务重入。
func (b *TaskBuilder) AllowOverlap() *TaskBuilder {
	b.task.skipIfRun = false
	return b
}

// Register 把声明过的任务统一注册到 cron 实例中。
func (s *Scheduler) Register(crontab *cron.Cron) error {
	for _, task := range s.tasks {
		if _, err := s.registerTask(crontab, task); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scheduler) registerTask(crontab *cron.Cron, task *scheduledTask) (cron.EntryID, error) {
	return registerScheduledTask(crontab, s.logger, s.taskContext(), task)
}

func registerScheduledTask(crontab *cron.Cron, logger *cronLogger, ctx context.Context, task *scheduledTask) (cron.EntryID, error) {
	if task.specErr != nil {
		return 0, fmt.Errorf("定时任务 %s 调度表达式无效: %w", task.name, task.specErr)
	}
	if task.spec == "" {
		return 0, fmt.Errorf("定时任务 %s 缺少调度表达式", task.name)
	}

	chain := cron.NewChain(cron.Recover(logger))
	if task.skipIfRun {
		chain = cron.NewChain(
			cron.SkipIfStillRunning(logger),
			cron.Recover(logger),
		)
	}

	entryID, err := crontab.AddJob(task.spec, chain.Then(recordedJob(ctx, task)))
	if err != nil {
		return 0, fmt.Errorf("添加定时任务失败 [%s] (schedule: %s): %w", task.name, task.spec, err)
	}

	log.Logger.Info("定时任务添加成功",
		zap.String("name", task.name),
		zap.String("schedule", task.spec),
		zap.Bool("skip_if_still_running", task.skipIfRun),
	)
	return entryID, nil
}

func (s *Scheduler) recordedJob(task *scheduledTask) cron.Job {
	return recordedJob(s.taskContext(), task)
}

func recordedJob(ctx context.Context, task *scheduledTask) cron.Job {
	return cron.FuncJob(func() {
		if task == nil || task.run == nil {
			return
		}

		if ctx == nil {
			ctx = context.Background()
		}
		taskCode := task.taskCode
		if taskCode == "" {
			taskCode = "cron:" + task.name
		}
		if task.enabledWhen != nil && !task.enabledWhen(ctx) {
			log.Logger.Info("定时任务已停用，跳过执行", zap.String("name", task.name))
			return
		}

		// 高危 cron 任务需要获取分布式锁，防止多实例并发执行。
		if task.isHighRisk {
			lock, lockErr := acquireCronTaskLock(ctx, taskCode)
			if lockErr != nil {
				log.Logger.Error("高危定时任务获取分布式锁失败，跳过执行",
					zap.String("name", task.name),
					zap.String("task_code", taskCode),
					zap.Error(lockErr))
				return
			}
			defer lock.Release(ctx)
		}

		run, recordErr := taskcenter.NewRunRecorder().Start(ctx, taskcenter.RunStart{
			TaskCode: taskCode,
			Kind:     model.TaskKindCron,
			Source:   model.TaskSourceCron,
			SourceID: task.name,
			CronSpec: task.spec,
		})
		if recordErr != nil {
			log.Logger.Warn("记录定时任务开始失败",
				zap.String("name", task.name),
				zap.Error(recordErr))
		}

		err := task.run(ctx)
		if err != nil {
			log.Logger.Error("定时任务执行失败",
				zap.String("name", task.name),
				zap.Error(err))
		}

		if run == nil {
			return
		}
		finishInput := taskcenter.RunFinish{Error: err}
		nextRunAt, nextRunErr := calculateNextRunAt(task.spec, time.Now())
		if nextRunErr != nil {
			log.Logger.Warn("计算定时任务下次执行时间失败",
				zap.String("name", task.name),
				zap.String("schedule", task.spec),
				zap.Error(nextRunErr))
		}
		finishInput.NextRunAt = nextRunAt
		if recordErr := taskcenter.NewRunRecorder().Finish(ctx, run, finishInput); recordErr != nil {
			log.Logger.Warn("记录定时任务结束失败",
				zap.String("name", task.name),
				zap.Error(recordErr))
		}
	})
}

func (s *Scheduler) taskContext() context.Context {
	if s == nil || s.ctx == nil {
		return context.Background()
	}
	return s.ctx
}

func calculateNextRunAt(spec string, base time.Time) (*time.Time, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, nil
	}

	schedule, err := cronexpr.Parse(spec)
	if err != nil {
		return nil, err
	}

	nextRunAt := schedule.Next(base)
	if nextRunAt.IsZero() {
		return nil, nil
	}
	return &nextRunAt, nil
}

func dailyAtSpec(value string) (string, error) {
	parts := strings.Split(value, ":")
	switch len(parts) {
	case 2:
		hour, err := parseTimePart("hour", parts[0], 0, 23)
		if err != nil {
			return "", err
		}
		minute, err := parseTimePart("minute", parts[1], 0, 59)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("0 %d %d * * *", minute, hour), nil
	case 3:
		hour, err := parseTimePart("hour", parts[0], 0, 23)
		if err != nil {
			return "", err
		}
		minute, err := parseTimePart("minute", parts[1], 0, 59)
		if err != nil {
			return "", err
		}
		second, err := parseTimePart("second", parts[2], 0, 59)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d %d %d * * *", second, minute, hour), nil
	default:
		return "", fmt.Errorf("invalid daily time format: %s", value)
	}
}

func parseTimePart(name, raw string, min, max int) (int, error) {
	raw = strings.TrimSpace(raw)
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %q", name, raw)
	}
	if value < min || value > max {
		return 0, fmt.Errorf("%s value out of range [%d,%d]: %d", name, min, max, value)
	}
	return value, nil
}

// acquireCronTaskLock 获取高风险定时任务分布式锁。
// Redis 不可用时返回 error（fail-closed）。
var acquireCronTaskLock = func(ctx context.Context, taskCode string) (*taskcenter.TaskLock, error) {
	client := data.RedisClient()
	if client == nil {
		return nil, fmt.Errorf("redis 不可用，无法获取高危任务锁")
	}
	lock := taskcenter.NewTaskLock(client, taskCode)
	if err := lock.Acquire(ctx); err != nil {
		return nil, err
	}
	return lock, nil
}
