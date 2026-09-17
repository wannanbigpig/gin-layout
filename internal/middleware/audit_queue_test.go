package middleware

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/queue"
	auditsvc "github.com/wannanbigpig/gin-layout/internal/service/audit"
)

func itoa(n int) string { return strconv.Itoa(n) }

func TestEnqueueAuditLogDelegatesToPublisher(t *testing.T) {
	called := false
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		Enqueue: func(ctx context.Context, kind string, snapshot *auditsvc.AuditLogSnapshot) error {
			called = true
			if kind != "request" {
				t.Fatalf("unexpected kind: %s", kind)
			}
			if snapshot == nil || snapshot.RequestID != "req-1" {
				t.Fatalf("unexpected snapshot: %#v", snapshot)
			}
			return nil
		},
	})
	defer restoreDeps()

	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Queue.Enable = true
	})
	defer restoreConfig()

	restoreLogger := log.ReplaceLoggerForTesting(zap.NewNop())
	defer restoreLogger()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/demo", bytes.NewBufferString(`{"name":"codex"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set(global.ContextKeyRequestID, "req-1")
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())
	cacheRequestBody(ctx)

	respRecorder := createResponseRecorder(ctx)
	respRecorder.body.WriteString(`{"code":0,"msg":"ok","data":{}}`)

	finalizeRequestLogging(ctx, respRecorder)
	if !called {
		t.Fatal("expected enqueue to be called")
	}
}

func TestEnqueueAuditLogFailureDoesNotPanic(t *testing.T) {
	// 先清掉前序用例可能泄漏的 worker，保证干净起点。
	resetAuditFallbackBufferForTesting()
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		Enqueue: func(ctx context.Context, kind string, snapshot *auditsvc.AuditLogSnapshot) error {
			return errors.New("enqueue failed")
		},
		PersistBatch: func(ctx context.Context, snapshots []*auditsvc.AuditLogSnapshot) error {
			return nil
		},
	})
	defer restoreDeps()
	// 注意 defer 为 LIFO：reset 在 restoreDeps 之前执行，确保停止 worker / drain 时仍是本用例的 no-op 依赖，
	// 不会在依赖还原成真实落库函数后触发真实 DB 写入。
	defer resetAuditFallbackBufferForTesting()

	enqueueAuditLog(nil, "request", &auditsvc.AuditLogSnapshot{RequestID: "req-2"})
}

func TestEnqueueAuditLogResetsUnavailableFlagAfterSuccess(t *testing.T) {
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		Enqueue: func(ctx context.Context, kind string, snapshot *auditsvc.AuditLogSnapshot) error {
			return nil
		},
	})
	defer restoreDeps()

	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Queue.Enable = true
	})
	defer restoreConfig()

	queueUnavailableLogged.Store(true)
	enqueueAuditLog(nil, "request", &auditsvc.AuditLogSnapshot{RequestID: "req-3"})

	if queueUnavailableLogged.Load() {
		t.Fatal("expected unavailable flag to be reset after successful enqueue")
	}
}

func TestEnqueueAuditLogMarksUnavailableWhenPublisherUnavailable(t *testing.T) {
	resetAuditFallbackBufferForTesting()
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		Enqueue: func(ctx context.Context, kind string, snapshot *auditsvc.AuditLogSnapshot) error {
			return queue.ErrPublisherUnavailable
		},
		PersistBatch: func(ctx context.Context, snapshots []*auditsvc.AuditLogSnapshot) error {
			return nil
		},
	})
	defer restoreDeps()
	defer resetAuditFallbackBufferForTesting()

	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Queue.Enable = true
	})
	defer restoreConfig()

	queueUnavailableLogged.Store(false)
	// 复位存储熔断冷却状态，避免前序用例残留导致 enqueueAuditLog 走冷却短路分支而漏设标志位。
	storageUnavailableSince.Store(0)
	enqueueAuditLog(nil, "request", &auditsvc.AuditLogSnapshot{RequestID: "req-4"})

	if !queueUnavailableLogged.Load() {
		t.Fatal("expected unavailable flag to be set when publisher unavailable")
	}
}

func TestEnqueueAuditLogPersistsSynchronouslyWhenQueueDisabled(t *testing.T) {
	resetAuditFallbackBufferForTesting()
	defer resetAuditFallbackBufferForTesting()
	persistCalled := false

	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		Enqueue: func(ctx context.Context, kind string, snapshot *auditsvc.AuditLogSnapshot) error {
			t.Fatal("enqueue should not be called when queue is disabled")
			return nil
		},
		PersistBatch: func(ctx context.Context, snapshots []*auditsvc.AuditLogSnapshot) error {
			persistCalled = true
			if len(snapshots) != 1 || snapshots[0].RequestID != "req-db-unavailable" {
				t.Fatalf("unexpected snapshots: %#v", snapshots)
			}
			return nil
		},
	})
	defer restoreDeps()

	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Queue.Enable = false
	})
	defer restoreConfig()

	storageUnavailable.Store(false)
	enqueueAuditLog(nil, "request", &auditsvc.AuditLogSnapshot{RequestID: "req-db-unavailable"})

	resetAuditFallbackBufferForTesting()

	if !persistCalled {
		t.Fatal("expected synchronous persistence when queue is disabled")
	}
}

func TestEnqueueAuditLogHandlesStorageUnavailableWhenQueueDisabled(t *testing.T) {
	resetAuditFallbackBufferForTesting()
	defer resetAuditFallbackBufferForTesting()
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		PersistBatch: func(ctx context.Context, snapshots []*auditsvc.AuditLogSnapshot) error {
			return model.ErrDBUninitialized
		},
	})
	defer restoreDeps()

	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Queue.Enable = false
	})
	defer restoreConfig()

	storageUnavailable.Store(false)
	enqueueAuditLog(nil, "request", &auditsvc.AuditLogSnapshot{RequestID: "req-db-unavailable"})

	resetAuditFallbackBufferForTesting()

	if !storageUnavailable.Load() {
		t.Fatal("expected storage unavailable flag to be set when sync persistence fails")
	}
}

func TestReportAuditPersistenceResultMarksStorageUnavailable(t *testing.T) {
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		Persist: func(_ context.Context, snapshot *auditsvc.AuditLogSnapshot) error {
			return model.ErrDBUninitialized
		},
	})
	defer restoreDeps()

	storageUnavailable.Store(false)
	reportAuditPersistenceResult("request", &auditsvc.AuditLogSnapshot{RequestID: "req-db-unavailable"}, "sync_direct")

	if !storageUnavailable.Load() {
		t.Fatal("expected storage unavailable flag to be set when db is unavailable")
	}
}

func TestReportAuditPersistenceResultResetsStorageUnavailableAfterSuccess(t *testing.T) {
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		Persist: func(_ context.Context, snapshot *auditsvc.AuditLogSnapshot) error {
			return nil
		},
	})
	defer restoreDeps()

	storageUnavailable.Store(true)
	reportAuditPersistenceResult("request", &auditsvc.AuditLogSnapshot{RequestID: "req-db-ok"}, "sync_direct")

	if storageUnavailable.Load() {
		t.Fatal("expected storage unavailable flag to be reset after successful persistence")
	}
}

// TestAuditStorageCooldownReopensAfterExpiry 守护熔断半开恢复：
// 存储不可用进入冷却期后会短路，冷却期过后应重新放行探测，避免 DB 短暂故障导致审计日志永久丢弃。
func TestAuditStorageCooldownReopensAfterExpiry(t *testing.T) {
	// 包级全局状态可能被前序测试污染，先复位到"可用"。
	markAuditStorageAvailable()
	defer markAuditStorageAvailable()

	// 标记不可用：冷却期内应短路。
	if first := markAuditStorageUnavailable(); !first {
		t.Fatal("expected first transition to report true")
	}
	if !auditStorageInCooldown() {
		t.Fatal("expected short-circuit while within cooldown window")
	}
	// 再次标记（持续故障）不应再报首次转换，但仍处于冷却期。
	if second := markAuditStorageUnavailable(); second {
		t.Fatal("expected non-first transition to report false during outage")
	}

	// 模拟冷却期已过：把上次不可用时刻推到 cooldown 之前，应放行探测。
	storageUnavailableSince.Store(time.Now().Add(-auditStorageRetryCooldown - time.Second).UnixNano())
	if auditStorageInCooldown() {
		t.Fatal("expected cooldown to reopen after retry window elapsed")
	}

	// 一次成功落库后应彻底复位。
	markAuditStorageAvailable()
	if auditStorageInCooldown() {
		t.Fatal("expected storage to be available after reset")
	}
}

// TestShouldDropBySamplingKeepsOneOutOfN 验证 1/N 采样：N=4 时每 4 条保留 1 条。
func TestShouldDropBySamplingKeepsOneOutOfN(t *testing.T) {
	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Queue.AuditSampleGetKeepEvery = 4
	})
	defer restoreConfig()

	resetAuditFallbackBufferForTesting()
	defer resetAuditFallbackBufferForTesting()

	// 准备低风险只读快照
	snap := &auditsvc.AuditLogSnapshot{
		RequestID:       "req-sample",
		Method:          http.MethodGet,
		IsHighRisk:      0,
		ResponseStatus:  http.StatusOK,
		OperationStatus: 0,
	}

	atomicResetSampleCounter()
	kept := 0
	for i := 0; i < 12; i++ {
		if !shouldDropBySampling(snap) {
			kept++
		}
	}
	if kept != 3 {
		t.Fatalf("expected 3 kept out of 12 with keep_every=4, got %d", kept)
	}
}

func TestShouldDropBySamplingAlwaysKeepsAuthenticatedRequests(t *testing.T) {
	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Queue.AuditSampleGetKeepEvery = 1000
	})
	defer restoreConfig()

	resetSampleCounterForTesting()
	snapshot := &auditsvc.AuditLogSnapshot{
		RequestID:       "req-authenticated",
		OperatorID:      42,
		Method:          http.MethodGet,
		ResponseStatus:  http.StatusOK,
		OperationStatus: 0,
	}
	for i := 0; i < 20; i++ {
		if shouldDropBySampling(snapshot) {
			t.Fatalf("authenticated request must not be sampled out (iteration %d)", i)
		}
	}
}

func TestFinalizeRequestLoggingRecordsStatsBeforeAuditSampling(t *testing.T) {
	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Queue.Enable = true
		cfg.Queue.AuditSampleGetKeepEvery = 10
		cfg.Queue.AuditSampleSlowKeepThresholdMs = 1000
	})
	defer restoreConfig()

	atomicResetSampleCounter()
	resetAuditFallbackBufferForTesting()
	defer resetAuditFallbackBufferForTesting()

	statsCount := 0
	enqueued := 0
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		Enqueue: func(_ context.Context, _ string, _ *auditsvc.AuditLogSnapshot) error {
			enqueued++
			return nil
		},
		RecordStat: func(_ context.Context, _ *auditsvc.AuditLogSnapshot) error {
			statsCount++
			return nil
		},
	})
	defer restoreDeps()

	for i := 0; i < 3; i++ {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/demo", nil)
		ctx.Set(global.ContextKeyRequestID, "req-sampled-"+itoa(i))
		ctx.Set(global.ContextKeyRequestStartTime, time.Now().Add(-10*time.Millisecond))

		respRecorder := createResponseRecorder(ctx)
		finalizeRequestLogging(ctx, respRecorder)
	}

	if statsCount != 3 {
		t.Fatalf("expected stats to record every request before sampling, got %d", statsCount)
	}
	if enqueued != 0 {
		t.Fatalf("expected sampled requests not to be enqueued, got %d", enqueued)
	}
}

func TestFinalizeRequestLoggingSkipsStatsAndAuditForProbePath(t *testing.T) {
	statsCount := 0
	enqueued := 0
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		Enqueue: func(_ context.Context, _ string, _ *auditsvc.AuditLogSnapshot) error {
			enqueued++
			return nil
		},
		RecordStat: func(_ context.Context, _ *auditsvc.AuditLogSnapshot) error {
			statsCount++
			return nil
		},
	})
	defer restoreDeps()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/ping", nil)
	ctx.Set(global.ContextKeyRequestID, "req-ping")
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())

	respRecorder := createResponseRecorder(ctx)
	finalizeRequestLogging(ctx, respRecorder)

	if statsCount != 0 {
		t.Fatalf("expected probe path not to affect business request stats, got %d", statsCount)
	}
	if enqueued != 0 {
		t.Fatalf("expected skipped audit path not to enqueue audit log, got %d", enqueued)
	}
}

func TestReadinessPathSkipsBusinessStatsAndAuditLog(t *testing.T) {
	statsCount := 0
	enqueued := 0
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		Enqueue: func(_ context.Context, _ string, _ *auditsvc.AuditLogSnapshot) error {
			enqueued++
			return nil
		},
		RecordStat: func(_ context.Context, _ *auditsvc.AuditLogSnapshot) error {
			statsCount++
			return nil
		},
	})
	defer restoreDeps()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/health/readiness", nil)
	ctx.Set(global.ContextKeyRequestID, "req-readiness")
	ctx.Set(global.ContextKeyRequestStartTime, time.Now())

	finalizeRequestLogging(ctx, createResponseRecorder(ctx))

	if statsCount != 0 {
		t.Fatalf("expected readiness not to affect business request stats, got %d", statsCount)
	}
	if enqueued != 0 {
		t.Fatalf("expected readiness not to enter request logs, got %d", enqueued)
	}
}

// TestShouldDropBySamplingDoesNotDropHighRiskOrWrites 验证高风险与写请求不参与采样丢弃。
func TestShouldDropBySamplingDoesNotDropHighRiskOrWrites(t *testing.T) {
	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Queue.AuditSampleGetKeepEvery = 1000
	})
	defer restoreConfig()

	atomicResetSampleCounter()
	highRisk := &auditsvc.AuditLogSnapshot{RequestID: "r", Method: http.MethodGet, IsHighRisk: 1, ResponseStatus: http.StatusOK}
	if shouldDropBySampling(highRisk) {
		t.Fatal("high risk request must not be sampled out")
	}
	writeReq := &auditsvc.AuditLogSnapshot{RequestID: "r", Method: http.MethodPost, IsHighRisk: 0, ResponseStatus: http.StatusOK}
	if shouldDropBySampling(writeReq) {
		t.Fatal("non-GET request must not be sampled out")
	}
	errResp := &auditsvc.AuditLogSnapshot{RequestID: "r", Method: http.MethodGet, IsHighRisk: 0, ResponseStatus: http.StatusInternalServerError}
	if shouldDropBySampling(errResp) {
		t.Fatal("error response must not be sampled out")
	}
}

// TestShouldDropBySamplingKeepsSlowReadOnlyRequests 验证低风险只读慢请求即便命中采样也始终保留。
func TestShouldDropBySamplingKeepsSlowReadOnlyRequests(t *testing.T) {
	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Queue.AuditSampleGetKeepEvery = 4
		cfg.Queue.AuditSampleSlowKeepThresholdMs = 1000 // 慢请求阈值 1s
	})
	defer restoreConfig()

	atomicResetSampleCounter()
	// 耗时 1.5s（1_500_000 μs）的低风险只读成功请求，超过阈值，应永不被丢弃。
	slow := &auditsvc.AuditLogSnapshot{
		RequestID:       "req-slow",
		Method:          http.MethodGet,
		IsHighRisk:      0,
		ResponseStatus:  http.StatusOK,
		OperationStatus: 0,
		ExecutionTimeUs: 1_500_000,
	}
	for i := 0; i < 12; i++ {
		if shouldDropBySampling(slow) {
			t.Fatalf("slow read-only request must not be sampled out (iteration %d)", i)
		}
	}

	// 同条件但耗时仅 10ms 的快请求仍按 1/4 采样。
	atomicResetSampleCounter()
	fast := &auditsvc.AuditLogSnapshot{
		RequestID:       "req-fast",
		Method:          http.MethodGet,
		IsHighRisk:      0,
		ResponseStatus:  http.StatusOK,
		OperationStatus: 0,
		ExecutionTimeUs: 10_000,
	}
	kept := 0
	for i := 0; i < 12; i++ {
		if !shouldDropBySampling(fast) {
			kept++
		}
	}
	if kept != 3 {
		t.Fatalf("expected 3 kept out of 12 for fast requests with keep_every=4, got %d", kept)
	}
}

// TestFallbackBufferBatchesAndDrains 验证缓冲会聚合后批量落库，并在停止时 drain 完所有条目。
func TestFallbackBufferBatchesAndDrains(t *testing.T) {
	resetAuditFallbackBufferForTesting()
	defer resetAuditFallbackBufferForTesting()

	var (
		mu        sync.Mutex
		persisted []string
	)
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		PersistBatch: func(ctx context.Context, snapshots []*auditsvc.AuditLogSnapshot) error {
			mu.Lock()
			defer mu.Unlock()
			for _, s := range snapshots {
				persisted = append(persisted, s.RequestID)
			}
			return nil
		},
	})
	defer restoreDeps()

	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Queue.Enable = false
		cfg.Queue.AuditFallbackBufferSize = 16
		cfg.Queue.AuditFallbackBatchSize = 8
		cfg.Queue.AuditFallbackFlushIntervalMs = 50
		cfg.Queue.AuditSampleGetKeepEvery = 1 // 关闭采样
	})
	defer restoreConfig()

	storageUnavailable.Store(false)
	for i := 0; i < 10; i++ {
		enqueueAuditLog(nil, "request", &auditsvc.AuditLogSnapshot{
			RequestID: "buf-" + itoa(i),
			Method:    http.MethodPost,
		})
	}

	// 触发 drain 并等待
	StopAuditLogQueueDrain(2 * time.Second)

	mu.Lock()
	got := len(persisted)
	mu.Unlock()
	if got != 10 {
		t.Fatalf("expected 10 persisted snapshots after drain, got %d", got)
	}
}

// TestFallbackBufferFallsBackToSyncWithoutEviction 验证缓冲满时不驱逐已接收条目，新条目走同步落库。
func TestFallbackBufferFallsBackToSyncWithoutEviction(t *testing.T) {
	resetAuditFallbackBufferForTesting()
	defer resetAuditFallbackBufferForTesting()

	var (
		mu             sync.Mutex
		persistedBatch []string
		persistedSync  []string
	)
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		PersistBatch: func(ctx context.Context, snapshots []*auditsvc.AuditLogSnapshot) error {
			mu.Lock()
			defer mu.Unlock()
			for _, s := range snapshots {
				persistedBatch = append(persistedBatch, s.RequestID)
			}
			return nil
		},
		Persist: func(_ context.Context, snapshot *auditsvc.AuditLogSnapshot) error {
			mu.Lock()
			defer mu.Unlock()
			persistedSync = append(persistedSync, snapshot.RequestID)
			return nil
		},
	})
	defer restoreDeps()

	restoreConfig := config.UpdateConfigForTesting(func(cfg *config.Conf) {
		cfg.Queue.Enable = false
		cfg.Queue.AuditFallbackBufferSize = 3
		cfg.Queue.AuditFallbackBatchSize = 100
		cfg.Queue.AuditFallbackFlushIntervalMs = 60_000 // 避免 worker 在 Offer 期间提前清空
		cfg.Queue.AuditSampleGetKeepEvery = 1
	})
	defer restoreConfig()

	storageUnavailable.Store(false)
	// 先塞 2 条低风险 GET 和 1 条写请求，缓冲已满 3 条。
	enqueueAuditLog(nil, "request", &auditsvc.AuditLogSnapshot{RequestID: "lo-1", Method: http.MethodGet, IsHighRisk: 0, ResponseStatus: http.StatusOK})
	enqueueAuditLog(nil, "request", &auditsvc.AuditLogSnapshot{RequestID: "lo-2", Method: http.MethodGet, IsHighRisk: 0, ResponseStatus: http.StatusOK})
	enqueueAuditLog(nil, "request", &auditsvc.AuditLogSnapshot{RequestID: "hi-1", Method: http.MethodPost, IsHighRisk: 1, ResponseStatus: http.StatusOK})

	// 再塞 2 条写请求：缓冲内容不变，hi-2/hi-3 同步落库。
	enqueueAuditLog(nil, "request", &auditsvc.AuditLogSnapshot{RequestID: "hi-2", Method: http.MethodPost, IsHighRisk: 1})
	enqueueAuditLog(nil, "request", &auditsvc.AuditLogSnapshot{RequestID: "hi-3", Method: http.MethodPost, IsHighRisk: 1})

	StopAuditLogQueueDrain(2 * time.Second)

	mu.Lock()
	defer mu.Unlock()
	// 期望缓冲内 3 条全部批量落库，溢出的 2 条全部同步落库。
	gotMap := map[string]bool{}
	for _, id := range append(persistedBatch, persistedSync...) {
		gotMap[id] = true
	}
	for _, must := range []string{"lo-1", "lo-2", "hi-1", "hi-2", "hi-3"} {
		if !gotMap[must] {
			t.Fatalf("expected %s preserved, got batch=%v sync=%v", must, persistedBatch, persistedSync)
		}
	}
	if len(persistedSync) != 2 {
		t.Fatalf("expected 2 overflow items persisted synchronously, got %v", persistedSync)
	}
}

func TestFallbackBufferRestoresBatchWhenStorageUnavailable(t *testing.T) {
	markAuditStorageAvailable()
	defer markAuditStorageAvailable()

	attempts := 0
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		PersistBatch: func(context.Context, []*auditsvc.AuditLogSnapshot) error {
			attempts++
			if attempts == 1 {
				return errors.New("driver: bad connection")
			}
			return nil
		},
	})
	defer restoreDeps()

	buffer := &auditFallbackBuffer{
		items: []*bufferedAuditItem{{
			kind:     "request",
			snapshot: &auditsvc.AuditLogSnapshot{RequestID: "retry-me"},
		}},
		batchSize: 1,
	}
	buffer.drainOnce()
	if pending := buffer.pendingCount(); pending != 1 {
		t.Fatalf("expected unavailable batch to remain buffered, pending=%d", pending)
	}

	markAuditStorageAvailable()
	buffer.drainOnce()
	if pending := buffer.pendingCount(); pending != 0 {
		t.Fatalf("expected restored batch to persist after recovery, pending=%d", pending)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 persistence attempts, got %d", attempts)
	}
}

func TestFallbackBufferIsolatesPermanentInvalidRecord(t *testing.T) {
	markAuditStorageAvailable()
	defer markAuditStorageAvailable()

	var persisted []string
	attempts := 0
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		PersistBatch: func(_ context.Context, snapshots []*auditsvc.AuditLogSnapshot) error {
			attempts++
			for _, snapshot := range snapshots {
				if snapshot.RequestID == "bad" {
					return &mysql.MySQLError{Number: 1406, Message: "Data too long"}
				}
			}
			for _, snapshot := range snapshots {
				persisted = append(persisted, snapshot.RequestID)
			}
			return nil
		},
	})
	defer restoreDeps()

	buffer := &auditFallbackBuffer{
		items: []*bufferedAuditItem{
			{kind: "request", snapshot: &auditsvc.AuditLogSnapshot{RequestID: "good-1"}},
			{kind: "request", snapshot: &auditsvc.AuditLogSnapshot{RequestID: "bad"}},
			{kind: "request", snapshot: &auditsvc.AuditLogSnapshot{RequestID: "good-2"}},
		},
		batchSize: 3,
	}
	buffer.drainOnce()

	if pending := buffer.pendingCount(); pending != 0 {
		t.Fatalf("permanent invalid record must not poison the queue, pending=%d", pending)
	}
	if got := strings.Join(persisted, ","); got != "good-1,good-2" {
		t.Fatalf("valid records should persist around the isolated record, got %q", got)
	}
	if attempts > 7 {
		t.Fatalf("isolation should remain bounded, attempts=%d", attempts)
	}
}

func TestFallbackBufferStopReportsInflightTimeout(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		PersistBatch: func(context.Context, []*auditsvc.AuditLogSnapshot) error {
			close(started)
			<-release
			return nil
		},
	})
	defer restoreDeps()

	buffer := newAuditFallbackBuffer()
	if !buffer.Offer("request", &auditsvc.AuditLogSnapshot{RequestID: "inflight"}) {
		t.Fatal("expected item to enter fallback buffer")
	}
	<-started
	if drained, _ := buffer.Stop(10 * time.Millisecond); drained {
		t.Fatal("expected stop to report an in-flight persistence timeout")
	}

	close(release)
	if drained, pending := buffer.Stop(time.Second); !drained || pending != 0 {
		t.Fatalf("expected worker to finish after persistence release, drained=%v pending=%d", drained, pending)
	}
}

func TestFallbackBufferShutdownProbesStorageDespiteCooldown(t *testing.T) {
	markAuditStorageAvailable()
	defer markAuditStorageAvailable()

	persisted := 0
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		PersistBatch: func(context.Context, []*auditsvc.AuditLogSnapshot) error {
			persisted++
			return nil
		},
	})
	defer restoreDeps()
	markAuditStorageUnavailable()

	buffer := &auditFallbackBuffer{
		items: []*bufferedAuditItem{{
			kind:     "request",
			snapshot: &auditsvc.AuditLogSnapshot{RequestID: "shutdown-probe"},
		}},
		batchSize: 1,
	}
	buffer.drainAll()
	if persisted != 1 {
		t.Fatalf("expected shutdown drain to probe storage once, got %d", persisted)
	}
	if pending := buffer.pendingCount(); pending != 0 {
		t.Fatalf("expected recovered storage to drain pending batch, pending=%d", pending)
	}
}

func TestFallbackOverflowDuringCooldownDoesNotBlockOnSyncPersist(t *testing.T) {
	previousBuffer := fallbackBuffer
	fallbackBuffer = &auditFallbackBuffer{
		started:  true,
		capacity: 1,
		items: []*bufferedAuditItem{{
			kind:     "request",
			snapshot: &auditsvc.AuditLogSnapshot{RequestID: "already-buffered"},
		}},
		notify: make(chan struct{}, 1),
	}
	defer func() { fallbackBuffer = previousBuffer }()

	persistCalls := 0
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		Persist: func(context.Context, *auditsvc.AuditLogSnapshot) error {
			persistCalls++
			return nil
		},
	})
	defer restoreDeps()
	markAuditStorageUnavailable()

	enqueueIntoFallbackBuffer("request", &auditsvc.AuditLogSnapshot{RequestID: "overflow"})

	if persistCalls != 0 {
		t.Fatalf("expected cooldown overflow not to synchronously hit storage, got %d calls", persistCalls)
	}
	if dropped := auditOverflowDropped.Load(); dropped != 1 {
		t.Fatalf("expected overflow loss to be counted, got %d", dropped)
	}
}

func TestSyncFallbackProvidesPersistenceDeadline(t *testing.T) {
	deadlineSeen := false
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		Persist: func(ctx context.Context, _ *auditsvc.AuditLogSnapshot) error {
			_, deadlineSeen = ctx.Deadline()
			return nil
		},
	})
	defer restoreDeps()

	reportAuditPersistenceResult("request", &auditsvc.AuditLogSnapshot{RequestID: "deadline"}, "test")
	if !deadlineSeen {
		t.Fatal("expected synchronous fallback persistence to have a fixed deadline")
	}
}

func TestRecordRequestStatUsesDeadlineAndCooldown(t *testing.T) {
	calls := 0
	deadlineSeen := false
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		RecordStat: func(ctx context.Context, _ *auditsvc.AuditLogSnapshot) error {
			calls++
			_, deadlineSeen = ctx.Deadline()
			return errors.New("database unavailable")
		},
	})
	defer restoreDeps()

	snapshot := &auditsvc.AuditLogSnapshot{RequestID: "stats-db-down"}
	recordRequestStat("request", snapshot)
	recordRequestStat("request", snapshot)

	if !deadlineSeen {
		t.Fatal("expected daily request stats persistence to have a fixed deadline")
	}
	if calls != 1 {
		t.Fatalf("expected cooldown to suppress repeated stats writes, calls=%d", calls)
	}
	if storageUnavailable.Load() {
		t.Fatal("request stats failure must not trip the request-log storage circuit")
	}
}

func TestRequestStatsFailureDoesNotBlockAuditBufferDrain(t *testing.T) {
	persisted := 0
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		RecordStat: func(context.Context, *auditsvc.AuditLogSnapshot) error {
			return errors.New("daily stats table unavailable")
		},
		PersistBatch: func(context.Context, []*auditsvc.AuditLogSnapshot) error {
			persisted++
			return nil
		},
	})
	defer restoreDeps()

	recordRequestStat("request", &auditsvc.AuditLogSnapshot{RequestID: "stats-failed"})
	buffer := &auditFallbackBuffer{
		items: []*bufferedAuditItem{{
			kind:     "request",
			snapshot: &auditsvc.AuditLogSnapshot{RequestID: "audit-still-healthy"},
		}},
		batchSize: 1,
	}
	buffer.drainOnce()

	if persisted != 1 || buffer.pendingCount() != 0 {
		t.Fatalf("stats failure must not block request-log drain, persisted=%d pending=%d", persisted, buffer.pendingCount())
	}
}

func TestFallbackOverflowAttemptsEverySyncPersistWhileStorageHealthy(t *testing.T) {
	previousBuffer := fallbackBuffer
	fallbackBuffer = &auditFallbackBuffer{
		started:  true,
		capacity: 1,
		items: []*bufferedAuditItem{{
			kind:     "request",
			snapshot: &auditsvc.AuditLogSnapshot{RequestID: "already-buffered"},
		}},
		notify: make(chan struct{}, 1),
	}
	defer func() { fallbackBuffer = previousBuffer }()

	started := make(chan struct{}, 2)
	release := make(chan struct{})
	var calls atomic.Int32
	restoreDeps := setAuditQueueDepsForTesting(auditQueueDeps{
		Persist: func(context.Context, *auditsvc.AuditLogSnapshot) error {
			calls.Add(1)
			started <- struct{}{}
			<-release
			return nil
		},
	})
	defer restoreDeps()

	var wg sync.WaitGroup
	for _, requestID := range []string{"sync-1", "sync-2"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			enqueueIntoFallbackBuffer("request", &auditsvc.AuditLogSnapshot{RequestID: requestID})
		}()
	}
	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("expected every overflow record to attempt synchronous persistence")
		}
	}
	close(release)
	wg.Wait()
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected every overflow record to attempt synchronous persistence, got %d", got)
	}
	if dropped := auditOverflowDropped.Load(); dropped != 0 {
		t.Fatalf("healthy storage must not drop concurrent overflow records, got %d", dropped)
	}
}

func TestAuditStorageHalfOpenAllowsSingleProbe(t *testing.T) {
	markAuditStorageUnavailable()
	defer markAuditStorageAvailable()
	storageUnavailableSince.Store(time.Now().Add(-auditStorageRetryCooldown - time.Second).UnixNano())
	storageRecoveryProbe.Store(false)

	var allowed atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if tryAcquireAuditStorageAttempt() {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := allowed.Load(); got != 1 {
		t.Fatalf("expected exactly one half-open storage probe, got %d", got)
	}
}

func atomicResetSampleCounter() {
	// 通过 setAuditQueueDepsForTesting 的副作用复位会破坏测试上下文，
	// 这里直接复位计数器更稳妥。
	resetSampleCounterForTesting()
}
