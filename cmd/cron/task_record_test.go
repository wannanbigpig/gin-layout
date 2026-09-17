package cron

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/service/taskcenter"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestRecordedJobRecordsCallEFailure(t *testing.T) {
	fake := &fakeCronRunRecorder{}
	restore := taskcenter.SetRecorderForTesting(fake)
	defer restore()

	expectedErr := errors.New("cron failed")
	schedule := NewSchedule(&cronLogger{})
	builder := schedule.CallE("demo", func() error {
		return expectedErr
	}).EveryFiveSeconds()

	startAt := time.Now()
	schedule.recordedJob(builder.task).Run()

	if len(fake.starts) != 1 {
		t.Fatalf("expected 1 start call, got %d", len(fake.starts))
	}
	start := fake.starts[0]
	if start.TaskCode != "cron:demo" || start.Kind != model.TaskKindCron || start.Source != model.TaskSourceCron {
		t.Fatalf("unexpected start input: %#v", start)
	}
	if start.CronSpec != "0/5 * * * * *" {
		t.Fatalf("unexpected cron spec: %s", start.CronSpec)
	}
	if len(fake.finishes) != 1 {
		t.Fatalf("expected 1 finish call, got %d", len(fake.finishes))
	}
	if !errors.Is(fake.finishes[0].Error, expectedErr) {
		t.Fatalf("unexpected finish error: %v", fake.finishes[0].Error)
	}
	if fake.finishes[0].NextRunAt == nil {
		t.Fatal("expected finish next run at to be set")
	}
	if !fake.finishes[0].NextRunAt.After(startAt) {
		t.Fatalf("expected next run at after start time, got %s", fake.finishes[0].NextRunAt.Format(time.DateTime))
	}
}

func TestRecordedJobPassesSchedulerContext(t *testing.T) {
	fake := &fakeCronRunRecorder{}
	restore := taskcenter.SetRecorderForTesting(fake)
	defer restore()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var observed context.Context
	schedule := NewScheduleWithContext(ctx, &cronLogger{})
	builder := schedule.CallContextE("demo", func(runCtx context.Context) error {
		observed = runCtx
		return runCtx.Err()
	}).EveryFiveSeconds()

	schedule.recordedJob(builder.task).Run()

	if observed != ctx {
		t.Fatal("expected job to receive scheduler context")
	}
	if len(fake.finishes) != 1 || !errors.Is(fake.finishes[0].Error, context.Canceled) {
		t.Fatalf("expected context canceled finish error, got %#v", fake.finishes)
	}
}

func TestRecordedJobReleasesHighRiskLockAfterSchedulerContextCanceled(t *testing.T) {
	releaseErr := errors.New("redis release failed")
	lock := &fakeCronTaskLock{releaseErr: releaseErr, released: make(chan observedReleaseContext, 1)}
	previousAcquire := acquireCronTaskLock
	acquireCronTaskLock = func(context.Context, string) (cronTaskLock, error) {
		return lock, nil
	}
	defer func() { acquireCronTaskLock = previousAcquire }()

	core, observed := observer.New(zap.ErrorLevel)
	restoreLogger := log.ReplaceLoggerForTesting(zap.New(core))
	defer restoreLogger()

	ctx, cancel := context.WithCancel(context.Background())
	schedule := NewScheduleWithContext(ctx, &cronLogger{})
	builder := schedule.Call("high-risk", func() {}).EveryFiveSeconds()
	builder.task.isHighRisk = true
	cancel()

	schedule.recordedJob(builder.task).Run()

	select {
	case releaseCtx := <-lock.released:
		if err := releaseCtx.err; err != nil {
			t.Fatalf("expected release context to ignore scheduler cancellation, got %v", err)
		}
		if !releaseCtx.hasDeadline || time.Until(releaseCtx.deadline) <= 0 || time.Until(releaseCtx.deadline) > taskLockReleaseTimeout {
			t.Fatalf("expected bounded release deadline, got %v, %v", releaseCtx.deadline, releaseCtx.hasDeadline)
		}
	case <-time.After(time.Second):
		t.Fatal("expected high-risk task lock release to be attempted")
	}

	entries := observed.All()
	if len(entries) != 1 || !strings.Contains(entries[0].Message, "释放分布式锁失败") {
		t.Fatalf("expected observable release failure log, got %#v", entries)
	}
	if entries[0].ContextMap()["task_code"] != "cron:high-risk" {
		t.Fatalf("expected task_code log field, got %#v", entries[0].ContextMap())
	}
}

func TestRecordedJobSkipsWhenRuntimeDisabled(t *testing.T) {
	fake := &fakeCronRunRecorder{}
	restore := taskcenter.SetRecorderForTesting(fake)
	defer restore()

	called := false
	schedule := NewSchedule(&cronLogger{})
	builder := schedule.Call("demo", func() {
		called = true
	}).EnabledWhen(func(context.Context) bool {
		return false
	}).EveryFiveSeconds()

	schedule.recordedJob(builder.task).Run()

	if called {
		t.Fatal("expected disabled task not to run")
	}
	if len(fake.starts) != 0 || len(fake.finishes) != 0 {
		t.Fatalf("expected disabled task not to be recorded, got starts=%d finishes=%d", len(fake.starts), len(fake.finishes))
	}
}

func TestCalculateNextRunAtInvalidSpec(t *testing.T) {
	nextRunAt, err := calculateNextRunAt("bad spec", time.Now())
	if err == nil {
		t.Fatal("expected parse error for invalid cron spec")
	}
	if nextRunAt != nil {
		t.Fatalf("expected nil next run at on parse error, got %v", nextRunAt)
	}
}

type fakeCronRunRecorder struct {
	starts   []taskcenter.RunStart
	finishes []taskcenter.RunFinish
}

type fakeCronTaskLock struct {
	releaseErr error
	released   chan observedReleaseContext
}

func (f *fakeCronTaskLock) Release(ctx context.Context) error {
	deadline, hasDeadline := ctx.Deadline()
	f.released <- observedReleaseContext{err: ctx.Err(), deadline: deadline, hasDeadline: hasDeadline}
	return f.releaseErr
}

type observedReleaseContext struct {
	err         error
	deadline    time.Time
	hasDeadline bool
}

func (f *fakeCronRunRecorder) Enqueue(ctx context.Context, input taskcenter.RunStart) (*model.TaskRun, error) {
	_ = ctx
	f.starts = append(f.starts, input)
	return &model.TaskRun{BaseModel: model.BaseModel{ID: uint(len(f.starts))}, TaskCode: input.TaskCode, Source: input.Source}, nil
}

func (f *fakeCronRunRecorder) Start(ctx context.Context, input taskcenter.RunStart) (*model.TaskRun, error) {
	_ = ctx
	f.starts = append(f.starts, input)
	return &model.TaskRun{BaseModel: model.BaseModel{ID: uint(len(f.starts))}, TaskCode: input.TaskCode, Source: input.Source}, nil
}

func (f *fakeCronRunRecorder) Finish(ctx context.Context, run *model.TaskRun, input taskcenter.RunFinish) error {
	_ = ctx
	_ = run
	f.finishes = append(f.finishes, input)
	return nil
}
