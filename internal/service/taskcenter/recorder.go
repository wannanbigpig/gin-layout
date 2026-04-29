package taskcenter

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

const (
	maxPayloadBytes = 64 * 1024
	maxErrorBytes   = 8 * 1024
)

// RunStart 描述一次任务开始执行时的上下文。
type RunStart struct {
	TaskCode       string
	Kind           string
	Source         string
	SourceID       string
	Queue          string
	CronSpec       string
	Attempt        int
	MaxRetry       int
	Payload        []byte
	TriggerUserID  uint
	TriggerAccount string
	TriggerConfirm string
	TriggerReason  string
}

// RunFinish 描述一次任务执行结束时的结果。
type RunFinish struct {
	Status            string
	Error             error
	NextRunAt         *time.Time
	CanceledBy        uint
	CanceledByAccount string
	CancelReason      string
}

// Recorder 持久化任务执行记录。
type Recorder interface {
	Enqueue(ctx context.Context, input RunStart) (*model.TaskRun, error)
	Start(ctx context.Context, input RunStart) (*model.TaskRun, error)
	Finish(ctx context.Context, run *model.TaskRun, input RunFinish) error
}

type recorder struct {
	db *gorm.DB
}

var (
	factoryMu       sync.RWMutex
	recorderFactory = func() Recorder {
		return NewRecorder()
	}
)

// NewRecorder 创建使用全局数据库连接的任务执行记录器。
func NewRecorder() Recorder {
	return &recorder{}
}

// NewRecorderWithDB 创建使用指定数据库连接的任务执行记录器，主要用于测试。
func NewRecorderWithDB(db *gorm.DB) Recorder {
	return &recorder{db: db}
}

// NewRunRecorder 返回当前默认记录器。
func NewRunRecorder() Recorder {
	factoryMu.RLock()
	defer factoryMu.RUnlock()
	return recorderFactory()
}

// SetRecorderForTesting 临时替换默认记录器。
func SetRecorderForTesting(next Recorder) func() {
	factoryMu.Lock()
	previous := recorderFactory
	recorderFactory = func() Recorder {
		return next
	}
	factoryMu.Unlock()

	return func() {
		factoryMu.Lock()
		recorderFactory = previous
		factoryMu.Unlock()
	}
}

func (r *recorder) Start(ctx context.Context, input RunStart) (*model.TaskRun, error) {
	if strings.TrimSpace(input.TaskCode) == "" {
		return nil, errors.New("task code is required")
	}
	if input.Kind == "" {
		input.Kind = model.TaskKindAsync
	}
	if input.Source == "" {
		input.Source = model.TaskSourceQueue
	}

	db, err := r.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}

	now := utils.FormatDate{Time: time.Now()}
	run := model.NewTaskRun()
	run.TaskCode = input.TaskCode
	run.Kind = input.Kind
	run.Source = input.Source
	run.SourceID = input.SourceID
	run.Queue = input.Queue
	run.TriggerUserID = input.TriggerUserID
	run.TriggerAccount = input.TriggerAccount
	run.Status = model.TaskRunStatusRunning
	run.Attempt = input.Attempt
	run.MaxRetry = input.MaxRetry
	run.Payload = truncateBytes(input.Payload, maxPayloadBytes)
	run.StartedAt = &now

	err = db.Transaction(func(tx *gorm.DB) error {
		// 如果存在同 source_id 的 pending/retrying 记录（例如手动触发后进入 worker），就复用该记录并推进状态。
		if input.SourceID != "" {
			var existing model.TaskRun
			findErr := tx.Where("task_code = ? AND source_id = ? AND status IN ?",
				input.TaskCode, input.SourceID, []string{model.TaskRunStatusPending, model.TaskRunStatusRetrying}).
				Order("id DESC").First(&existing).Error
			if findErr == nil {
				run = &existing
				run.Kind = input.Kind
				run.Source = input.Source
				run.Queue = input.Queue
				run.Status = model.TaskRunStatusRunning
				run.Attempt = input.Attempt
				run.MaxRetry = input.MaxRetry
				run.Payload = truncateBytes(input.Payload, maxPayloadBytes)
				run.StartedAt = &now
				if err := tx.Model(&model.TaskRun{}).Where("id = ?", run.ID).Updates(map[string]any{
					"kind":       run.Kind,
					"source":     run.Source,
					"queue":      run.Queue,
					"status":     run.Status,
					"attempt":    run.Attempt,
					"max_retry":  run.MaxRetry,
					"payload":    run.Payload,
					"started_at": run.StartedAt,
				}).Error; err != nil {
					return err
				}
				if err := tx.Create(newEvent(run.ID, model.TaskEventStart, "task started", inputMeta(input))).Error; err != nil {
					return err
				}
				if input.Source == model.TaskSourceCron {
					return upsertCronState(tx, run, input.CronSpec, "", nil)
				}
				return nil
			}
			if !errors.Is(findErr, gorm.ErrRecordNotFound) {
				return findErr
			}
		}

		if err := tx.Create(run).Error; err != nil {
			return err
		}
		if err := tx.Create(newEvent(run.ID, model.TaskEventStart, "task started", inputMeta(input))).Error; err != nil {
			return err
		}
		if input.Source == model.TaskSourceCron {
			return upsertCronState(tx, run, input.CronSpec, "", nil)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return run, nil
}

func (r *recorder) Enqueue(ctx context.Context, input RunStart) (*model.TaskRun, error) {
	if strings.TrimSpace(input.TaskCode) == "" {
		return nil, errors.New("task code is required")
	}
	if input.Kind == "" {
		input.Kind = model.TaskKindAsync
	}
	if input.Source == "" {
		input.Source = model.TaskSourceQueue
	}

	db, err := r.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}

	run := model.NewTaskRun()
	run.TaskCode = input.TaskCode
	run.Kind = input.Kind
	run.Source = input.Source
	run.SourceID = input.SourceID
	run.Queue = input.Queue
	run.TriggerUserID = input.TriggerUserID
	run.TriggerAccount = input.TriggerAccount
	run.Status = model.TaskRunStatusPending
	run.Attempt = input.Attempt
	run.MaxRetry = input.MaxRetry
	run.Payload = truncateBytes(input.Payload, maxPayloadBytes)

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(run).Error; err != nil {
			return err
		}
		return tx.Create(newEvent(run.ID, model.TaskEventEnqueue, "task enqueued", inputMeta(input))).Error
	}); err != nil {
		return nil, err
	}
	return run, nil
}

func (r *recorder) Finish(ctx context.Context, run *model.TaskRun, input RunFinish) error {
	if run == nil || run.ID == 0 {
		return nil
	}
	status := input.Status
	if status == "" {
		status = model.TaskRunStatusSuccess
		if input.Error != nil {
			status = model.TaskRunStatusFailed
		}
	}

	db, err := r.dbWithContext(ctx)
	if err != nil {
		return err
	}

	finishedAt := utils.FormatDate{Time: time.Now()}
	errorMessage := ""
	eventType, eventMessage := resolveFinishEvent(status)
	if input.Error != nil {
		errorMessage = truncateString(input.Error.Error(), maxErrorBytes)
		eventType = model.TaskEventFail
		eventMessage = errorMessage
	}
	input.Status = status

	durationMS := float64(0)
	if run.StartedAt != nil && !run.StartedAt.IsZero() {
		duration := finishedAt.Time.Sub(run.StartedAt.Time)
		durationMS = float64(duration.Nanoseconds()) / 1000000.0
		durationMS = float64(int(durationMS*10000+0.5)) / 10000.0
	}

	run.Status = status
	run.ErrorMessage = errorMessage
	run.FinishedAt = &finishedAt
	run.DurationMS = durationMS

	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.TaskRun{}).Where("id = ?", run.ID).Updates(map[string]any{
			"status":        run.Status,
			"error_message": run.ErrorMessage,
			"finished_at":   run.FinishedAt,
			"duration_ms":   run.DurationMS,
		}).Error; err != nil {
			return err
		}
		if err := tx.Create(newEvent(run.ID, eventType, eventMessage, finishMeta(input))).Error; err != nil {
			return err
		}
		if run.Source == model.TaskSourceCron {
			return upsertCronState(tx, run, "", errorMessage, input.NextRunAt)
		}
		return nil
	})
}

func resolveFinishEvent(status string) (eventType string, message string) {
	switch status {
	case model.TaskRunStatusFailed:
		return model.TaskEventFail, "task failed"
	case model.TaskRunStatusCanceled:
		return model.TaskEventCancel, "task canceled"
	case model.TaskRunStatusRetrying:
		return model.TaskEventRetry, "task retrying"
	default:
		return model.TaskEventSuccess, "task succeeded"
	}
}

func (r *recorder) dbWithContext(ctx context.Context) (*gorm.DB, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if r.db != nil {
		return r.db.WithContext(ctx), nil
	}
	db, err := model.GetDB()
	if err != nil {
		return nil, err
	}
	return db.WithContext(ctx), nil
}

func newEvent(runID uint, eventType, message string, meta map[string]any) *model.TaskRunEvent {
	return &model.TaskRunEvent{
		RunID:     runID,
		EventType: eventType,
		Message:   truncateString(message, maxErrorBytes),
		Meta:      marshalMeta(meta),
	}
}

func inputMeta(input RunStart) map[string]any {
	meta := map[string]any{
		"kind":            input.Kind,
		"source":          input.Source,
		"source_id":       input.SourceID,
		"queue":           input.Queue,
		"attempt":         input.Attempt,
		"max_retry":       input.MaxRetry,
		"trigger_user_id": input.TriggerUserID,
		"cron_spec":       input.CronSpec,
	}
	if strings.TrimSpace(input.TriggerAccount) != "" {
		meta["trigger_account"] = strings.TrimSpace(input.TriggerAccount)
	}
	if strings.TrimSpace(input.TriggerConfirm) != "" {
		meta["trigger_confirm"] = strings.TrimSpace(input.TriggerConfirm)
	}
	if strings.TrimSpace(input.TriggerReason) != "" {
		meta["trigger_reason"] = strings.TrimSpace(input.TriggerReason)
	}
	return meta
}

func finishMeta(input RunFinish) map[string]any {
	meta := map[string]any{
		"status": input.Status,
	}
	if input.NextRunAt != nil {
		meta["next_run_at"] = input.NextRunAt.Format("2006-01-02 15:04:05")
	}
	if input.CanceledBy > 0 {
		meta["canceled_by"] = input.CanceledBy
	}
	if strings.TrimSpace(input.CanceledByAccount) != "" {
		meta["canceled_by_account"] = strings.TrimSpace(input.CanceledByAccount)
	}
	if input.CancelReason != "" {
		meta["cancel_reason"] = strings.TrimSpace(input.CancelReason)
	}
	return meta
}

func marshalMeta(meta map[string]any) string {
	if len(meta) == 0 {
		return "{}"
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func upsertCronState(tx *gorm.DB, run *model.TaskRun, cronSpec string, lastError string, nextRunAt *time.Time) error {
	if run == nil || run.TaskCode == "" {
		return nil
	}

	state := model.NewCronTaskState()
	state.TaskCode = run.TaskCode
	state.CronSpec = cronSpec
	state.LastRunID = run.ID
	state.LastStatus = run.Status
	state.LastStartedAt = run.StartedAt
	state.LastFinishedAt = run.FinishedAt
	state.LastError = truncateString(lastError, maxErrorBytes)
	if nextRunAt != nil {
		state.NextRunAt = &utils.FormatDate{Time: *nextRunAt}
	}

	assignments := []string{
		"last_run_id",
		"last_status",
		"last_started_at",
		"last_finished_at",
		"next_run_at",
		"last_error",
		"updated_at",
	}
	if cronSpec != "" {
		assignments = append(assignments, "cron_spec")
	}

	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "task_code"}},
		DoUpdates: clause.AssignmentColumns(assignments),
	}).Create(state).Error
}

func truncateBytes(raw []byte, limit int) string {
	if len(raw) == 0 {
		return ""
	}
	if len(raw) <= limit {
		return string(raw)
	}
	return string(raw[:limit]) + "...(truncated)"
}

func truncateString(raw string, limit int) string {
	if raw == "" || len(raw) <= limit {
		return raw
	}
	return raw[:limit] + "...(truncated)"
}
