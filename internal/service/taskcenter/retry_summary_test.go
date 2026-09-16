package taskcenter

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"gorm.io/gorm"
)

func TestRetrySummariesBatchLatestRows(t *testing.T) {
	db := newTaskCenterTestDB(t)
	for _, run := range []model.TaskRun{
		{RetryRootRunID: 10, RetrySeq: 1, Status: model.TaskRunStatusFailed},
		{RetryRootRunID: 10, RetrySeq: 2, Status: model.TaskRunStatusRunning},
		{RetryRootRunID: 20, RetrySeq: 1, Status: model.TaskRunStatusFailed},
		{RetryRootRunID: 20, RetrySeq: 2, Status: model.TaskRunStatusSuccess},
	} {
		if err := db.Create(&run).Error; err != nil {
			t.Fatal(err)
		}
	}
	var queries atomic.Int32
	if err := db.Callback().Query().Before("gorm:query").Register("count_retry_queries", func(tx *gorm.DB) {
		if !tx.DryRun {
			queries.Add(1)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Callback().Row().Before("gorm:row").Register("count_retry_rows", func(tx *gorm.DB) {
		if !tx.DryRun {
			queries.Add(1)
		}
	}); err != nil {
		t.Fatal(err)
	}
	result, err := loadTaskRetryChainSummaries(context.Background(), db, []uint{10, 20, 30})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 || result[10].LatestRetryRunID != 2 || result[20].LatestRetryRunID != 4 || result[10].ActiveCount != 1 || result[20].SuccessCount != 1 {
		t.Fatalf("unexpected summaries: %#v", result)
	}
	if queries.Load() != 2 {
		t.Fatalf("expected 2 queries regardless of chain count, got %d", queries.Load())
	}
}
