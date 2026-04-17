package cron

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
)

// Scheduler 提供链式的任务注册方式。
type Scheduler struct {
	logger *cronLogger
	tasks  []*scheduledTask
}

type scheduledTask struct {
	name      string
	spec      string
	specErr   error
	job       cron.Job
	skipIfRun bool
}

// TaskBuilder 用于链式声明任务调度规则。
type TaskBuilder struct {
	task *scheduledTask
}

func registerTasks(crontab *cron.Cron) error {
	logger := &cronLogger{}
	schedule := NewSchedule(logger)
	defineSchedule(schedule)
	return schedule.Register(crontab)
}

// NewSchedule 创建任务声明器。
func NewSchedule(logger *cronLogger) *Scheduler {
	return &Scheduler{
		logger: logger,
		tasks:  make([]*scheduledTask, 0, 4),
	}
}

// Call 注册一个函数任务，默认启用防重入。
func (s *Scheduler) Call(name string, fn func()) *TaskBuilder {
	task := &scheduledTask{
		name:      name,
		job:       cron.FuncJob(fn),
		skipIfRun: true,
	}
	s.tasks = append(s.tasks, task)
	return &TaskBuilder{task: task}
}

// CallE 注册一个返回 error 的函数任务。
func (s *Scheduler) CallE(name string, fn func() error) *TaskBuilder {
	return s.Call(name, func() {
		if err := fn(); err != nil {
			log.Logger.Error("定时任务执行失败",
				zap.String("name", name),
				zap.Error(err),
			)
		}
	})
}

// Cron 直接使用 cron 表达式。
func (b *TaskBuilder) Cron(spec string) *TaskBuilder {
	b.task.spec = spec
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
		if err := s.registerTask(crontab, task); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scheduler) registerTask(crontab *cron.Cron, task *scheduledTask) error {
	if task.specErr != nil {
		return fmt.Errorf("定时任务 %s 调度表达式无效: %w", task.name, task.specErr)
	}
	if task.spec == "" {
		return fmt.Errorf("定时任务 %s 缺少调度表达式", task.name)
	}

	chain := cron.NewChain(cron.Recover(s.logger))
	if task.skipIfRun {
		chain = cron.NewChain(
			cron.SkipIfStillRunning(s.logger),
			cron.Recover(s.logger),
		)
	}

	if _, err := crontab.AddJob(task.spec, chain.Then(task.job)); err != nil {
		return fmt.Errorf("添加定时任务失败 [%s] (schedule: %s): %w", task.name, task.spec, err)
	}

	log.Logger.Info("定时任务添加成功",
		zap.String("name", task.name),
		zap.String("schedule", task.spec),
		zap.Bool("skip_if_still_running", task.skipIfRun),
	)
	return nil
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
