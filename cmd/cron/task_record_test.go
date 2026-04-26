package cron

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/service/taskcenter"
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
