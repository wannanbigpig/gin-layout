package asynqx

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/jobs"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/service"
	"github.com/wannanbigpig/gin-layout/internal/service/taskcenter"
)

func newFinalizationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	// 覆盖基础模型的 MySQL 专用整数声明，业务字段仍由真实模型生成。
	if err := db.AutoMigrate(
		&struct {
			model.TaskRun
			ID uint `gorm:"primaryKey;autoIncrement"`
		}{},
		&struct {
			model.TaskRunEvent
			ID uint `gorm:"primaryKey;autoIncrement"`
		}{},
		&struct {
			model.TaskRunStat
			ID uint `gorm:"primaryKey;autoIncrement"`
		}{},
		&struct {
			model.TaskDefinition
			ID        uint `gorm:"primaryKey;autoIncrement"`
			DeletedAt uint `gorm:"default:0"`
		}{},
		&struct {
			model.SysExportRecord
			ID uint `gorm:"primaryKey;autoIncrement"`
		}{},
	); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX task_run_stats_window ON task_run_stats (task_code, kind, source, window_start, window_size)").Error; err != nil {
		t.Fatal(err)
	}
	service.ResetExportRecordsTableCacheForTest()
	return db
}

func TestTimeoutCallbackFinalizesAndFencesLateHandler(t *testing.T) {
	db := newFinalizationDB(t)
	r := taskcenter.NewRecorderWithDB(db)
	s := service.NewExportCenterServiceWithDeps(service.ExportCenterServiceDeps{DB: db})
	first, err := r.Start(context.Background(), taskcenter.RunStart{TaskCode: jobs.TypeExportFileList, SourceID: "timeout-task", Attempt: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateRecord(context.Background(), service.ExportRecordCreateInput{Scene: model.ExportSceneFileList, TaskRunID: first.ID}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := s.ClaimExecution(context.Background(), first.ID, "timeout-task:attempt:1"); err != nil || !ok {
		t.Fatalf("claim=%v err=%v", ok, err)
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	finish := taskcenter.RunFinish{Status: model.TaskRunStatusRetrying, Error: context.DeadlineExceeded}
	if err := finishActiveAsynqTask(ctx, db, jobs.TypeExportFileList, "timeout-task", 1, finish); err != nil {
		t.Fatal(err)
	}
	var stored model.TaskRun
	if err := db.First(&stored, first.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.TaskRunStatusRetrying {
		t.Fatalf("status=%s", stored.Status)
	}
	var export model.SysExportRecord
	if err := db.Where("task_run_id = ?", first.ID).First(&export).Error; err != nil {
		t.Fatal(err)
	}
	if export.Status != model.TaskRunStatusPending {
		t.Fatalf("export status=%s", export.Status)
	}
	second, err := r.Start(context.Background(), taskcenter.RunStart{TaskCode: jobs.TypeExportFileList, SourceID: "timeout-task", Attempt: 2})
	if err != nil || second.ID != first.ID {
		t.Fatalf("retry=%+v err=%v", second, err)
	}
	if _, ok, err := s.ClaimExecution(context.Background(), second.ID, "timeout-task:attempt:2", "timeout-task:attempt:1"); err != nil || !ok {
		t.Fatalf("claim=%v err=%v", ok, err)
	}
	for _, lateErr := range []error{nil, context.DeadlineExceeded} {
		if err := r.Finish(ctx, first, taskcenter.RunFinish{Error: lateErr}); !errors.Is(err, taskcenter.ErrTaskRunStatusChanged) {
			t.Fatalf("old handler must be fenced: %v", err)
		}
	}
	if err := s.MarkPendingForRetry(ctx, first.ID, context.DeadlineExceeded, "timeout-task:attempt:1"); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkFailed(ctx, first.ID, context.DeadlineExceeded, "timeout-task:attempt:1"); err != nil {
		t.Fatal(err)
	}
	if err := finishActiveAsynqTask(ctx, db, jobs.TypeExportFileList, "timeout-task", 1, finish); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&stored, first.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&export, export.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.TaskRunStatusRunning || stored.Attempt != 2 || export.Status != model.TaskRunStatusRunning || export.ExecutionToken != "timeout-task:attempt:2" {
		t.Fatalf("late handler changed new attempt: run=%+v export=%+v", stored, export)
	}
	if err := r.Finish(context.Background(), second, taskcenter.RunFinish{}); err != nil {
		t.Fatal(err)
	}
	file := model.NewUploadFiles()
	file.ID = 9
	if err := s.MarkSuccess(context.Background(), second.ID, service.ExportRecordSuccessInput{File: file}); err != nil {
		t.Fatal(err)
	}
	if err := finishActiveAsynqTask(ctx, db, jobs.TypeExportFileList, "timeout-task", 2, taskcenter.RunFinish{Error: context.DeadlineExceeded}); err != nil {
		t.Fatal(err)
	}
	var stats model.TaskRunStat
	if err := db.First(&stats).Error; err != nil {
		t.Fatal(err)
	}
	if stats.TotalCount != 1 || stats.SuccessCount != 1 {
		t.Fatalf("duplicate statistics: %+v", stats)
	}
}

func TestTimeoutCallbackFinalizesLastAttempt(t *testing.T) {
	db := newFinalizationDB(t)
	r := taskcenter.NewRecorderWithDB(db)
	s := service.NewExportCenterServiceWithDeps(service.ExportCenterServiceDeps{DB: db})
	run, err := r.Start(context.Background(), taskcenter.RunStart{TaskCode: jobs.TypeExportFileList, SourceID: "final-task", Attempt: 3})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateRecord(context.Background(), service.ExportRecordCreateInput{Scene: model.ExportSceneFileList, TaskRunID: run.ID}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := s.ClaimExecution(context.Background(), run.ID, "final-task:attempt:3"); err != nil || !ok {
		t.Fatalf("claim=%v err=%v", ok, err)
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	finish := resolveAsynqRunFinish(ctx, nil)
	if !errors.Is(finish.Error, context.DeadlineExceeded) {
		t.Fatalf("expired execution cannot succeed: %+v", finish)
	}
	if err := finishActiveAsynqTask(ctx, db, jobs.TypeExportFileList, "final-task", 3, finish); err != nil {
		t.Fatal(err)
	}
	var stored model.TaskRun
	if err := db.First(&stored, run.ID).Error; err != nil {
		t.Fatal(err)
	}
	var export model.SysExportRecord
	if err := db.Where("task_run_id = ?", run.ID).First(&export).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.TaskRunStatusTimeout || export.Status != model.TaskRunStatusFailed {
		t.Fatalf("unfinished timeout: %s / %s", stored.Status, export.Status)
	}
	var stats model.TaskRunStat
	if err := db.First(&stats).Error; err != nil {
		t.Fatal(err)
	}
	if stats.TimeoutCount != 1 || stats.TotalCount != 1 {
		t.Fatalf("wrong timeout statistics: %+v", stats)
	}
}

func TestFailureCallbackClosesUnclaimedExport(t *testing.T) {
	for _, wrapperFinished := range []bool{false, true} {
		t.Run(map[bool]string{false: "timeout before claim", true: "claim error after wrapper finish"}[wrapperFinished], func(t *testing.T) {
			db := newFinalizationDB(t)
			r := taskcenter.NewRecorderWithDB(db)
			s := service.NewExportCenterServiceWithDeps(service.ExportCenterServiceDeps{DB: db})
			run, err := r.Start(context.Background(), taskcenter.RunStart{TaskCode: jobs.TypeExportFileList, SourceID: "unclaimed-task", Attempt: 3})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.CreateRecord(context.Background(), service.ExportRecordCreateInput{Scene: model.ExportSceneFileList, TaskRunID: run.ID}); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			defer cancel()
			finish := taskcenter.RunFinish{Error: context.DeadlineExceeded}
			if wrapperFinished {
				finish.Error = errors.New("claim failed")
				if err := r.Finish(ctx, run, finish); err != nil {
					t.Fatal(err)
				}
			}
			for i := 0; i < 2; i++ {
				if err := finishActiveAsynqTask(ctx, db, jobs.TypeExportFileList, "unclaimed-task", 3, finish); err != nil {
					t.Fatal(err)
				}
			}
			var export model.SysExportRecord
			if err := db.Where("task_run_id = ?", run.ID).First(&export).Error; err != nil {
				t.Fatal(err)
			}
			if export.Status != model.TaskRunStatusFailed || export.FinishedAt == nil {
				t.Fatalf("unclaimed export left active: %+v", export)
			}
			var stats []model.TaskRunStat
			if err := db.Find(&stats).Error; err != nil {
				t.Fatal(err)
			}
			if len(stats) == 0 {
				t.Fatal("missing final statistics")
			}
			for _, stat := range stats {
				if stat.TotalCount != 1 {
					t.Fatalf("callback duplicated statistics: %+v", stat)
				}
			}
		})
	}
}

func TestFailureCallbackPreservesCanceledTask(t *testing.T) {
	db := newFinalizationDB(t)
	r := taskcenter.NewRecorderWithDB(db)
	s := service.NewExportCenterServiceWithDeps(service.ExportCenterServiceDeps{DB: db})
	run, err := r.Start(context.Background(), taskcenter.RunStart{TaskCode: jobs.TypeExportFileList, SourceID: "canceled-task", Attempt: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateRecord(context.Background(), service.ExportRecordCreateInput{Scene: model.ExportSceneFileList, TaskRunID: run.ID}); err != nil {
		t.Fatal(err)
	}
	if err := r.Finish(context.Background(), run, taskcenter.RunFinish{Status: model.TaskRunStatusCanceled}); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkCanceledIfExists(context.Background(), run.ID, "operator canceled"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := finishActiveAsynqTask(ctx, db, jobs.TypeExportFileList, "canceled-task", 1, taskcenter.RunFinish{Error: context.Canceled}); err != nil {
		t.Fatal(err)
	}
	var export model.SysExportRecord
	if err := db.Where("task_run_id = ?", run.ID).First(&export).Error; err != nil {
		t.Fatal(err)
	}
	if export.Status != model.TaskRunStatusCanceled {
		t.Fatalf("canceled export overwritten: %+v", export)
	}
	var stat model.TaskRunStat
	if err := db.First(&stat).Error; err != nil {
		t.Fatal(err)
	}
	if stat.TotalCount != 1 || stat.CanceledCount != 1 {
		t.Fatalf("canceled statistics overwritten: %+v", stat)
	}
}
