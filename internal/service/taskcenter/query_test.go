package taskcenter

import (
	"testing"
	"time"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
	"gorm.io/gorm"
)

func TestNormalizeTaskRunStatsRangeDefaultsToRecentWindow(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.Local)

	start, end, err := normalizeTaskRunStatsRange(&form.TaskRunStatsQuery{}, now)
	if err != nil {
		t.Fatalf("normalizeTaskRunStatsRange returned error: %v", err)
	}
	if !end.Equal(now) {
		t.Fatalf("expected end to be now, got %s", end)
	}
	if !start.Equal(now.Add(-taskRunStatsDefaultWindow)) {
		t.Fatalf("expected default start %s, got %s", now.Add(-taskRunStatsDefaultWindow), start)
	}
}

func TestNormalizeTaskRunStatsRangeRejectsOversizedWindow(t *testing.T) {
	_, _, err := normalizeTaskRunStatsRange(&form.TaskRunStatsQuery{
		StartTime: "2026-01-01 00:00:00",
		EndTime:   "2026-02-02 00:00:00",
	}, time.Date(2026, 2, 2, 0, 0, 0, 0, time.Local))
	if err == nil {
		t.Fatal("expected oversized stats range to fail")
	}
	if be, ok := err.(*e.BusinessError); !ok || be.GetCode() != e.InvalidParameter {
		t.Fatalf("expected InvalidParameter business error, got %T %v", err, err)
	}
}

func TestTaskRunStatsTrendDefaultWindowExcludesOldRows(t *testing.T) {
	db := newTaskCenterTestDB(t)
	service := NewTaskCenterServiceWithDeps(TaskCenterServiceDeps{DB: db})
	now := time.Now()
	insertTaskRunStat(t, db, "demo:send", now.Add(-10*24*time.Hour), 99)
	insertTaskRunStat(t, db, "demo:send", now.Add(-time.Hour), 3)

	summary, err := service.TaskRunStats(&form.TaskRunStatsQuery{TaskCode: "demo:send"})
	if err != nil {
		t.Fatalf("TaskRunStats returned error: %v", err)
	}
	if summary.TotalCount != 3 {
		t.Fatalf("expected summary to exclude old rows, got total %d", summary.TotalCount)
	}

	trend, err := service.TaskRunStatsTrend(&form.TaskRunStatsQuery{TaskCode: "demo:send"})
	if err != nil {
		t.Fatalf("TaskRunStatsTrend returned error: %v", err)
	}
	if len(trend) != 1 {
		t.Fatalf("expected one trend point, got %d", len(trend))
	}
	if trend[0].TotalCount != 3 {
		t.Fatalf("expected trend to exclude old rows, got total %d", trend[0].TotalCount)
	}
}

func TestTaskRunStatsTrendUsesHourWindowWithinOneDay(t *testing.T) {
	db := newTaskCenterTestDB(t)
	service := NewTaskCenterServiceWithDeps(TaskCenterServiceDeps{DB: db})
	start := time.Date(2026, 6, 10, 8, 0, 0, 0, time.Local)
	insertTaskRunStat(t, db, "demo:send", start.Add(time.Hour), 3)
	insertTaskRunStat(t, db, "demo:send", start.Add(2*time.Hour), 5)

	trend, err := service.TaskRunStatsTrend(&form.TaskRunStatsQuery{
		TaskCode:  "demo:send",
		StartTime: "2026-06-10 00:00:00",
		EndTime:   "2026-06-11 00:00:00",
	})
	if err != nil {
		t.Fatalf("TaskRunStatsTrend returned error: %v", err)
	}
	if len(trend) != 2 {
		t.Fatalf("expected hourly trend points, got %d", len(trend))
	}
	for _, item := range trend {
		if item.WindowSize != model.TaskRunStatsWindowHour {
			t.Fatalf("expected hour window size, got %#v", item)
		}
	}
}

func TestTaskRunStatsTrendAggregatesByDayWhenRangeExceedsOneDay(t *testing.T) {
	db := newTaskCenterTestDB(t)
	service := NewTaskCenterServiceWithDeps(TaskCenterServiceDeps{DB: db})
	firstDay := time.Date(2026, 6, 9, 18, 0, 0, 0, time.Local)
	secondDay := time.Date(2026, 6, 10, 9, 0, 0, 0, time.Local)
	insertTaskRunStatDetail(t, db, taskRunStatSeed{
		TaskCode:        "demo:send",
		WindowStart:     firstDay,
		TotalCount:      3,
		SuccessCount:    2,
		FailedCount:     1,
		DurationTotalMS: 30,
		DurationMaxMS:   20,
	})
	insertTaskRunStatDetail(t, db, taskRunStatSeed{
		TaskCode:        "demo:send",
		WindowStart:     firstDay.Add(time.Hour),
		TotalCount:      5,
		SuccessCount:    4,
		FailedCount:     1,
		DurationTotalMS: 80,
		DurationMaxMS:   50,
	})
	insertTaskRunStatDetail(t, db, taskRunStatSeed{
		TaskCode:        "demo:send",
		WindowStart:     secondDay,
		TotalCount:      7,
		SuccessCount:    7,
		DurationTotalMS: 70,
		DurationMaxMS:   30,
	})

	trend, err := service.TaskRunStatsTrend(&form.TaskRunStatsQuery{
		TaskCode:  "demo:send",
		StartTime: "2026-06-09 18:00:00",
		EndTime:   "2026-06-10 18:00:01",
	})
	if err != nil {
		t.Fatalf("TaskRunStatsTrend returned error: %v", err)
	}
	if len(trend) != 2 {
		t.Fatalf("expected daily trend points, got %d", len(trend))
	}
	if trend[0].WindowSize != model.TaskRunStatsWindowDay || trend[0].WindowStart.String() != "2026-06-09 00:00:00" {
		t.Fatalf("expected first point to be 2026-06-09 day, got %#v", trend[0])
	}
	if trend[0].TotalCount != 8 || trend[0].SuccessCount != 6 || trend[0].FailedCount != 2 || trend[0].DurationTotalMS != 110 || trend[0].DurationMaxMS != 50 {
		t.Fatalf("expected first day totals to aggregate hourly rows, got %#v", trend[0])
	}
	if trend[1].WindowSize != model.TaskRunStatsWindowDay || trend[1].WindowStart.String() != "2026-06-10 00:00:00" || trend[1].TotalCount != 7 {
		t.Fatalf("expected second point to be 2026-06-10 day, got %#v", trend[1])
	}
}

func TestListTaskRunsFillsMissingTriggerAccountFromAdminUser(t *testing.T) {
	db := newTaskCenterTestDB(t)
	now := time.Now()
	insertAdminUser(t, db, 1, "super_admin", 0)
	insertTaskRun(t, db, 101, 1, "", now)
	insertTaskRun(t, db, 102, 1, "recorded_admin", now.Add(time.Second))

	result := NewTaskCenterServiceWithDeps(TaskCenterServiceDeps{DB: db}).ListTaskRuns(&form.TaskRunList{
		Paginate: form.Paginate{Page: 1, PerPage: 10},
	})
	if result.Total != 2 || len(result.Data) != 2 {
		t.Fatalf("expected 2 task runs, got %#v", result)
	}
	first, ok := result.Data[0].(*resources.TaskRunListResources)
	if !ok {
		t.Fatalf("unexpected first item type: %T", result.Data[0])
	}
	if first.ID != 102 || first.TriggerAccount != "recorded_admin" {
		t.Fatalf("expected existing trigger_account to be kept, got %#v", first)
	}
	second, ok := result.Data[1].(*resources.TaskRunListResources)
	if !ok {
		t.Fatalf("unexpected second item type: %T", result.Data[1])
	}
	if second.ID != 101 || second.TriggerAccount != "super_admin" {
		t.Fatalf("expected missing trigger_account to be filled, got %#v", second)
	}
}

func TestTaskRunDetailFillsMissingTriggerAccountFromAdminUser(t *testing.T) {
	db := newTaskCenterTestDB(t)
	insertAdminUser(t, db, 1, "super_admin", 0)
	insertTaskRun(t, db, 101, 1, "", time.Now())

	detail, err := NewTaskCenterServiceWithDeps(TaskCenterServiceDeps{DB: db}).TaskRunDetail(101)
	if err != nil {
		t.Fatalf("TaskRunDetail returned error: %v", err)
	}
	item, ok := detail.(*resources.TaskRunResources)
	if !ok {
		t.Fatalf("unexpected detail type: %T", detail)
	}
	if item.TriggerAccount != "super_admin" {
		t.Fatalf("expected trigger_account to be filled, got %#v", item)
	}
}

func TestTaskRunDetailExposesLatestRetryRunForRootFailure(t *testing.T) {
	db := newTaskCenterTestDB(t)
	now := time.Now()
	insertTaskRunWithRetry(t, db, taskRunSeed{
		ID:        166,
		CreatedAt: now,
		Status:    model.TaskRunStatusFailed,
		MaxRetry:  25,
	})
	insertTaskRunWithRetry(t, db, taskRunSeed{
		ID:             1709,
		CreatedAt:      now.Add(time.Second),
		Status:         model.TaskRunStatusSuccess,
		MaxRetry:       25,
		RetryOfRunID:   166,
		RetryRootRunID: 166,
		RetrySeq:       1,
	})

	detail, err := NewTaskCenterServiceWithDeps(TaskCenterServiceDeps{DB: db}).TaskRunDetail(166)
	if err != nil {
		t.Fatalf("TaskRunDetail returned error: %v", err)
	}
	item, ok := detail.(*resources.TaskRunResources)
	if !ok {
		t.Fatalf("unexpected detail type: %T", detail)
	}
	if item.RetryCount != 1 || item.LatestRetryRunID != 1709 || item.LatestRetryStatus != model.TaskRunStatusSuccess {
		t.Fatalf("expected root failure to expose latest retry summary, got %#v", item)
	}
}

func insertTaskRunStat(t *testing.T, db *gorm.DB, taskCode string, windowStart time.Time, total uint64) {
	t.Helper()
	insertTaskRunStatDetail(t, db, taskRunStatSeed{
		TaskCode:     taskCode,
		WindowStart:  windowStart,
		TotalCount:   total,
		SuccessCount: total,
	})
}

type taskRunStatSeed struct {
	TaskCode        string
	WindowStart     time.Time
	TotalCount      uint64
	SuccessCount    uint64
	FailedCount     uint64
	DurationTotalMS float64
	DurationMaxMS   float64
}

func insertTaskRunStatDetail(t *testing.T, db *gorm.DB, seed taskRunStatSeed) {
	t.Helper()
	if seed.TaskCode == "" {
		seed.TaskCode = "demo:send"
	}
	stat := &model.TaskRunStat{
		TaskCode:        seed.TaskCode,
		Kind:            model.TaskKindAsync,
		Source:          model.TaskSourceQueue,
		WindowStart:     utils.FormatDate{Time: seed.WindowStart},
		WindowSize:      model.TaskRunStatsWindowHour,
		TotalCount:      seed.TotalCount,
		SuccessCount:    seed.SuccessCount,
		FailedCount:     seed.FailedCount,
		DurationTotalMS: seed.DurationTotalMS,
		DurationMaxMS:   seed.DurationMaxMS,
	}
	if err := db.Create(stat).Error; err != nil {
		t.Fatalf("insert task run stat failed: %v", err)
	}
}

func insertAdminUser(t *testing.T, db *gorm.DB, id uint, username string, deletedAt int64) {
	t.Helper()
	now := time.Now()
	if err := db.Exec(`INSERT INTO admin_user (id, created_at, updated_at, deleted_at, username, nickname) VALUES (?, ?, ?, ?, ?, ?)`,
		id, now, now, deletedAt, username, username).Error; err != nil {
		t.Fatalf("insert admin user failed: %v", err)
	}
}

func insertTaskRun(t *testing.T, db *gorm.DB, id uint, triggerUserID uint, triggerAccount string, createdAt time.Time) {
	t.Helper()
	if err := db.Exec(`INSERT INTO task_runs (id, created_at, updated_at, task_code, kind, source, source_id, queue, status, trigger_user_id, trigger_account, detail_record_mode) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, createdAt, createdAt, "export:request-log", model.TaskKindAsync, model.TaskSourceManual, "manual:test", "export", model.TaskRunStatusSuccess, triggerUserID, triggerAccount, model.TaskDetailRecordForced).Error; err != nil {
		t.Fatalf("insert task run failed: %v", err)
	}
}

type taskRunSeed struct {
	ID             uint
	CreatedAt      time.Time
	Status         string
	MaxRetry       int
	RetryOfRunID   uint
	RetryRootRunID uint
	RetrySeq       int
}

func insertTaskRunWithRetry(t *testing.T, db *gorm.DB, seed taskRunSeed) {
	t.Helper()
	if seed.CreatedAt.IsZero() {
		seed.CreatedAt = time.Now()
	}
	if seed.Status == "" {
		seed.Status = model.TaskRunStatusFailed
	}
	if err := db.Exec(`INSERT INTO task_runs (id, created_at, updated_at, task_code, kind, source, source_id, queue, status, max_retry, retry_of_run_id, retry_root_run_id, retry_seq, detail_record_mode) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		seed.ID, seed.CreatedAt, seed.CreatedAt, "export:request-log", model.TaskKindAsync, model.TaskSourceManual, "manual:test", "export", seed.Status, seed.MaxRetry, seed.RetryOfRunID, seed.RetryRootRunID, seed.RetrySeq, model.TaskDetailRecordForced).Error; err != nil {
		t.Fatalf("insert task run failed: %v", err)
	}
}
