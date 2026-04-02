package queue

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/wannanbigpig/gin-layout/config"
)

var (
	ErrPublisherUnavailable = errors.New("queue publisher unavailable")
	ErrSkipRetry            = errors.New("queue skip retry")
)

// Job 表示一个可发布的异步任务。
type Job interface {
	Type() string
	Queue() string
	Payload() ([]byte, error)
	Options() []JobOption
}

// JobInfo 表示任务发布后的基础元信息。
type JobInfo struct {
	ID    string
	Queue string
	Type  string
}

// Publisher 负责发布任务。
type Publisher interface {
	Enqueue(ctx context.Context, job Job) (JobInfo, error)
}

// Handler 负责消费任务 payload。
type Handler func(ctx context.Context, payload []byte) error

// Registry 保存任务类型到 handler 的映射。
type Registry interface {
	Register(taskType string, handler Handler)
	Entries() []Registration
}

// Registration 描述一个已注册的任务处理器。
type Registration struct {
	TaskType string
	Handler  Handler
}

// JobOptionType 表示任务选项类型。
type JobOptionType string

const (
	JobOptionMaxRetry  JobOptionType = "max_retry"
	JobOptionQueue     JobOptionType = "queue"
	JobOptionTimeout   JobOptionType = "timeout"
	JobOptionRetention JobOptionType = "retention"
	JobOptionTaskID    JobOptionType = "task_id"
)

// JobOption 表示项目内统一的任务选项。
type JobOption struct {
	Type          JobOptionType
	IntValue      int
	StringValue   string
	DurationValue time.Duration
}

func WithMaxRetry(n int) JobOption {
	return JobOption{Type: JobOptionMaxRetry, IntValue: n}
}

func WithQueue(name string) JobOption {
	return JobOption{Type: JobOptionQueue, StringValue: name}
}

func WithTimeout(timeout time.Duration) JobOption {
	return JobOption{Type: JobOptionTimeout, DurationValue: timeout}
}

func WithRetention(retention time.Duration) JobOption {
	return JobOption{Type: JobOptionRetention, DurationValue: retention}
}

func WithTaskID(taskID string) JobOption {
	return JobOption{Type: JobOptionTaskID, StringValue: taskID}
}

type publisherFactory func(cfg *config.Conf) (Publisher, error)

var (
	publisherMu      sync.RWMutex
	activePublisher  Publisher
	activePublisherF publisherFactory
)

// RegisterPublisherFactory 注册默认的 publisher 构建器。
func RegisterPublisherFactory(factory func(cfg *config.Conf) (Publisher, error)) {
	publisherMu.Lock()
	defer publisherMu.Unlock()
	activePublisherF = factory
}

// InitPublisher 根据当前配置初始化全局 publisher。
func InitPublisher(cfg *config.Conf) error {
	publisherMu.Lock()
	defer publisherMu.Unlock()

	if cfg == nil || !cfg.Queue.Enable {
		activePublisher = nil
		return nil
	}
	if activePublisherF == nil {
		return errors.New("queue publisher factory not registered")
	}

	publisher, err := activePublisherF(cfg)
	if err != nil {
		activePublisher = nil
		return err
	}
	activePublisher = publisher
	return nil
}

// PublisherOrNil 返回当前全局 publisher；未启用时返回 nil。
func PublisherOrNil() Publisher {
	publisherMu.RLock()
	defer publisherMu.RUnlock()
	return activePublisher
}

// SetPublisherForTesting 仅用于测试时替换全局 publisher。
func SetPublisherForTesting(publisher Publisher) func() {
	publisherMu.Lock()
	previous := activePublisher
	activePublisher = publisher
	publisherMu.Unlock()

	return func() {
		publisherMu.Lock()
		activePublisher = previous
		publisherMu.Unlock()
	}
}

type memoryRegistry struct {
	mu      sync.RWMutex
	entries map[string]Handler
}

// NewRegistry 创建一个内存 registry。
func NewRegistry() Registry {
	return &memoryRegistry{
		entries: make(map[string]Handler),
	}
}

func (r *memoryRegistry) Register(taskType string, handler Handler) {
	if taskType == "" || handler == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[taskType] = handler
}

func (r *memoryRegistry) Entries() []Registration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	registrations := make([]Registration, 0, len(r.entries))
	for taskType, handler := range r.entries {
		registrations = append(registrations, Registration{
			TaskType: taskType,
			Handler:  handler,
		})
	}
	return registrations
}
