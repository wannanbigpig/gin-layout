package audit

import "testing"

func TestRuntimeMetricsSnapshotTracksAuditOutcomes(t *testing.T) {
	before := RuntimeMetrics()
	RecordAccepted()
	RecordSampledOut()
	RecordQueueEnqueued()
	RecordQueueFallback()
	RecordFallbackBuffered()
	RecordPersisted(2)
	RecordPersistFailure()
	RecordPermanentDrop()
	RecordOverflowDrop()
	RecordStatsFailure()
	SetFallbackPending(3)
	SetStorageUnavailable(true)
	t.Cleanup(func() {
		SetFallbackPending(int(before.FallbackPending))
		SetStorageUnavailable(before.StorageUnavailable)
	})

	after := RuntimeMetrics()
	if after.Accepted != before.Accepted+1 || after.SampledOut != before.SampledOut+1 {
		t.Fatalf("accepted/sample metrics mismatch: before=%#v after=%#v", before, after)
	}
	if after.QueueEnqueued != before.QueueEnqueued+1 || after.QueueFallback != before.QueueFallback+1 || after.FallbackBuffered != before.FallbackBuffered+1 {
		t.Fatalf("queue metrics mismatch: before=%#v after=%#v", before, after)
	}
	if after.Persisted != before.Persisted+2 || after.PersistFailures != before.PersistFailures+1 {
		t.Fatalf("persistence metrics mismatch: before=%#v after=%#v", before, after)
	}
	if after.PermanentDropped != before.PermanentDropped+1 || after.OverflowDropped != before.OverflowDropped+1 {
		t.Fatalf("drop metrics mismatch: before=%#v after=%#v", before, after)
	}
	if after.StatsFailures != before.StatsFailures+1 || after.FallbackPending != 3 || !after.StorageUnavailable {
		t.Fatalf("health metrics mismatch: before=%#v after=%#v", before, after)
	}
}
