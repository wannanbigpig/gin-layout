package audit

import "sync/atomic"

// RuntimeMetricsSnapshot 是当前 service 进程内审计链路的累计指标。
// 多实例部署时应由监控系统按实例采集并聚合，不能把单个进程值视为全局总量。
type RuntimeMetricsSnapshot struct {
	Accepted           uint64 `json:"accepted"`
	SampledOut         uint64 `json:"sampled_out"`
	QueueEnqueued      uint64 `json:"queue_enqueued"`
	QueueFallback      uint64 `json:"queue_fallback"`
	FallbackBuffered   uint64 `json:"fallback_buffered"`
	Persisted          uint64 `json:"persisted"`
	PersistFailures    uint64 `json:"persist_failures"`
	PermanentDropped   uint64 `json:"permanent_dropped"`
	OverflowDropped    uint64 `json:"overflow_dropped"`
	StatsFailures      uint64 `json:"stats_failures"`
	FallbackPending    int64  `json:"fallback_pending"`
	StorageUnavailable bool   `json:"storage_unavailable"`
}

var auditRuntimeMetrics struct {
	accepted           atomic.Uint64
	sampledOut         atomic.Uint64
	queueEnqueued      atomic.Uint64
	queueFallback      atomic.Uint64
	fallbackBuffered   atomic.Uint64
	persisted          atomic.Uint64
	persistFailures    atomic.Uint64
	permanentDropped   atomic.Uint64
	overflowDropped    atomic.Uint64
	statsFailures      atomic.Uint64
	fallbackPending    atomic.Int64
	storageUnavailable atomic.Bool
}

func RecordAccepted()                  { auditRuntimeMetrics.accepted.Add(1) }
func RecordSampledOut()                { auditRuntimeMetrics.sampledOut.Add(1) }
func RecordQueueEnqueued()             { auditRuntimeMetrics.queueEnqueued.Add(1) }
func RecordQueueFallback()             { auditRuntimeMetrics.queueFallback.Add(1) }
func RecordFallbackBuffered()          { auditRuntimeMetrics.fallbackBuffered.Add(1) }
func RecordPersistFailure()            { auditRuntimeMetrics.persistFailures.Add(1) }
func RecordPermanentDrop()             { auditRuntimeMetrics.permanentDropped.Add(1) }
func RecordOverflowDrop()              { auditRuntimeMetrics.overflowDropped.Add(1) }
func RecordStatsFailure()              { auditRuntimeMetrics.statsFailures.Add(1) }
func SetFallbackPending(count int)     { auditRuntimeMetrics.fallbackPending.Store(int64(count)) }
func SetStorageUnavailable(value bool) { auditRuntimeMetrics.storageUnavailable.Store(value) }

func RecordPersisted(count int) {
	if count > 0 {
		auditRuntimeMetrics.persisted.Add(uint64(count))
	}
}

// RuntimeMetrics 返回无副作用的进程内审计指标快照。
func RuntimeMetrics() RuntimeMetricsSnapshot {
	return RuntimeMetricsSnapshot{
		Accepted:           auditRuntimeMetrics.accepted.Load(),
		SampledOut:         auditRuntimeMetrics.sampledOut.Load(),
		QueueEnqueued:      auditRuntimeMetrics.queueEnqueued.Load(),
		QueueFallback:      auditRuntimeMetrics.queueFallback.Load(),
		FallbackBuffered:   auditRuntimeMetrics.fallbackBuffered.Load(),
		Persisted:          auditRuntimeMetrics.persisted.Load(),
		PersistFailures:    auditRuntimeMetrics.persistFailures.Load(),
		PermanentDropped:   auditRuntimeMetrics.permanentDropped.Load(),
		OverflowDropped:    auditRuntimeMetrics.overflowDropped.Load(),
		StatsFailures:      auditRuntimeMetrics.statsFailures.Load(),
		FallbackPending:    auditRuntimeMetrics.fallbackPending.Load(),
		StorageUnavailable: auditRuntimeMetrics.storageUnavailable.Load(),
	}
}
