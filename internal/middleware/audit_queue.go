package middleware

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/jobs"
	perrors "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/queue"
	audit "github.com/wannanbigpig/gin-layout/internal/service/audit"
)

const (
	auditEnqueueTimeout              = 2 * time.Second
	defaultAuditFallbackBufferSize   = 1024
	defaultAuditFallbackBatchSize    = 200
	defaultAuditFallbackFlushIntervalMs = 1000
	defaultAuditSampleGetKeepEvery   = 10
	auditBatchPersistTimeout         = 10 * time.Second
)

var (
	enqueueAuditTaskFn      = jobs.EnqueueAuditLog
	persistAuditLogFn       = audit.PersistAuditLog
	persistAuditLogBatchFn  = audit.PersistAuditLogsBatch

	queueUnavailableLogged atomic.Bool
	storageUnavailable     atomic.Bool

	// auditSampleCounter 用于低风险只读请求的 1/N 采样。
	auditSampleCounter uint64

	fallbackBuffer = newAuditFallbackBuffer()
)

type auditQueueDeps struct {
	Enqueue      func(ctx context.Context, kind string, snapshot *audit.AuditLogSnapshot) error
	Persist      func(snapshot *audit.AuditLogSnapshot) error
	PersistBatch func(ctx context.Context, snapshots []*audit.AuditLogSnapshot) error
}

func setAuditQueueDepsForTesting(deps auditQueueDeps) func() {
	previousEnqueue := enqueueAuditTaskFn
	previousPersist := persistAuditLogFn
	previousPersistBatch := persistAuditLogBatchFn
	previousQueueUnavailable := queueUnavailableLogged.Load()
	previousStorageUnavailable := storageUnavailable.Load()

	enqueueAuditTaskFn = deps.Enqueue
	if enqueueAuditTaskFn == nil {
		enqueueAuditTaskFn = jobs.EnqueueAuditLog
	}
	persistAuditLogFn = deps.Persist
	if persistAuditLogFn == nil {
		persistAuditLogFn = audit.PersistAuditLog
	}
	persistAuditLogBatchFn = deps.PersistBatch
	if persistAuditLogBatchFn == nil {
		persistAuditLogBatchFn = audit.PersistAuditLogsBatch
	}

	queueUnavailableLogged.Store(false)
	storageUnavailable.Store(false)
	atomic.StoreUint64(&auditSampleCounter, 0)

	return func() {
		enqueueAuditTaskFn = previousEnqueue
		persistAuditLogFn = previousPersist
		persistAuditLogBatchFn = previousPersistBatch
		queueUnavailableLogged.Store(previousQueueUnavailable)
		storageUnavailable.Store(previousStorageUnavailable)
	}
}

// enqueueAuditLog 是审计日志的统一投递入口。
//
// 流程：
// 1. 对低风险只读请求按比例采样后丢弃；
// 2. 已知存储不可用时直接短路，避免反复打 DB；
// 3. 队列启用且可用时走 asynq 投递；
// 4. 队列禁用 / 投递失败时进入内存缓冲 + 批量异步落库，最终失败兜底同步 INSERT。
func enqueueAuditLog(_ *gin.Context, kind string, snapshot *audit.AuditLogSnapshot) {
	if snapshot == nil {
		return
	}

	if shouldDropBySampling(snapshot) {
		return
	}

	if storageUnavailable.Load() {
		// 已知存储不可用：跳过缓冲与同步落库，避免持续放大 DB 压力。
		// 后续一次成功 persist 后会清除该标记。
		return
	}

	cfg := config.GetConfig()
	if cfg == nil || !cfg.Queue.Enable {
		enqueueIntoFallbackBuffer(kind, snapshot)
		return
	}

	// 审计入队不能跟随请求 ctx，否则客户端断开 / handler 返回后 ctx 立刻被取消，
	// 入队会失败并降级到同步落库，背离“异步审计”的目的。
	ctx, cancel := context.WithTimeout(context.Background(), auditEnqueueTimeout)
	defer cancel()

	if err := enqueueAuditTaskFn(ctx, kind, snapshot); err != nil {
		if errors.Is(err, queue.ErrPublisherUnavailable) {
			if queueUnavailableLogged.CompareAndSwap(false, true) {
				log.Warn("Audit queue publisher unavailable, fallback to buffered persist",
					zap.String("operation", "enqueue_audit_log"),
					zap.String("kind", kind),
					zap.String("request_id", snapshot.RequestID))
			}
		} else {
			log.Warn("Enqueue audit log failed, fallback to buffered persist",
				zap.String("operation", "enqueue_audit_log"),
				zap.String("kind", kind),
				zap.String("request_id", snapshot.RequestID),
				zap.Error(err))
		}
		enqueueIntoFallbackBuffer(kind, snapshot)
		return
	}

	// 队列恢复后复位告警开关，保证下次不可用时仍能打首条告警。
	queueUnavailableLogged.Store(false)
}

// shouldDropBySampling 判定是否需要按采样比例丢弃当前快照。
//
// 命中条件（全部满足才进入采样）：
//   - IsHighRisk == 0（低风险）
//   - 只读方法（GET/HEAD/OPTIONS）
//   - ResponseStatus < 400（HTTP 层成功）
//   - OperationStatus == 0（业务层成功）
//
// 新契约分工：
//   - ResponseStatus：HTTP 状态码，4xx/5xx 表示协议层失败。
//   - OperationStatus：业务状态码，!= 0 表示业务失败（与 HTTP 状态码无关）。
//
// 采样语义：保留 1/N，N<=1 表示不采样。
func shouldDropBySampling(snapshot *audit.AuditLogSnapshot) bool {
	if snapshot == nil {
		return false
	}
	if snapshot.IsHighRisk != 0 {
		return false
	}
	if !isReadOnlyMethod(snapshot.Method) {
		return false
	}
	if snapshot.ResponseStatus >= http.StatusBadRequest {
		return false
	}
	if snapshot.OperationStatus != 0 {
		return false
	}

	keepEvery := resolveAuditSampleKeepEvery()
	if keepEvery <= 1 {
		return false
	}
	// 用原子计数保证多协程下分布均匀；保留 N、2N、3N ... 计数对应的快照。
	n := atomic.AddUint64(&auditSampleCounter, 1)
	return n%uint64(keepEvery) != 0
}

func isReadOnlyMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func resolveAuditSampleKeepEvery() int {
	cfg := config.GetConfig()
	if cfg == nil {
		return defaultAuditSampleGetKeepEvery
	}
	if cfg.Queue.AuditSampleGetKeepEvery <= 0 {
		return defaultAuditSampleGetKeepEvery
	}
	return cfg.Queue.AuditSampleGetKeepEvery
}

// enqueueIntoFallbackBuffer 将快照投递到降级缓冲，缓冲满时回退到同步 INSERT 兜底。
func enqueueIntoFallbackBuffer(kind string, snapshot *audit.AuditLogSnapshot) {
	if !fallbackBuffer.Offer(kind, snapshot) {
		reportAuditPersistenceResult(kind, snapshot, "sync_fallback_buffer_full")
	}
}

func reportAuditPersistenceResult(kind string, snapshot *audit.AuditLogSnapshot, mode string) {
	if snapshot == nil {
		return
	}

	if err := persistAuditLogFn(snapshot); err != nil {
		if perrors.IsDependencyNotReady(err) {
			if storageUnavailable.CompareAndSwap(false, true) {
				log.Warn("Audit log storage unavailable, skip persistence",
					zap.String("kind", kind),
					zap.String("mode", mode),
					zap.String("request_id", snapshot.RequestID),
					zap.Error(err))
			}
			return
		}
		log.Error("Persist audit log failed",
			zap.String("kind", kind),
			zap.String("mode", mode),
			zap.String("request_id", snapshot.RequestID),
			zap.Error(err))
		return
	}

	storageUnavailable.Store(false)
}

// ---------------------------- 降级缓冲实现 ----------------------------

type bufferedAuditItem struct {
	kind     string
	snapshot *audit.AuditLogSnapshot
}

// auditFallbackBuffer 是有界内存环形缓冲。
//
// 设计要点：
// - 容量来自配置（默认 1024），首次写入时按当时配置初始化 worker。
// - 满时优先丢弃“低风险只读 2xx”的最旧条目，腾出空间接纳新条目；
//   找不到可丢弃条目时返回 false，调用方走同步 INSERT 兜底。
// - 单个后台 worker 拉取最多 batchSize 条或等待最长 flushInterval 时间后批量落库。
// - 退出阶段调用 Stop() 阻塞至缓冲清空或达到 drain 超时。
type auditFallbackBuffer struct {
	mu      sync.Mutex
	items   []*bufferedAuditItem
	notify  chan struct{}
	stopCh  chan struct{}
	doneCh  chan struct{}
	started bool

	// 启动后固化的配置参数（避免运行中动态变更带来的复杂性）。
	capacity      int
	batchSize     int
	flushInterval time.Duration
}

func newAuditFallbackBuffer() *auditFallbackBuffer {
	return &auditFallbackBuffer{}
}

// Offer 尝试将一条审计快照入队。返回 false 表示无法入队，应由调用方同步兜底。
func (b *auditFallbackBuffer) Offer(kind string, snapshot *audit.AuditLogSnapshot) bool {
	if snapshot == nil {
		return true
	}
	b.mu.Lock()
	b.ensureStartedLocked()

	if len(b.items) >= b.capacity {
		if !b.dropLowRiskOldestLocked() {
			b.mu.Unlock()
			return false
		}
	}
	b.items = append(b.items, &bufferedAuditItem{kind: kind, snapshot: snapshot})
	b.mu.Unlock()

	b.notifyWorker()
	return true
}

// dropLowRiskOldestLocked 从队首扫描丢弃首个低风险只读 2xx 条目，否则返回 false。
func (b *auditFallbackBuffer) dropLowRiskOldestLocked() bool {
	for idx, item := range b.items {
		if item == nil || item.snapshot == nil {
			continue
		}
		if isLowRiskReadOnly2xx(item.snapshot) {
			snap := item.snapshot
			log.Warn("Audit fallback buffer full, dropping low-risk oldest entry",
				zap.String("kind", item.kind),
				zap.String("request_id", snap.RequestID),
				zap.String("path", snap.BaseURL),
				zap.String("method", snap.Method),
				zap.Int("response_status", snap.ResponseStatus),
				zap.Int("operation_status", snap.OperationStatus))
			b.items = append(b.items[:idx], b.items[idx+1:]...)
			return true
		}
	}
	return false
}

// isLowRiskReadOnly2xx 判断快照是否为"低风险 + 只读 + HTTP/业务均成功"，用于缓冲满时的丢弃优先级。
//
// 新契约下 OperationStatus != 0 表示业务失败，不应被当作低风险可丢弃条目。
func isLowRiskReadOnly2xx(snapshot *audit.AuditLogSnapshot) bool {
	if snapshot == nil {
		return false
	}
	if snapshot.IsHighRisk != 0 {
		return false
	}
	if !isReadOnlyMethod(snapshot.Method) {
		return false
	}
	if snapshot.ResponseStatus != 0 && snapshot.ResponseStatus >= http.StatusBadRequest {
		return false
	}
	if snapshot.OperationStatus != 0 {
		return false
	}
	return true
}

func (b *auditFallbackBuffer) ensureStartedLocked() {
	if b.started {
		return
	}
	cfg := config.GetConfig()
	capacity := defaultAuditFallbackBufferSize
	batchSize := defaultAuditFallbackBatchSize
	flushIntervalMs := defaultAuditFallbackFlushIntervalMs
	if cfg != nil {
		if cfg.Queue.AuditFallbackBufferSize > 0 {
			capacity = cfg.Queue.AuditFallbackBufferSize
		}
		if cfg.Queue.AuditFallbackBatchSize > 0 {
			batchSize = cfg.Queue.AuditFallbackBatchSize
		}
		if cfg.Queue.AuditFallbackFlushIntervalMs > 0 {
			flushIntervalMs = cfg.Queue.AuditFallbackFlushIntervalMs
		}
	}
	b.capacity = capacity
	b.batchSize = batchSize
	b.flushInterval = time.Duration(flushIntervalMs) * time.Millisecond
	b.items = make([]*bufferedAuditItem, 0, capacity)
	b.notify = make(chan struct{}, 1)
	b.stopCh = make(chan struct{})
	b.doneCh = make(chan struct{})
	b.started = true
	go b.runWorker()
}

func (b *auditFallbackBuffer) notifyWorker() {
	select {
	case b.notify <- struct{}{}:
	default:
	}
}

func (b *auditFallbackBuffer) runWorker() {
	defer close(b.doneCh)
	for {
		select {
		case <-b.stopCh:
			b.drainAll()
			return
		case <-b.notify:
			b.drainOnce()
		case <-time.After(b.flushInterval):
			b.drainOnce()
		}
	}
}

// drainOnce 单次拉取一批条目并尝试批量落库。
func (b *auditFallbackBuffer) drainOnce() {
	batch := b.takeBatch(b.batchSize)
	if len(batch) == 0 {
		return
	}
	b.persistBatch(batch)
}

// drainAll 在停机阶段尽力清空缓冲。
func (b *auditFallbackBuffer) drainAll() {
	for {
		batch := b.takeBatch(b.batchSize)
		if len(batch) == 0 {
			return
		}
		b.persistBatch(batch)
	}
}

func (b *auditFallbackBuffer) takeBatch(n int) []*bufferedAuditItem {
	b.mu.Lock()
	defer b.mu.Unlock()
	if n <= 0 || len(b.items) == 0 {
		return nil
	}
	if n > len(b.items) {
		n = len(b.items)
	}
	taken := make([]*bufferedAuditItem, n)
	copy(taken, b.items[:n])
	b.items = append(b.items[:0], b.items[n:]...)
	return taken
}

func (b *auditFallbackBuffer) persistBatch(batch []*bufferedAuditItem) {
	snapshots := make([]*audit.AuditLogSnapshot, 0, len(batch))
	for _, item := range batch {
		if item == nil || item.snapshot == nil {
			continue
		}
		snapshots = append(snapshots, item.snapshot)
	}
	if len(snapshots) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), auditBatchPersistTimeout)
	defer cancel()

	if err := persistAuditLogBatchFn(ctx, snapshots); err != nil {
		if perrors.IsDependencyNotReady(err) {
			if storageUnavailable.CompareAndSwap(false, true) {
				log.Warn("Audit log storage unavailable, skip batch persistence",
					zap.Int("batch_size", len(snapshots)),
					zap.Error(err))
			}
			// 存储不可用：丢弃这批，避免阻塞 worker；后续条目会被 enqueueAuditLog 短路。
			return
		}
		log.Error("Batch persist audit log failed, fallback to per-item sync persist",
			zap.Int("batch_size", len(snapshots)),
			zap.Error(err))
		// 批量失败但不是依赖未就绪：逐条同步兜底，最大化保留可写入的数据。
		for _, item := range batch {
			if item == nil || item.snapshot == nil {
				continue
			}
			reportAuditPersistenceResult(item.kind, item.snapshot, "sync_fallback_batch_error")
		}
		return
	}

	storageUnavailable.Store(false)
}

// Stop 触发缓冲停止并等待 worker 完成 drain。drainTimeout<=0 时立即返回不等待。
func (b *auditFallbackBuffer) Stop(drainTimeout time.Duration) {
	b.mu.Lock()
	if !b.started {
		b.mu.Unlock()
		return
	}
	stopCh := b.stopCh
	doneCh := b.doneCh
	b.mu.Unlock()

	select {
	case <-stopCh:
		// already stopped
	default:
		close(stopCh)
	}

	if drainTimeout <= 0 {
		return
	}
	select {
	case <-doneCh:
	case <-time.After(drainTimeout):
	}
}

// StopAuditLogQueueDrain 在进程退出前调用以等待降级缓冲清空，建议传入 ≤5s 的超时。
func StopAuditLogQueueDrain(drainTimeout time.Duration) {
	fallbackBuffer.Stop(drainTimeout)
}

// resetAuditFallbackBufferForTesting 在单元测试间重置缓冲单例，避免协程泄漏与状态串扰。
func resetAuditFallbackBufferForTesting() {
	fallbackBuffer.Stop(time.Second)
	fallbackBuffer = newAuditFallbackBuffer()
}

// resetSampleCounterForTesting 在单元测试间重置采样计数器，避免测试间的计数串扰。
func resetSampleCounterForTesting() {
	atomic.StoreUint64(&auditSampleCounter, 0)
}
