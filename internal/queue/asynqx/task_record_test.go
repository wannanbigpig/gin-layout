package asynqx

import (
	"context"
	"errors"
	"testing"

	"github.com/hibiken/asynq"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/service/taskcenter"
)

func TestRecordAsynqTaskStartAndFinish(t *testing.T) {
	fake := &fakeRunRecorder{}
	restore := taskcenter.SetRecorderForTesting(fake)
	defer restore()

	task := asynq.NewTask("demo:send", []byte(`{"name":"codex"}`))
	run := recordAsynqTaskStart(context.Background(), task)
	if run == nil {
		t.Fatal("expected run to be returned")
	}
	recordAsynqTaskFinish(context.Background(), run, errors.New("boom"))

	if len(fake.starts) != 1 {
		t.Fatalf("expected 1 start call, got %d", len(fake.starts))
	}
	start := fake.starts[0]
	if start.TaskCode != "demo:send" || start.Kind != model.TaskKindAsync || start.Source != model.TaskSourceQueue {
		t.Fatalf("unexpected start input: %#v", start)
	}
	if string(start.Payload) != `{"name":"codex"}` {
		t.Fatalf("unexpected payload: %s", string(start.Payload))
	}
	if len(fake.finishes) != 1 {
		t.Fatalf("expected 1 finish call, got %d", len(fake.finishes))
	}
	if fake.finishes[0].Error == nil || fake.finishes[0].Error.Error() != "boom" {
		t.Fatalf("unexpected finish input: %#v", fake.finishes[0])
	}
}

type fakeRunRecorder struct {
	starts   []taskcenter.RunStart
	finishes []taskcenter.RunFinish
}

func (f *fakeRunRecorder) Enqueue(ctx context.Context, input taskcenter.RunStart) (*model.TaskRun, error) {
	_ = ctx
	f.starts = append(f.starts, input)
	return &model.TaskRun{BaseModel: model.BaseModel{ID: uint(len(f.starts))}, TaskCode: input.TaskCode, Source: input.Source, SourceID: input.SourceID}, nil
}

func (f *fakeRunRecorder) Start(ctx context.Context, input taskcenter.RunStart) (*model.TaskRun, error) {
	_ = ctx
	f.starts = append(f.starts, input)
	return &model.TaskRun{BaseModel: model.BaseModel{ID: uint(len(f.starts))}, TaskCode: input.TaskCode, Source: input.Source, SourceID: input.SourceID}, nil
}

func (f *fakeRunRecorder) Finish(ctx context.Context, run *model.TaskRun, input taskcenter.RunFinish) error {
	_ = ctx
	_ = run
	f.finishes = append(f.finishes, input)
	return nil
}
