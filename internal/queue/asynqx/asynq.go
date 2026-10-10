package asynqx

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/jobs"
	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/queue"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/service/taskcenter"
)

func init() {
	queue.RegisterPublisherFactory(NewPublisher)
	queue.RegisterInspectorFactory(NewInspector)
}

type publisher struct {
	client    *asynq.Client
	namespace string
}

type inspector struct {
	raw       *asynq.Inspector
	namespace string
}

// NewPublisher 创建 Asynq publisher。
func NewPublisher(cfg *config.Conf) (queue.Publisher, error) {
	if cfg == nil {
		cfg = config.GetConfig()
	}
	if cfg == nil || !cfg.Queue.Enable {
		return nil, nil
	}

	redisOpt, err := newRedisConnOpt(cfg)
	if err != nil {
		return nil, err
	}

	client := asynq.NewClient(redisOpt)
	return &publisher{
		client:    client,
		namespace: strings.TrimSpace(cfg.Queue.Namespace),
	}, nil
}

// NewInspector 创建 Asynq inspector。
func NewInspector(cfg *config.Conf) (queue.Inspector, error) {
	if cfg == nil {
		cfg = config.GetConfig()
	}
	if cfg == nil || !cfg.Queue.Enable {
		return nil, nil
	}

	redisOpt, err := newRedisConnOpt(cfg)
	if err != nil {
		return nil, err
	}

	return &inspector{
		raw:       asynq.NewInspector(redisOpt),
		namespace: strings.TrimSpace(cfg.Queue.Namespace),
	}, nil
}

func (p *publisher) Enqueue(ctx context.Context, job queue.Job) (queue.JobInfo, error) {
	if job == nil {
		return queue.JobInfo{}, errors.New("queue job is nil")
	}
	payload, err := job.Payload()
	if err != nil {
		return queue.JobInfo{}, err
	}
	task := asynq.NewTask(job.Type(), payload)

	options := make([]asynq.Option, 0, len(job.Options())+1)
	options = append(options, asynq.Queue(prefixedQueueName(p.namespace, job.Queue())))
	options = append(options, mapOptions(p.namespace, job.Options())...)

	info, err := p.client.EnqueueContext(ctx, task, options...)
	if err != nil {
		return queue.JobInfo{}, err
	}
	return queue.JobInfo{
		ID:    info.ID,
		Queue: unprefixQueueName(p.namespace, info.Queue),
		Type:  info.Type,
	}, nil
}

func (i *inspector) DeleteTask(ctx context.Context, queueName, taskID string) error {
	_ = ctx
	if i == nil || i.raw == nil {
		return queue.ErrInspectorUnavailable
	}
	return normalizeInspectorError(i.raw.DeleteTask(prefixedQueueName(i.namespace, queueName), taskID))
}

func (i *inspector) CancelProcessing(ctx context.Context, taskID string) error {
	_ = ctx
	if i == nil || i.raw == nil {
		return queue.ErrInspectorUnavailable
	}
	return normalizeInspectorError(i.raw.CancelProcessing(taskID))
}

func (i *inspector) GetQueueStats(ctx context.Context, queueName string) (*queue.QueueStats, error) {
	_ = ctx
	if i == nil || i.raw == nil {
		return nil, queue.ErrInspectorUnavailable
	}
	info, err := i.raw.GetQueueInfo(prefixedQueueName(i.namespace, queueName))
	if err != nil {
		return nil, normalizeInspectorError(err)
	}
	return convertQueueInfo(info, i.namespace), nil
}

func (i *inspector) GetAllQueueStats(ctx context.Context) ([]*queue.QueueStats, error) {
	_ = ctx
	if i == nil || i.raw == nil {
		return nil, queue.ErrInspectorUnavailable
	}
	queueNames, err := i.raw.Queues()
	if err != nil {
		return nil, err
	}

	statsByIndex := make([]*queue.QueueStats, len(queueNames))
	var wg sync.WaitGroup
	wg.Add(len(queueNames))
	for idx, name := range queueNames {
		idx, name := idx, name
		go func() {
			defer wg.Done()
			info, err := i.raw.GetQueueInfo(name)
			if err != nil {
				return
			}
			statsByIndex[idx] = convertQueueInfo(info, i.namespace)
		}()
	}
	wg.Wait()

	stats := make([]*queue.QueueStats, 0, len(queueNames))
	for _, item := range statsByIndex {
		if item != nil {
			stats = append(stats, item)
		}
	}
	return stats, nil
}

func convertQueueInfo(info *asynq.QueueInfo, namespace string) *queue.QueueStats {
	return &queue.QueueStats{
		Queue:     unprefixQueueName(namespace, info.Queue),
		Size:      info.Size,
		Pending:   info.Pending,
		Active:    info.Active,
		Scheduled: info.Scheduled,
		Retry:     info.Retry,
		Archived:  info.Archived,
		Completed: info.Completed,
		Processed: info.Processed,
		Failed:    info.Failed,
		Paused:    info.Paused,
	}
}

// NewServer 创建 Asynq worker server 和 mux。
func NewServer(cfg *config.Conf, registry queue.Registry) (*asynq.Server, *asynq.ServeMux, error) {
	if cfg == nil {
		cfg = config.GetConfig()
	}
	if cfg == nil {
		return nil, nil, errors.New("queue config is nil")
	}
	if registry == nil {
		return nil, nil, errors.New("queue registry is nil")
	}

	redisOpt, err := newRedisConnOpt(cfg)
	if err != nil {
		return nil, nil, err
	}

	server := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency:    cfg.Queue.Concurrency,
		Queues:         prefixedQueues(cfg.Queue.Namespace, cfg.Queue.Queues),
		StrictPriority: cfg.Queue.StrictPriority,
		ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
			log.Warn("Asynq task failed",
				zap.String("task_type", task.Type()),
				zap.Error(err))
			recordAsynqTaskError(ctx, task, err)
		}),
	})

	mux := asynq.NewServeMux()
	for _, entry := range registry.Entries() {
		entry := entry
		mux.HandleFunc(entry.TaskType, func(ctx context.Context, task *asynq.Task) error {
			run := recordAsynqTaskStart(ctx, task)
			err := entry.Handler(ctx, task.Payload())
			recordAsynqTaskFinish(ctx, run, err)
			if err == nil {
				return nil
			}
			if errors.Is(err, queue.ErrSkipRetry) {
				return fmt.Errorf("%w: %w", err, asynq.SkipRetry)
			}
			return err
		})
	}
	return server, mux, nil
}

func recordAsynqTaskStart(ctx context.Context, task *asynq.Task) *model.TaskRun {
	if task == nil {
		return nil
	}

	taskID, _ := asynq.GetTaskID(ctx)
	retryCount, _ := asynq.GetRetryCount(ctx)
	maxRetry, _ := asynq.GetMaxRetry(ctx)
	queueName, _ := asynq.GetQueueName(ctx)

	recorder := taskcenter.NewRunRecorder()
	run, err := recorder.Start(ctx, taskcenter.RunStart{
		TaskCode: task.Type(),
		Kind:     model.TaskKindAsync,
		Source:   model.TaskSourceQueue,
		SourceID: taskID,
		Queue:    unprefixQueueName(currentQueueNamespace(), queueName),
		Attempt:  retryCount + 1,
		MaxRetry: maxRetry,
		Payload:  task.Payload(),
	})
	if err != nil {
		log.Warn("Record async task start failed",
			zap.String("task_type", task.Type()),
			zap.String("task_id", taskID),
			zap.Error(err))
		return nil
	}
	return run
}

func recordAsynqTaskFinish(ctx context.Context, run *model.TaskRun, taskErr error) {
	if run == nil {
		return
	}
	if err := taskcenter.NewRunRecorder().Finish(ctx, run, resolveAsynqRunFinish(ctx, taskErr)); err != nil && !errors.Is(err, taskcenter.ErrTaskRunStatusChanged) {
		log.Warn("Record async task finish failed",
			zap.String("task_type", run.TaskCode),
			zap.String("task_id", run.SourceID),
			zap.Error(err))
	}
}

func resolveAsynqRunFinish(ctx context.Context, taskErr error) taskcenter.RunFinish {
	if taskErr == nil && ctx.Err() != nil {
		taskErr = ctx.Err()
	}
	finish := taskcenter.RunFinish{Error: taskErr}
	if taskErr == nil || errors.Is(taskErr, queue.ErrSkipRetry) {
		return finish
	}
	retryCount, retryOK := asynq.GetRetryCount(ctx)
	maxRetry, maxRetryOK := asynq.GetMaxRetry(ctx)
	if retryOK && maxRetryOK && retryCount < maxRetry {
		finish.Status = model.TaskRunStatusRetrying
	}
	return finish
}

// Asynq 超时不会等待 handler 返回，必须在队列失败回调中补齐本次执行状态。
func recordAsynqTaskError(ctx context.Context, task *asynq.Task, taskErr error) {
	if task == nil {
		return
	}
	taskID, ok := asynq.GetTaskID(ctx)
	if !ok || taskID == "" {
		return
	}
	db, err := model.GetDB()
	if err == nil {
		retryCount, _ := asynq.GetRetryCount(ctx)
		err = finishActiveAsynqTask(ctx, db, task.Type(), taskID, retryCount+1, resolveAsynqRunFinish(ctx, taskErr))
	}
	if err != nil {
		log.Warn("Finalize async task failure failed", zap.String("task_type", task.Type()), zap.String("task_id", taskID), zap.Error(err))
	}
}

func finishActiveAsynqTask(ctx context.Context, db *gorm.DB, taskCode, taskID string, attempt int, finish taskcenter.RunFinish) error {
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	return db.WithContext(finishCtx).Transaction(func(tx *gorm.DB) error {
		var run model.TaskRun
		// 锁住当前尝试直到导出状态也已落库，避免空 token 的未领取记录与下一次重试竞争。
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("task_code = ? AND source_id = ? AND attempt = ? AND status IN ?", taskCode, taskID, attempt, []string{
				model.TaskRunStatusPending, model.TaskRunStatusRunning, model.TaskRunStatusRetrying,
				model.TaskRunStatusFailed, model.TaskRunStatusTimeout, model.TaskRunStatusInterrupted,
			}).Order("id DESC").First(&run).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if run.Status == model.TaskRunStatusPending || run.Status == model.TaskRunStatusRunning {
			if err := taskcenter.NewRecorderWithDB(tx).Finish(finishCtx, &run, finish); err != nil {
				return err
			}
		}
		// wrapper 可能先结束任务，但领取导出失败时尚未写入导出终态；重复回调仅补齐导出。
		if taskCode != jobs.TypeExportFileList && taskCode != jobs.TypeExportRequestLog {
			return nil
		}
		svc := service.NewExportCenterServiceWithDeps(service.ExportCenterServiceDeps{DB: tx})
		token := fmt.Sprintf("%s:attempt:%d", taskID, attempt)
		if run.Status == model.TaskRunStatusRetrying {
			return svc.MarkPendingForRetry(finishCtx, run.ID, finish.Error, token)
		}
		// 领取前失败时 token 仍为空；行锁确保此时尚未开始下一次尝试。
		return svc.MarkFailed(finishCtx, run.ID, finish.Error, token, "")
	})
}

func currentQueueNamespace() string {
	cfg := config.GetConfig()
	if cfg == nil {
		return ""
	}
	return cfg.Queue.Namespace
}

func newRedisConnOpt(cfg *config.Conf) (asynq.RedisClientOpt, error) {
	if cfg == nil {
		return asynq.RedisClientOpt{}, errors.New("queue config is nil")
	}
	if cfg.Queue.UseDefaultRedis {
		if !cfg.Redis.Enable {
			return asynq.RedisClientOpt{}, errors.New("queue uses default redis, but redis.enable is false")
		}
		host := strings.TrimSpace(cfg.Redis.Host)
		port := strings.TrimSpace(cfg.Redis.Port)
		if host == "" || port == "" {
			return asynq.RedisClientOpt{}, errors.New("queue uses default redis, but redis host/port is empty")
		}
		return asynq.RedisClientOpt{
			Addr:     host + ":" + port,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.Database,
		}, nil
	}

	host := strings.TrimSpace(cfg.Queue.Redis.Host)
	port := strings.TrimSpace(cfg.Queue.Redis.Port)
	if host == "" || port == "" {
		return asynq.RedisClientOpt{}, errors.New("queue.redis host/port is required when queue.use_default_redis is false")
	}

	return asynq.RedisClientOpt{
		Addr:     host + ":" + port,
		Password: cfg.Queue.Redis.Password,
		DB:       cfg.Queue.Redis.Database,
	}, nil
}

func normalizeInspectorError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, asynq.ErrQueueNotFound) {
		return queue.ErrQueueNotFound
	}
	if errors.Is(err, asynq.ErrTaskNotFound) {
		return queue.ErrTaskNotFound
	}
	return err
}

func mapOptions(namespace string, options []queue.JobOption) []asynq.Option {
	mapped := make([]asynq.Option, 0, len(options))
	for _, option := range options {
		switch option.Type {
		case queue.JobOptionMaxRetry:
			mapped = append(mapped, asynq.MaxRetry(option.IntValue))
		case queue.JobOptionQueue:
			mapped = append(mapped, asynq.Queue(prefixedQueueName(namespace, option.StringValue)))
		case queue.JobOptionTimeout:
			mapped = append(mapped, asynq.Timeout(option.DurationValue))
		case queue.JobOptionRetention:
			mapped = append(mapped, asynq.Retention(option.DurationValue))
		case queue.JobOptionTaskID:
			mapped = append(mapped, asynq.TaskID(option.StringValue))
		}
	}
	return mapped
}

func prefixedQueues(namespace string, queues map[string]int) map[string]int {
	if len(queues) == 0 {
		return map[string]int{"default": 1}
	}

	prefixed := make(map[string]int, len(queues))
	for name, priority := range queues {
		prefixed[prefixedQueueName(namespace, name)] = priority
	}
	return prefixed
}

func prefixedQueueName(namespace, name string) string {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		trimmedName = "default"
	}
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return trimmedName
	}
	return namespace + ":" + trimmedName
}

func unprefixQueueName(namespace, name string) string {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return name
	}

	prefix := namespace + ":"
	if strings.HasPrefix(name, prefix) {
		return strings.TrimPrefix(name, prefix)
	}
	return name
}
