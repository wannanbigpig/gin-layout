package asynqx

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/config"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/queue"
)

func init() {
	queue.RegisterPublisherFactory(NewPublisher)
}

type publisher struct {
	client    *asynq.Client
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
		}),
	})

	mux := asynq.NewServeMux()
	for _, entry := range registry.Entries() {
		entry := entry
		mux.HandleFunc(entry.TaskType, func(ctx context.Context, task *asynq.Task) error {
			err := entry.Handler(ctx, task.Payload())
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
