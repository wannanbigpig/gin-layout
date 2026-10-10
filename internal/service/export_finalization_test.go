package service

import (
	"context"
	"testing"
	"time"

	"github.com/wannanbigpig/gin-layout/internal/model"
)

func TestExportFinalizesAfterExecutionDeadline(t *testing.T) {
	for _, retry := range []bool{false, true} {
		t.Run(map[bool]string{false: "final attempt", true: "automatic retry"}[retry], func(t *testing.T) {
			db := newExportCenterTestDB(t)
			s := NewExportCenterServiceWithDeps(ExportCenterServiceDeps{DB: db})
			record, err := s.CreateRecord(context.Background(), ExportRecordCreateInput{Scene: model.ExportSceneFileList, TaskRunID: 1})
			if err != nil {
				t.Fatal(err)
			}
			if _, claimed, err := s.ClaimExecution(context.Background(), 1, "timeout-task"); err != nil || !claimed {
				t.Fatalf("claim=%v err=%v", claimed, err)
			}
			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			defer cancel()
			want := model.TaskRunStatusFailed
			if retry {
				want = model.TaskRunStatusPending
				err = s.MarkPendingForRetry(ctx, 1, ctx.Err())
			} else {
				err = s.MarkFailed(ctx, 1, ctx.Err())
			}
			if err != nil {
				t.Fatal(err)
			}
			var stored model.SysExportRecord
			if err := db.First(&stored, record.ID).Error; err != nil {
				t.Fatal(err)
			}
			if stored.Status != want || stored.ExecutionToken != "" {
				t.Fatalf("unfinished export: %+v", stored)
			}
			if retry && stored.FinishedAt != nil {
				t.Fatal("retry was finalized")
			}
			if !retry && stored.FinishedAt == nil {
				t.Fatal("terminal export has no finish time")
			}
		})
	}
}
