package middleware

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/jobs"
	"github.com/wannanbigpig/gin-layout/internal/model"
	perrors "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"github.com/wannanbigpig/gin-layout/internal/queue"
	audit "github.com/wannanbigpig/gin-layout/internal/service/audit"
)

const (
	auditEnqueueTimeout                   = 2 * time.Second
	auditSyncPersistTimeout               = 2 * time.Second
	auditStatsPersistTimeout              = 250 * time.Millisecond
	defaultAuditSampleGetKeepEvery        = 10
	defaultAuditSampleSlowKeepThresholdMs = 1000
	// auditStorageRetryCooldown 是存储不可用后的熔断冷却期：冷却期内短路，过后放行一次探测，
	// 避免 DB 短暂故障被永久判定为不可用（探测成功即复位，失败则重新计时）。
	auditStorageRetryCooldown = 30 * time.Second
)

var (
	queueUnavailableLogged atomic.Bool
	auditOverflowLogged    atomic.Bool
	auditOverflowDropped   atomic.Uint64
	storageUnavailable     atomic.Bool
	storageRecoveryProbe   atomic.Bool
	// storageUnavailableSince 记录最近一次判定存储不可用的时刻（UnixNano），用于冷却期半开探测。
	storageUnavailableSince atomic.Int64
	statsUnavailable        atomic.Bool
	statsRecoveryProbe      atomic.Bool
	statsUnavailableSince   atomic.Int64

	// auditSampleCounter 用于低风险只读请求的 1/N 采样。
	auditSampleCounter atomic.Uint64
)

type auditQueueDeps struct {
	Enqueue      func(ctx context.Context, kind string, snapshot *audit.AuditLogSnapshot) error
	Persist      func(ctx context.Context, snapshot *audit.AuditLogSnapshot) error
	PersistBatch func(ctx context.Context, snapshots []*audit.AuditLogSnapshot) error
	RecordStat   func(ctx context.Context, snapshot *audit.AuditLogSnapshot) error
}

// activeDeps 原子持有审计落库依赖。后台 worker goroutine 会并发读取，测试会在还原时写入，
// 用原子指针替换裸函数变量，消除「worker 读 / 测试写」之间的数据竞争。
var activeDeps atomic.Pointer[auditQueueDeps]

func init() {
	activeDeps.Store(&auditQueueDeps{
		Enqueue:      jobs.EnqueueAuditLog,
		Persist:      audit.PersistAuditLogContext,
		PersistBatch: audit.PersistAuditLogsBatch,
		RecordStat:   audit.RecordDailyRequestStatContext,
	})
}

// currentDeps 原子读取当前生效的依赖集合。
func currentDeps() auditQueueDeps {
	return *activeDeps.Load()
}

func setAuditQueueDepsForTesting(deps auditQueueDeps) func() {
	previous := *activeDeps.Load()
	previousQueueUnavailable := queueUnavailableLogged.Load()
	previousStorageUnavailable := storageUnavailable.Load()
	previousStorageUnavailableSince := storageUnavailableSince.Load()
	previousStorageRecoveryProbe := storageRecoveryProbe.Load()
	previousStatsUnavailable := statsUnavailable.Load()
	previousStatsUnavailableSince := statsUnavailableSince.Load()
	previousStatsRecoveryProbe := statsRecoveryProbe.Load()
	previousOverflowLogged := auditOverflowLogged.Load()
	previousOverflowDropped := auditOverflowDropped.Load()

	next := auditQueueDeps{
		Enqueue:      deps.Enqueue,
		Persist:      deps.Persist,
		PersistBatch: deps.PersistBatch,
		RecordStat:   deps.RecordStat,
	}
	if next.Enqueue == nil {
		next.Enqueue = jobs.EnqueueAuditLog
	}
	if next.Persist == nil {
		next.Persist = audit.PersistAuditLogContext
	}
	if next.PersistBatch == nil {
		next.PersistBatch = audit.PersistAuditLogsBatch
	}
	if next.RecordStat == nil {
		next.RecordStat = audit.RecordDailyRequestStatContext
	}
	activeDeps.Store(&next)

	queueUnavailableLogged.Store(false)
	storageUnavailable.Store(false)
	storageUnavailableSince.Store(0)
	storageRecoveryProbe.Store(false)
	statsUnavailable.Store(false)
	statsUnavailableSince.Store(0)
	statsRecoveryProbe.Store(false)
	auditOverflowLogged.Store(false)
	auditOverflowDropped.Store(0)
	auditSampleCounter.Store(0)

	return func() {
		activeDeps.Store(&previous)
		queueUnavailableLogged.Store(previousQueueUnavailable)
		storageUnavailable.Store(previousStorageUnavailable)
		storageUnavailableSince.Store(previousStorageUnavailableSince)
		storageRecoveryProbe.Store(previousStorageRecoveryProbe)
		statsUnavailable.Store(previousStatsUnavailable)
		statsUnavailableSince.Store(previousStatsUnavailableSince)
		statsRecoveryProbe.Store(previousStatsRecoveryProbe)
		auditOverflowLogged.Store(previousOverflowLogged)
		auditOverflowDropped.Store(previousOverflowDropped)
	}
}

// enqueueAuditLog 是审计日志的统一投递入口。
//
// 流程：
// 1. 已认证业务请求不参与采样；只有无法关联用户的低风险只读流量可按配置采样；
// 2. 队列启用且可用时走 asynq 投递；
// 3. 队列禁用 / 投递失败时进入内存缓冲 + 批量异步落库，缓冲满时同步 INSERT 兜底。
func enqueueAuditLog(_ *gin.Context, kind string, snapshot *audit.AuditLogSnapshot) {
	if snapshot == nil {
		return
	}

	if shouldDropBySampling(snapshot) {
		audit.RecordSampledOut()
		return
	}
	audit.RecordAccepted()

	cfg := config.GetConfig()
	if cfg == nil || !cfg.Queue.Enable {
		enqueueIntoFallbackBuffer(kind, snapshot)
		return
	}

	// 审计入队不能跟随请求 ctx，否则客户端断开 / handler 返回后 ctx 立刻被取消，
	// 入队会失败并降级到同步落库，背离“异步审计”的目的。
	ctx, cancel := context.WithTimeout(context.Background(), auditEnqueueTimeout)
	defer cancel()

	if err := currentDeps().Enqueue(ctx, kind, snapshot); err != nil {
		audit.RecordQueueFallback()
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
	audit.RecordQueueEnqueued()
}

func recordRequestStat(kind string, snapshot *audit.AuditLogSnapshot) {
	if kind != jobs.AuditLogKindRequest && kind != jobs.AuditLogKindPanic {
		return
	}
	if !tryAcquireRequestStatsAttempt() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), auditStatsPersistTimeout)
	defer cancel()
	if err := currentDeps().RecordStat(ctx, snapshot); err != nil {
		if markRequestStatsUnavailable() {
			log.Warn("Record request stats failed; statistics storage entered cooldown",
				zap.String("kind", kind),
				zap.String("request_id", snapshot.RequestID),
				zap.Error(err))
		}
		audit.RecordStatsFailure()
		return
	}
	markRequestStatsAvailable()
}

// auditStorageInCooldown 判断当前是否处于"存储不可用且冷却期未到"的短路状态。
// 标记为不可用但冷却期已过时返回 false，放行一条快照去探测存储是否恢复。
func auditStorageInCooldown() bool {
	if !storageUnavailable.Load() {
		return false
	}
	since := storageUnavailableSince.Load()
	if since == 0 {
		// 未记录时刻（如测试直接 Store(true)）：保守按仍不可用处理。
		return true
	}
	return time.Since(time.Unix(0, since)) < auditStorageRetryCooldown
}

// tryAcquireAuditStorageAttempt 在存储健康时直接放行；进入冷却后，仅在冷却到期时允许一个半开探测。
func tryAcquireAuditStorageAttempt() bool {
	if !storageUnavailable.Load() {
		return true
	}
	if auditStorageInCooldown() {
		return false
	}
	return storageRecoveryProbe.CompareAndSwap(false, true)
}

func requestStatsInCooldown() bool {
	if !statsUnavailable.Load() {
		return false
	}
	since := statsUnavailableSince.Load()
	if since == 0 {
		return true
	}
	return time.Since(time.Unix(0, since)) < auditStorageRetryCooldown
}

func tryAcquireRequestStatsAttempt() bool {
	if !statsUnavailable.Load() {
		return true
	}
	if requestStatsInCooldown() {
		return false
	}
	return statsRecoveryProbe.CompareAndSwap(false, true)
}

func markRequestStatsUnavailable() bool {
	firstTransition := statsUnavailable.CompareAndSwap(false, true)
	statsUnavailableSince.Store(time.Now().UnixNano())
	statsRecoveryProbe.Store(false)
	return firstTransition
}

func markRequestStatsAvailable() {
	statsUnavailable.Store(false)
	statsUnavailableSince.Store(0)
	statsRecoveryProbe.Store(false)
}

// markAuditStorageUnavailable 标记存储不可用并刷新计时；返回 true 表示首次从可用转入不可用，
// 供调用方仅在状态翻转时打印一条告警，避免持续故障期间日志风暴。
func markAuditStorageUnavailable() bool {
	firstTransition := storageUnavailable.CompareAndSwap(false, true)
	storageUnavailableSince.Store(time.Now().UnixNano())
	storageRecoveryProbe.Store(false)
	audit.SetStorageUnavailable(true)
	return firstTransition
}

// markAuditStorageAvailable 在一次成功落库后复位熔断状态。
func markAuditStorageAvailable() {
	storageUnavailable.Store(false)
	storageUnavailableSince.Store(0)
	storageRecoveryProbe.Store(false)
	audit.SetStorageUnavailable(false)
	if dropped := auditOverflowDropped.Swap(0); dropped > 0 {
		log.Warn("Audit log storage recovered after fallback overflow",
			zap.Uint64("dropped_count", dropped))
	}
	auditOverflowLogged.Store(false)
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
// 慢请求豁免：即便命中上述采样候选，若执行耗时 >= 阈值（默认 1s）也强制保留，避免慢请求被采样丢弃。
//
// 采样语义：保留 1/N，N<=1 表示不采样。
func shouldDropBySampling(snapshot *audit.AuditLogSnapshot) bool {
	if snapshot == nil {
		return false
	}
	// 个人请求记录以 operator_id 作为强制授权范围。已认证请求若参与采样，
	// 用户将看到随机缺失的历史，因此无论读写、快慢都始终保留。
	if snapshot.OperatorID > 0 {
		return false
	}
	// WebSocket 建连为低频且具运维价值的事件，不参与只读请求采样，始终保留。
	if snapshot.RequestKind == model.RequestKindWebSocket {
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
	// 慢请求即便是低风险只读成功，也具备性能排查价值，豁免采样始终保留。
	if thresholdUs := resolveAuditSampleSlowThresholdUs(); thresholdUs > 0 && snapshot.ExecutionTimeUs >= thresholdUs {
		return false
	}

	keepEvery := resolveAuditSampleKeepEvery()
	if keepEvery <= 1 {
		return false
	}
	// 用原子计数保证多协程下分布均匀；保留 N、2N、3N ... 计数对应的快照。
	n := auditSampleCounter.Add(1)
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

// resolveAuditSampleSlowThresholdUs 返回慢请求豁免阈值（微秒），耗时 >= 该值的低风险只读请求豁免采样。
// 配置 >0 时按配置值，否则用默认 1000ms。
func resolveAuditSampleSlowThresholdUs() int64 {
	thresholdMs := defaultAuditSampleSlowKeepThresholdMs
	if cfg := config.GetConfig(); cfg != nil && cfg.Queue.AuditSampleSlowKeepThresholdMs > 0 {
		thresholdMs = cfg.Queue.AuditSampleSlowKeepThresholdMs
	}
	return int64(thresholdMs) * 1000
}

// enqueueIntoFallbackBuffer 将快照投递到降级缓冲，缓冲满时回退到同步 INSERT 兜底。
func enqueueIntoFallbackBuffer(kind string, snapshot *audit.AuditLogSnapshot) {
	if fallbackBuffer.Offer(kind, snapshot) {
		audit.RecordFallbackBuffered()
		audit.SetFallbackPending(fallbackBuffer.pendingCount())
		return
	}
	if !tryAcquireAuditStorageAttempt() {
		reportAuditOverflowDrop(kind, snapshot)
		return
	}
	reportAuditPersistenceResult(kind, snapshot, "sync_fallback_buffer_full")
}

func reportAuditOverflowDrop(kind string, snapshot *audit.AuditLogSnapshot) {
	audit.RecordOverflowDrop()
	count := auditOverflowDropped.Add(1)
	if !auditOverflowLogged.CompareAndSwap(false, true) {
		return
	}
	log.Error("Audit fallback buffer full and synchronous fallback unavailable; record not persisted",
		zap.String("kind", kind),
		zap.String("request_id", snapshot.RequestID),
		zap.Uint64("dropped_count", count))
}

func reportAuditPersistenceResult(kind string, snapshot *audit.AuditLogSnapshot, mode string) {
	if snapshot == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), auditSyncPersistTimeout)
	defer cancel()
	if err := currentDeps().Persist(ctx, snapshot); err != nil {
		if isPermanentAuditPersistenceError(err) {
			audit.RecordPermanentDrop()
			log.Error("Drop permanently invalid audit log record",
				zap.String("kind", kind),
				zap.String("mode", mode),
				zap.String("request_id", snapshot.RequestID),
				zap.Error(err))
			return
		}
		audit.RecordPersistFailure()
		if markAuditStorageUnavailable() {
			log.Warn("Audit log storage unavailable, skip persistence",
				zap.String("kind", kind),
				zap.String("mode", mode),
				zap.String("request_id", snapshot.RequestID),
				zap.Bool("dependency_not_ready", perrors.IsDependencyNotReady(err)),
				zap.Error(err))
		}
		return
	}

	markAuditStorageAvailable()
	audit.RecordPersisted(1)
}

// resetSampleCounterForTesting 在单元测试间重置采样计数器，避免测试间的计数串扰。
func resetSampleCounterForTesting() {
	auditSampleCounter.Store(0)
}
