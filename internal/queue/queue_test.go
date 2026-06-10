package queue

import (
	"context"
	"errors"
	"testing"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/config/autoload"
)

type testPayload struct {
	Name string `json:"name"`
}

func (p testPayload) Validate() error {
	if p.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

type stubPublisher struct {
	lastJob Job
}

type stubInspector struct {
	deleteCalled bool
	cancelCalled bool
	lastQueue    string
	lastTaskID   string
	lastCancelID string
	stats        *QueueStats
	statsErr     error
	allStats     []*QueueStats
	allStatsErr  error
}

func (s *stubPublisher) Enqueue(ctx context.Context, job Job) (JobInfo, error) {
	_ = ctx
	s.lastJob = job
	return JobInfo{ID: "job-1", Queue: job.Queue(), Type: job.Type()}, nil
}

func (s *stubInspector) DeleteTask(ctx context.Context, queueName, taskID string) error {
	_ = ctx
	s.deleteCalled = true
	s.lastQueue = queueName
	s.lastTaskID = taskID
	return nil
}

func (s *stubInspector) CancelProcessing(ctx context.Context, taskID string) error {
	_ = ctx
	s.cancelCalled = true
	s.lastCancelID = taskID
	return nil
}

func (s *stubInspector) GetQueueStats(ctx context.Context, queueName string) (*QueueStats, error) {
	_ = ctx
	_ = queueName
	if s.statsErr != nil {
		return nil, s.statsErr
	}
	return s.stats, nil
}

func (s *stubInspector) GetAllQueueStats(ctx context.Context) ([]*QueueStats, error) {
	_ = ctx
	if s.allStatsErr != nil {
		return nil, s.allStatsErr
	}
	return s.allStats, nil
}

func TestPublishJSONUsesGlobalPublisher(t *testing.T) {
	publisher := &stubPublisher{}
	restore := SetPublisherForTesting(publisher)
	defer restore()

	info, err := PublishJSON(context.Background(), "demo:send", "critical", testPayload{Name: "codex"}, WithMaxRetry(3))
	if err != nil {
		t.Fatalf("PublishJSON returned error: %v", err)
	}
	if info.Type != "demo:send" || info.Queue != "critical" {
		t.Fatalf("unexpected job info: %#v", info)
	}
	if publisher.lastJob == nil {
		t.Fatal("expected publisher to receive job")
	}
}

func TestRegisterJSONDecodesAndValidatesPayload(t *testing.T) {
	registry := NewRegistry()
	called := false

	RegisterJSON(registry, "demo:send", func(ctx context.Context, payload testPayload) error {
		_ = ctx
		called = true
		if payload.Name != "codex" {
			t.Fatalf("unexpected payload: %#v", payload)
		}
		return nil
	})

	entries := registry.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 registry entry, got %d", len(entries))
	}

	if err := entries[0].Handler(context.Background(), []byte(`{"name":"codex"}`)); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if !called {
		t.Fatal("expected handler to be called")
	}
}

func TestRegisterJSONReturnsSkipRetryForInvalidPayload(t *testing.T) {
	registry := NewRegistry()
	RegisterJSON(registry, "demo:send", func(ctx context.Context, payload testPayload) error {
		_ = ctx
		_ = payload
		return nil
	})

	entries := registry.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 registry entry, got %d", len(entries))
	}

	err := entries[0].Handler(context.Background(), []byte(`{}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrSkipRetry) {
		t.Fatalf("expected skip retry error, got %v", err)
	}
}

func TestInitPublisherStoresLastError(t *testing.T) {
	previousFactory := activePublisherF
	previousPublisher := activePublisher
	previousInitErr := activePublisherE
	t.Cleanup(func() {
		activePublisherF = previousFactory
		activePublisher = previousPublisher
		activePublisherE = previousInitErr
	})

	activePublisherF = func(cfg *config.Conf) (Publisher, error) {
		_ = cfg
		return nil, errTestPublisherInit
	}

	cfg := &config.Conf{
		Queue: autoload.QueueConfig{Enable: true},
	}
	err := InitPublisher(cfg)
	if !errors.Is(err, errTestPublisherInit) {
		t.Fatalf("expected init error %v, got %v", errTestPublisherInit, err)
	}
	if !errors.Is(PublisherInitError(), errTestPublisherInit) {
		t.Fatalf("expected stored init error %v, got %v", errTestPublisherInit, PublisherInitError())
	}
	if PublisherOrNil() != nil {
		t.Fatal("expected publisher to remain nil on init failure")
	}
}

func TestPublishRetriesPublisherInitAfterFailure(t *testing.T) {
	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Queue = autoload.QueueConfig{Enable: true}
	})
	defer restoreConfig()

	previousFactory := activePublisherF
	previousPublisher := activePublisher
	previousInitErr := activePublisherE
	t.Cleanup(func() {
		activePublisherF = previousFactory
		activePublisher = previousPublisher
		activePublisherE = previousInitErr
	})

	publisher := &stubPublisher{}
	activePublisher = nil
	activePublisherE = errTestPublisherInit
	activePublisherF = func(cfg *config.Conf) (Publisher, error) {
		_ = cfg
		return publisher, nil
	}

	info, err := PublishJSON(context.Background(), "demo:send", "default", testPayload{Name: "codex"})
	if err != nil {
		t.Fatalf("PublishJSON returned error: %v", err)
	}
	if info.ID != "job-1" || publisher.lastJob == nil {
		t.Fatalf("expected retry to publish job, info=%#v publisher=%#v", info, publisher)
	}
	if PublisherInitError() != nil {
		t.Fatalf("expected init error to be cleared, got %v", PublisherInitError())
	}
}

func TestDeleteTaskUsesGlobalInspector(t *testing.T) {
	inspector := &stubInspector{}
	restore := SetInspectorForTesting(inspector)
	defer restore()

	if err := DeleteTask(context.Background(), "default", "task-1"); err != nil {
		t.Fatalf("DeleteTask returned error: %v", err)
	}
	if !inspector.deleteCalled || inspector.lastQueue != "default" || inspector.lastTaskID != "task-1" {
		t.Fatalf("unexpected inspector state: %#v", inspector)
	}
}

func TestCancelProcessingUsesGlobalInspector(t *testing.T) {
	inspector := &stubInspector{}
	restore := SetInspectorForTesting(inspector)
	defer restore()

	if err := CancelProcessing(context.Background(), "task-1"); err != nil {
		t.Fatalf("CancelProcessing returned error: %v", err)
	}
	if !inspector.cancelCalled || inspector.lastCancelID != "task-1" {
		t.Fatalf("unexpected inspector state: %#v", inspector)
	}
}

func TestDeleteTaskReturnsUnavailableWithoutInspector(t *testing.T) {
	restore := SetInspectorForTesting(nil)
	defer restore()

	err := DeleteTask(context.Background(), "default", "task-1")
	if !errors.Is(err, ErrInspectorUnavailable) {
		t.Fatalf("expected ErrInspectorUnavailable, got %v", err)
	}
}

func TestDeleteTaskRetriesInspectorInitAfterFailure(t *testing.T) {
	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Queue = autoload.QueueConfig{Enable: true}
	})
	defer restoreConfig()

	previousFactory := activeInspectorF
	previousInspector := activeInspector
	previousInitErr := activeInspectorE
	t.Cleanup(func() {
		activeInspectorF = previousFactory
		activeInspector = previousInspector
		activeInspectorE = previousInitErr
	})

	inspector := &stubInspector{}
	activeInspector = nil
	activeInspectorE = errTestPublisherInit
	activeInspectorF = func(cfg *config.Conf) (Inspector, error) {
		_ = cfg
		return inspector, nil
	}

	if err := DeleteTask(context.Background(), "default", "task-1"); err != nil {
		t.Fatalf("DeleteTask returned error: %v", err)
	}
	if !inspector.deleteCalled {
		t.Fatal("expected retry to initialize inspector and delete task")
	}
	if InspectorInitError() != nil {
		t.Fatalf("expected init error to be cleared, got %v", InspectorInitError())
	}
}

func TestGetQueueStatsUsesGlobalInspector(t *testing.T) {
	want := &QueueStats{Queue: "default", Pending: 5, Active: 2, Retry: 1}
	inspector := &stubInspector{stats: want}
	restore := SetInspectorForTesting(inspector)
	defer restore()

	got, err := GetQueueStats(context.Background(), "default")
	if err != nil {
		t.Fatalf("GetQueueStats returned error: %v", err)
	}
	if got.Queue != want.Queue || got.Pending != want.Pending || got.Active != want.Active || got.Retry != want.Retry {
		t.Fatalf("unexpected stats: got=%#v want=%#v", got, want)
	}
}

func TestGetQueueStatsReturnsUnavailableWithoutInspector(t *testing.T) {
	restore := SetInspectorForTesting(nil)
	defer restore()

	_, err := GetQueueStats(context.Background(), "default")
	if !errors.Is(err, ErrInspectorUnavailable) {
		t.Fatalf("expected ErrInspectorUnavailable, got %v", err)
	}
}

func TestGetAllQueueStatsUsesGlobalInspector(t *testing.T) {
	want := []*QueueStats{
		{Queue: "default", Pending: 3, Active: 1},
		{Queue: "critical", Pending: 10, Active: 5},
	}
	inspector := &stubInspector{allStats: want}
	restore := SetInspectorForTesting(inspector)
	defer restore()

	got, err := GetAllQueueStats(context.Background())
	if err != nil {
		t.Fatalf("GetAllQueueStats returned error: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d stats, got %d", len(want), len(got))
	}
	for i, s := range got {
		if s.Queue != want[i].Queue {
			t.Fatalf("stats[%d] queue mismatch: got=%s want=%s", i, s.Queue, want[i].Queue)
		}
	}
}

func TestGetAllQueueStatsReturnsUnavailableWithoutInspector(t *testing.T) {
	restore := SetInspectorForTesting(nil)
	defer restore()

	_, err := GetAllQueueStats(context.Background())
	if !errors.Is(err, ErrInspectorUnavailable) {
		t.Fatalf("expected ErrInspectorUnavailable, got %v", err)
	}
}

func TestGetQueueStatsPropagatesError(t *testing.T) {
	inspector := &stubInspector{statsErr: errors.New("redis down")}
	restore := SetInspectorForTesting(inspector)
	defer restore()

	_, err := GetQueueStats(context.Background(), "default")
	if err == nil || err.Error() != "redis down" {
		t.Fatalf("expected 'redis down' error, got %v", err)
	}
}

var errTestPublisherInit = errors.New("publisher init failed")
