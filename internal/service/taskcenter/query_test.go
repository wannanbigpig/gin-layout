package taskcenter

import (
	"testing"
	"time"

	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
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

func insertTaskRunStat(t *testing.T, db *gorm.DB, taskCode string, windowStart time.Time, total uint64) {
	t.Helper()
	stat := &model.TaskRunStat{
		TaskCode:     taskCode,
		Kind:         model.TaskKindAsync,
		Source:       model.TaskSourceQueue,
		WindowStart:  utils.FormatDate{Time: windowStart},
		WindowSize:   model.TaskRunStatsWindowHour,
		TotalCount:   total,
		SuccessCount: total,
	}
	if err := db.Create(stat).Error; err != nil {
		t.Fatalf("insert task run stat failed: %v", err)
	}
}
