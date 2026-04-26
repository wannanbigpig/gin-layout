package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/wannanbigpig/gin-layout/config"
)

var (
	ErrPublisherUnavailable = errors.New("queue publisher unavailable")
	ErrInspectorUnavailable = errors.New("queue inspector unavailable")
	ErrQueueNotFound        = errors.New("queue not found")
	ErrTaskNotFound         = errors.New("queue task not found")
	ErrSkipRetry            = errors.New("queue skip retry")
)

const DefaultQueue = "default"

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

// Inspector 负责对已入队任务执行控制操作。
type Inspector interface {
	DeleteTask(ctx context.Context, queueName, taskID string) error
	CancelProcessing(ctx context.Context, taskID string) error
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

// Validatable 表示 payload 支持自校验。
type Validatable interface {
	Validate() error
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

type jsonJob struct {
	taskType  string
	queueName string
	payload   any
	options   []JobOption
}

type skipRetryError struct {
	err error
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

// NewJSONJob 创建一个基于 JSON payload 的通用任务。
func NewJSONJob(taskType, queueName string, payload any, opts ...JobOption) Job {
	if queueName == "" {
		queueName = DefaultQueue
	}
	return &jsonJob{
		taskType:  taskType,
		queueName: queueName,
		payload:   payload,
		options:   opts,
	}
}

// Publish 使用全局 publisher 发布任务。
func Publish(ctx context.Context, job Job) (JobInfo, error) {
	publisher := PublisherOrNil()
	if publisher == nil {
		return JobInfo{}, ErrPublisherUnavailable
	}
	return publisher.Enqueue(ctx, job)
}

// PublishJSON 发布一个 JSON 任务。
func PublishJSON(ctx context.Context, taskType, queueName string, payload any, opts ...JobOption) (JobInfo, error) {
	return Publish(ctx, NewJSONJob(taskType, queueName, payload, opts...))
}

// RegisterJSON 注册一个基于 JSON payload 的处理器。
func RegisterJSON[T any](registry Registry, taskType string, handler func(ctx context.Context, payload T) error) {
	if registry == nil || taskType == "" || handler == nil {
		return
	}
	registry.Register(taskType, func(ctx context.Context, raw []byte) error {
		var payload T
		if err := json.Unmarshal(raw, &payload); err != nil {
			return SkipRetry(fmt.Errorf("decode %s payload failed: %w", taskType, err))
		}
		if err := validatePayload(payload); err != nil {
			return SkipRetry(fmt.Errorf("invalid %s payload: %w", taskType, err))
		}
		return handler(ctx, payload)
	})
}

// SkipRetry 标记任务错误为不再重试。
func SkipRetry(err error) error {
	return &skipRetryError{err: err}
}

func (j *jsonJob) Type() string {
	return j.taskType
}

func (j *jsonJob) Queue() string {
	if j.queueName == "" {
		return DefaultQueue
	}
	return j.queueName
}

func (j *jsonJob) Payload() ([]byte, error) {
	return json.Marshal(j.payload)
}

func (j *jsonJob) Options() []JobOption {
	return append([]JobOption(nil), j.options...)
}

func (e *skipRetryError) Error() string {
	if e == nil || e.err == nil {
		return ErrSkipRetry.Error()
	}
	return e.err.Error()
}

func (e *skipRetryError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func (e *skipRetryError) Is(target error) bool {
	return target == ErrSkipRetry
}

type publisherFactory func(cfg *config.Conf) (Publisher, error)
type inspectorFactory func(cfg *config.Conf) (Inspector, error)

var (
	publisherMu      sync.RWMutex
	activePublisher  Publisher
	activePublisherF publisherFactory
	activePublisherE error

	inspectorMu      sync.RWMutex
	activeInspector  Inspector
	activeInspectorF inspectorFactory
	activeInspectorE error
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
		activePublisherE = nil
		return nil
	}
	if activePublisherF == nil {
		activePublisher = nil
		activePublisherE = errors.New("queue publisher factory not registered")
		return activePublisherE
	}

	publisher, err := activePublisherF(cfg)
	if err != nil {
		activePublisher = nil
		activePublisherE = err
		return err
	}
	activePublisher = publisher
	activePublisherE = nil
	return nil
}

// RegisterInspectorFactory 注册默认的 inspector 构建器。
func RegisterInspectorFactory(factory func(cfg *config.Conf) (Inspector, error)) {
	inspectorMu.Lock()
	defer inspectorMu.Unlock()
	activeInspectorF = factory
}

// InitInspector 根据当前配置初始化全局 inspector。
func InitInspector(cfg *config.Conf) error {
	inspectorMu.Lock()
	defer inspectorMu.Unlock()

	if cfg == nil || !cfg.Queue.Enable {
		activeInspector = nil
		activeInspectorE = nil
		return nil
	}
	if activeInspectorF == nil {
		activeInspector = nil
		activeInspectorE = errors.New("queue inspector factory not registered")
		return activeInspectorE
	}

	inspector, err := activeInspectorF(cfg)
	if err != nil {
		activeInspector = nil
		activeInspectorE = err
		return err
	}
	activeInspector = inspector
	activeInspectorE = nil
	return nil
}

// PublisherOrNil 返回当前全局 publisher；未启用时返回 nil。
func PublisherOrNil() Publisher {
	publisherMu.RLock()
	defer publisherMu.RUnlock()
	return activePublisher
}

// PublisherInitError 返回最近一次 publisher 初始化错误。
func PublisherInitError() error {
	publisherMu.RLock()
	defer publisherMu.RUnlock()
	return activePublisherE
}

// InspectorOrNil 返回当前全局 inspector；未启用时返回 nil。
func InspectorOrNil() Inspector {
	inspectorMu.RLock()
	defer inspectorMu.RUnlock()
	return activeInspector
}

// InspectorInitError 返回最近一次 inspector 初始化错误。
func InspectorInitError() error {
	inspectorMu.RLock()
	defer inspectorMu.RUnlock()
	return activeInspectorE
}

// DeleteTask 删除队列中的任务（pending/scheduled/retry/archived）。
func DeleteTask(ctx context.Context, queueName, taskID string) error {
	inspector := InspectorOrNil()
	if inspector == nil {
		return ErrInspectorUnavailable
	}
	return inspector.DeleteTask(ctx, queueName, taskID)
}

// CancelProcessing 发送取消正在执行任务的信号（best-effort）。
func CancelProcessing(ctx context.Context, taskID string) error {
	inspector := InspectorOrNil()
	if inspector == nil {
		return ErrInspectorUnavailable
	}
	return inspector.CancelProcessing(ctx, taskID)
}

// SetPublisherForTesting 仅用于测试时替换全局 publisher。
func SetPublisherForTesting(publisher Publisher) func() {
	publisherMu.Lock()
	previous := activePublisher
	previousErr := activePublisherE
	activePublisher = publisher
	activePublisherE = nil
	publisherMu.Unlock()

	return func() {
		publisherMu.Lock()
		activePublisher = previous
		activePublisherE = previousErr
		publisherMu.Unlock()
	}
}

// SetInspectorForTesting 仅用于测试时替换全局 inspector。
func SetInspectorForTesting(inspector Inspector) func() {
	inspectorMu.Lock()
	previous := activeInspector
	previousErr := activeInspectorE
	activeInspector = inspector
	activeInspectorE = nil
	inspectorMu.Unlock()

	return func() {
		inspectorMu.Lock()
		activeInspector = previous
		activeInspectorE = previousErr
		inspectorMu.Unlock()
	}
}

func validatePayload(payload any) error {
	if payload == nil {
		return nil
	}
	if validatable, ok := payload.(Validatable); ok {
		return validatable.Validate()
	}
	return nil
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
