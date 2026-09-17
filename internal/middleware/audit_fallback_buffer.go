package middleware

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/config"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	audit "github.com/wannanbigpig/gin-layout/internal/service/audit"
)

const (
	defaultAuditFallbackBufferSize      = 1024
	defaultAuditFallbackBatchSize       = 200
	defaultAuditFallbackFlushIntervalMs = 1000
	auditBatchPersistTimeout            = 10 * time.Second
)

var fallbackBuffer = newAuditFallbackBuffer()

// ---------------------------- 降级缓冲实现 ----------------------------

type bufferedAuditItem struct {
	kind     string
	snapshot *audit.AuditLogSnapshot
}

// auditFallbackBuffer 是有界内存环形缓冲。
//
// 设计要点：
//   - 容量来自配置（默认 1024），首次写入时按当时配置初始化 worker。
//   - 满时不驱逐已接收条目，返回 false 由调用方走同步 INSERT 兜底。
//   - 单个后台 worker 拉取最多 batchSize 条或等待最长 flushInterval 时间后批量落库。
//   - 退出阶段调用 Stop() 阻塞至缓冲清空或达到 drain 超时。
type auditFallbackBuffer struct {
	mu       sync.Mutex
	items    []*bufferedAuditItem
	notify   chan struct{}
	stopCh   chan struct{}
	doneCh   chan struct{}
	stopOnce sync.Once
	started  bool

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
		b.mu.Unlock()
		return false
	}
	b.items = append(b.items, &bufferedAuditItem{kind: kind, snapshot: snapshot})
	b.mu.Unlock()

	b.notifyWorker()
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
	if !tryAcquireAuditStorageAttempt() {
		return
	}
	batch := b.takeBatch(b.batchSize)
	if len(batch) == 0 {
		storageRecoveryProbe.Store(false)
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
		// 停机时忽略旧的冷却状态并至少探测一次：存储若已恢复则继续排空；
		// 仍不可用时 persistBatch 会放回当前批次，此处停止避免紧循环。
		if auditStorageInCooldown() {
			log.Warn("Audit fallback buffer drain stopped while storage is unavailable",
				zap.Int("pending", b.pendingCount()))
			return
		}
	}
}

func (b *auditFallbackBuffer) pendingCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.items)
}

// restoreBatch 把存储不可用时取出的整批放回队首，等待冷却期后重试。
// 批次在锁外落库期间可能有新条目入队，因此故障瞬间最多短暂超出一个 batchSize；
// 后续 Offer 会回退到同步落库，不会无界增长。
func (b *auditFallbackBuffer) restoreBatch(batch []*bufferedAuditItem) {
	if len(batch) == 0 {
		return
	}
	b.mu.Lock()
	restored := make([]*bufferedAuditItem, 0, len(batch)+len(b.items))
	restored = append(restored, batch...)
	restored = append(restored, b.items...)
	b.items = restored
	b.mu.Unlock()
	audit.SetFallbackPending(b.pendingCount())
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
	remaining := copy(b.items, b.items[n:])
	clear(b.items[remaining:])
	b.items = b.items[:remaining]
	audit.SetFallbackPending(len(b.items))
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

	if err := currentDeps().PersistBatch(ctx, snapshots); err != nil {
		audit.RecordPersistFailure()
		if isPermanentAuditPersistenceError(err) {
			log.Error("Audit log batch contains permanently invalid data; isolating records",
				zap.Int("batch_size", len(snapshots)),
				zap.Error(err))
			if b.persistIsolated(ctx, batch) {
				markAuditStorageAvailable()
			}
			return
		}
		if markAuditStorageUnavailable() {
			log.Warn("Audit log batch persistence failed, defer the atomic batch",
				zap.Int("batch_size", len(snapshots)),
				zap.Error(err))
		}
		// PersistAuditLogsBatch 使用显式事务，批次要么全部提交、要么全部回滚。
		// 任何错误都保留原批次并延迟重试，避免连接断开时逐条写放大故障。
		b.restoreBatch(batch)
		return
	}

	markAuditStorageAvailable()
	audit.RecordPersisted(len(snapshots))
}

// persistIsolated 仅在数据库明确返回永久行错误时二分批次，保留可写记录并隔离坏记录。
// 未知错误按临时存储故障处理，避免数据库故障期间发生逐条写放大。
func (b *auditFallbackBuffer) persistIsolated(ctx context.Context, batch []*bufferedAuditItem) bool {
	if len(batch) == 0 {
		return true
	}
	snapshots := snapshotsFromBufferedAuditItems(batch)
	if len(snapshots) == 0 {
		return true
	}
	if err := currentDeps().PersistBatch(ctx, snapshots); err != nil {
		audit.RecordPersistFailure()
		if !isPermanentAuditPersistenceError(err) {
			if markAuditStorageUnavailable() {
				log.Warn("Audit log storage failed while isolating invalid records",
					zap.Int("batch_size", len(snapshots)),
					zap.Error(err))
			}
			b.restoreBatch(batch)
			return false
		}
		if len(batch) == 1 {
			audit.RecordPermanentDrop()
			log.Error("Drop permanently invalid audit log record",
				zap.String("request_id", snapshots[0].RequestID),
				zap.Error(err))
			return true
		}
		middle := len(batch) / 2
		leftHealthy := b.persistIsolated(ctx, batch[:middle])
		rightHealthy := b.persistIsolated(ctx, batch[middle:])
		return leftHealthy && rightHealthy
	}
	audit.RecordPersisted(len(snapshots))
	return true
}

func snapshotsFromBufferedAuditItems(batch []*bufferedAuditItem) []*audit.AuditLogSnapshot {
	snapshots := make([]*audit.AuditLogSnapshot, 0, len(batch))
	for _, item := range batch {
		if item == nil || item.snapshot == nil {
			continue
		}
		snapshots = append(snapshots, item.snapshot)
	}
	return snapshots
}

func isPermanentAuditPersistenceError(err error) bool {
	var mysqlErr *mysql.MySQLError
	if !errors.As(err, &mysqlErr) {
		return false
	}
	switch mysqlErr.Number {
	case 1048, // column cannot be null
		1062, // duplicate entry
		1264, // out of range
		1292, // incorrect value
		1366, // incorrect string value
		1406, // data too long
		1452, // foreign key constraint
		3819: // check constraint
		return true
	default:
		return false
	}
}

// Stop 触发缓冲停止并等待 worker 完成 drain。
// 返回 drained=false 时，调用方不得继续关闭 DB，否则会与仍在落库的 worker 竞态。
func (b *auditFallbackBuffer) Stop(drainTimeout time.Duration) (drained bool, pending int) {
	b.mu.Lock()
	if !b.started {
		b.mu.Unlock()
		return true, 0
	}
	stopOnce := &b.stopOnce
	stopCh := b.stopCh
	doneCh := b.doneCh
	b.mu.Unlock()

	stopOnce.Do(func() { close(stopCh) })

	if drainTimeout <= 0 {
		return false, b.pendingCount()
	}
	select {
	case <-doneCh:
		pending = b.pendingCount()
		return pending == 0, pending
	case <-time.After(drainTimeout):
		return false, b.pendingCount()
	}
}

// StopAuditLogQueueDrain 在进程退出前调用以等待降级缓冲清空。
func StopAuditLogQueueDrain(drainTimeout time.Duration) (drained bool, pending int) {
	return fallbackBuffer.Stop(drainTimeout)
}

// resetAuditFallbackBufferForTesting 在单元测试间重置缓冲单例，避免协程泄漏与状态串扰。
func resetAuditFallbackBufferForTesting() {
	_, _ = fallbackBuffer.Stop(auditBatchPersistTimeout + time.Second)
	fallbackBuffer = newAuditFallbackBuffer()
}
