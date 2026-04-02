package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/wannanbigpig/gin-layout/config"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/queue"
	auditsvc "github.com/wannanbigpig/gin-layout/internal/service/audit"
	"go.uber.org/zap"
)

const (
	AuditLogTaskType = "audit:request_log.write"
	AuditQueueName   = "audit"

	AuditLogKindRequest = "request"
	AuditLogKindPanic   = "panic"
)

var persistAuditLogFunc = auditsvc.PersistAuditLog

// AuditLogPayload 表示异步审计日志任务 payload。
type AuditLogPayload struct {
	Kind     string                     `json:"kind"`
	Snapshot *auditsvc.AuditLogSnapshot `json:"snapshot"`
}

// AuditLogJob 表示异步审计日志任务。
type AuditLogJob struct {
	payload AuditLogPayload
}

// NewAuditLogJob 创建一个审计日志任务。
func NewAuditLogJob(kind string, snapshot *auditsvc.AuditLogSnapshot) (*AuditLogJob, error) {
	payload := AuditLogPayload{
		Kind:     kind,
		Snapshot: snapshot,
	}
	if err := payload.Validate(); err != nil {
		return nil, err
	}
	return &AuditLogJob{payload: payload}, nil
}

func (j *AuditLogJob) Type() string {
	return AuditLogTaskType
}

func (j *AuditLogJob) Queue() string {
	return AuditQueueName
}

func (j *AuditLogJob) Payload() ([]byte, error) {
	return json.Marshal(j.payload)
}

func (j *AuditLogJob) Options() []queue.JobOption {
	cfg := config.GetConfig()
	maxRetry := 3
	timeout := 10 * time.Second
	if cfg != nil {
		if cfg.Queue.AuditMaxRetry > 0 {
			maxRetry = cfg.Queue.AuditMaxRetry
		}
		if cfg.Queue.AuditTimeoutSeconds > 0 {
			timeout = time.Duration(cfg.Queue.AuditTimeoutSeconds) * time.Second
		}
	}
	return []queue.JobOption{
		queue.WithMaxRetry(maxRetry),
		queue.WithTimeout(timeout),
	}
}

// Validate 校验 payload 是否满足最小要求。
func (p AuditLogPayload) Validate() error {
	if p.Kind != AuditLogKindRequest && p.Kind != AuditLogKindPanic {
		return fmt.Errorf("invalid audit log kind %q", p.Kind)
	}
	if p.Snapshot == nil || p.Snapshot.RequestID == "" {
		return fmt.Errorf("audit log snapshot is invalid")
	}
	return nil
}

// EnqueueAuditLog 发布异步审计日志任务。
func EnqueueAuditLog(ctx context.Context, kind string, snapshot *auditsvc.AuditLogSnapshot) error {
	publisher := queue.PublisherOrNil()
	if publisher == nil {
		return queue.ErrPublisherUnavailable
	}
	job, err := NewAuditLogJob(kind, snapshot)
	if err != nil {
		return err
	}
	_, err = publisher.Enqueue(ctx, job)
	return err
}

// AuditLogHandler 负责消费审计日志任务并落库。
type AuditLogHandler struct{}

func (h AuditLogHandler) Handle(ctx context.Context, payload []byte) error {
	var request AuditLogPayload
	if err := json.Unmarshal(payload, &request); err != nil {
		return fmt.Errorf("decode audit log payload failed: %w", queue.ErrSkipRetry)
	}
	if err := request.Validate(); err != nil {
		return fmt.Errorf("invalid audit log payload: %w", queue.ErrSkipRetry)
	}
	if err := persistAuditLogFunc(request.Snapshot); err != nil {
		log.Error("Persist audit log failed",
			zap.String("request_id", request.Snapshot.RequestID),
			zap.Error(err))
		return err
	}
	return nil
}

// RegisterAll 注册当前版本的全部异步任务。
func RegisterAll(registry queue.Registry) {
	if registry == nil {
		return
	}
	registry.Register(AuditLogTaskType, AuditLogHandler{}.Handle)
}
