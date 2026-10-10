package jobs

import (
	"context"
	"errors"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/service"
)

func TestIsFinalExportAttempt(t *testing.T) {
	tests := []struct {
		name       string
		retryCount int
		maxRetry   int
		hasMeta    bool
		want       bool
	}{
		{name: "direct invocation fails closed", hasMeta: false, want: true},
		{name: "initial attempt will retry", retryCount: 0, maxRetry: 2, hasMeta: true, want: false},
		{name: "first retry will retry", retryCount: 1, maxRetry: 2, hasMeta: true, want: false},
		{name: "last attempt is terminal", retryCount: 2, maxRetry: 2, hasMeta: true, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isFinalExportAttempt(tt.retryCount, tt.maxRetry, tt.hasMeta); got != tt.want {
				t.Fatalf("isFinalExportAttempt()=%v want=%v", got, tt.want)
			}
		})
	}
}

func TestCommitGeneratedExportCleansOnlyLosingFile(t *testing.T) {
	winner := &model.SysExportRecord{FileID: 51, FileUUID: "winner", FileName: "winner.csv"}
	winner.Status = model.TaskRunStatusSuccess
	lifecycle := &stubExportLifecycle{commitRecord: winner, commitAccepted: false}
	cleaner := &stubGeneratedFileCleaner{}
	loser := &model.UploadFiles{UUID: "loser"}
	loser.ID = 52

	record, committed, err := commitGeneratedExport(context.Background(), lifecycle, cleaner, 901, service.ExportRecordSuccessInput{File: loser, FileName: "loser.csv"})
	if err != nil || committed || record != winner {
		t.Fatalf("unexpected commit result: committed=%v record=%#v err=%v", committed, record, err)
	}
	if len(cleaner.destroyedIDs) != 1 || cleaner.destroyedIDs[0] != loser.ID {
		t.Fatalf("expected losing file cleanup, got %#v", cleaner.destroyedIDs)
	}

	cleaner.destroyedIDs = nil
	sameFile := &model.UploadFiles{UUID: "winner"}
	sameFile.ID = winner.FileID
	if _, _, err := commitGeneratedExport(context.Background(), lifecycle, cleaner, 901, service.ExportRecordSuccessInput{File: sameFile}); err != nil {
		t.Fatalf("same-file idempotent commit failed: %v", err)
	}
	if len(cleaner.destroyedIDs) != 0 {
		t.Fatalf("winning file must not be cleaned: %#v", cleaner.destroyedIDs)
	}
}

func TestCommitGeneratedExportCleansFileAfterConfirmedCommitFailure(t *testing.T) {
	commitErr := errors.New("commit export result failed")
	file := &model.UploadFiles{UUID: "generated"}
	file.ID = 61

	t.Run("unreferenced generated file is cleaned", func(t *testing.T) {
		lifecycle := &stubExportLifecycle{commitErr: commitErr}
		cleaner := &stubGeneratedFileCleaner{}

		_, committed, err := commitGeneratedExport(context.Background(), lifecycle, cleaner, 902, service.ExportRecordSuccessInput{File: file})
		if err != commitErr || committed {
			t.Fatalf("expected original commit error, committed=%v err=%v", committed, err)
		}
		if len(cleaner.destroyedIDs) != 1 || cleaner.destroyedIDs[0] != file.ID {
			t.Fatalf("expected generated file cleanup, got %#v", cleaner.destroyedIDs)
		}
	})

	t.Run("cleanup failure does not mask commit error", func(t *testing.T) {
		cleanupErr := errors.New("delete object failed")
		lifecycle := &stubExportLifecycle{commitErr: commitErr}
		cleaner := &stubGeneratedFileCleaner{err: cleanupErr}

		_, _, err := commitGeneratedExport(context.Background(), lifecycle, cleaner, 903, service.ExportRecordSuccessInput{File: file})
		if err != commitErr {
			t.Fatalf("expected original commit error, got %v", err)
		}
		if len(cleaner.destroyedIDs) != 1 || cleaner.destroyedIDs[0] != file.ID {
			t.Fatalf("expected cleanup attempt, got %#v", cleaner.destroyedIDs)
		}
	})

	t.Run("confirmed committed file is retained", func(t *testing.T) {
		completed := &model.SysExportRecord{FileID: file.ID}
		completed.Status = model.TaskRunStatusSuccess
		lifecycle := &stubExportLifecycle{commitErr: commitErr, completedRecord: completed}
		cleaner := &stubGeneratedFileCleaner{}

		record, _, err := commitGeneratedExport(context.Background(), lifecycle, cleaner, 904, service.ExportRecordSuccessInput{File: file})
		if err != commitErr || record != completed {
			t.Fatalf("expected completed record and original error, record=%#v err=%v", record, err)
		}
		if len(cleaner.destroyedIDs) != 0 {
			t.Fatalf("committed file must not be cleaned: %#v", cleaner.destroyedIDs)
		}
	})

	t.Run("unknown commit state is retained", func(t *testing.T) {
		lookupErr := errors.New("lookup commit result failed")
		lifecycle := &stubExportLifecycle{commitErr: commitErr, completedErr: lookupErr}
		cleaner := &stubGeneratedFileCleaner{}

		_, _, err := commitGeneratedExport(context.Background(), lifecycle, cleaner, 905, service.ExportRecordSuccessInput{File: file})
		if err != commitErr {
			t.Fatalf("expected original commit error, got %v", err)
		}
		if len(cleaner.destroyedIDs) != 0 {
			t.Fatalf("file with unknown commit state must not be cleaned: %#v", cleaner.destroyedIDs)
		}
	})
}

type stubExportLifecycle struct {
	commitRecord    *model.SysExportRecord
	commitAccepted  bool
	commitErr       error
	completedRecord *model.SysExportRecord
	completedErr    error
}

func (s *stubExportLifecycle) ClaimExecution(context.Context, uint, string, ...string) (*model.SysExportRecord, bool, error) {
	return nil, false, nil
}

func (s *stubExportLifecycle) CommitSuccess(context.Context, uint, service.ExportRecordSuccessInput) (*model.SysExportRecord, bool, error) {
	return s.commitRecord, s.commitAccepted, s.commitErr
}

func (s *stubExportLifecycle) CompletedRecordForTaskRun(context.Context, uint) (*model.SysExportRecord, error) {
	return s.completedRecord, s.completedErr
}

func (s *stubExportLifecycle) MarkFailed(context.Context, uint, error, ...string) error {
	return nil
}

func (s *stubExportLifecycle) MarkPendingForRetry(context.Context, uint, error, ...string) error {
	return nil
}

type stubGeneratedFileCleaner struct {
	destroyedIDs []uint
	err          error
}

func (s *stubGeneratedFileCleaner) Destroy(id uint) error {
	s.destroyedIDs = append(s.destroyedIDs, id)
	return s.err
}
