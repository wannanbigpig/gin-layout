package service

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

func TestFileAuditWrappersCaptureCommittedBeforeAndAfter(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db})

	source := model.UploadFileFolder{Name: "source", LogicalPath: "/source"}
	target := model.UploadFileFolder{Name: "target", LogicalPath: "/target"}
	if err := db.Create(&source).Error; err != nil {
		t.Fatalf("create source folder: %v", err)
	}
	if err := db.Create(&target).Error; err != nil {
		t.Fatalf("create target folder: %v", err)
	}

	renameFolder := model.UploadFileFolder{ParentID: source.ID, Name: "old-folder", LogicalPath: "/source/old-folder"}
	moveFolder := model.UploadFileFolder{ParentID: source.ID, Name: "move-folder", LogicalPath: "/source/move-folder"}
	if err := db.Create(&renameFolder).Error; err != nil {
		t.Fatalf("create rename folder: %v", err)
	}
	if err := db.Create(&moveFolder).Error; err != nil {
		t.Fatalf("create move folder: %v", err)
	}

	renameFile := newFileAuditTestFile("old-name.txt", source.ID, "/source")
	moveFile := newFileAuditTestFile("move-name.txt", source.ID, "/source")
	if err := db.Create(&renameFile).Error; err != nil {
		t.Fatalf("create rename file: %v", err)
	}
	if err := db.Create(&moveFile).Error; err != nil {
		t.Fatalf("create move file: %v", err)
	}

	_, fileDiff, err := service.UpdateFileWithAudit(&form.FileUpdate{ID: renameFile.ID, OriginName: "new-name.txt"})
	if err != nil {
		t.Fatalf("UpdateFileWithAudit() error = %v", err)
	}
	if len(fileDiff.Before) != 1 || len(fileDiff.After) != 1 ||
		fileDiff.Before[0].OriginName != "old-name.txt" || fileDiff.After[0].OriginName != "new-name.txt" {
		t.Fatalf("unexpected file update audit diff: %#v", fileDiff)
	}

	_, folderUpdateDiff, err := service.UpdateFolderWithAudit(&form.FileFolderUpdate{ID: renameFolder.ID, Name: "new-folder"}, 7)
	if err != nil {
		t.Fatalf("UpdateFolderWithAudit() error = %v", err)
	}
	if folderUpdateDiff.Before.Name != "old-folder" || folderUpdateDiff.After.Name != "new-folder" ||
		folderUpdateDiff.After.LogicalPath != "/source/new-folder" {
		t.Fatalf("unexpected folder update audit diff: %#v", folderUpdateDiff)
	}

	_, folderMoveDiff, err := service.MoveFolderWithAudit(&form.FileFolderMove{ID: moveFolder.ID, ParentID: target.ID}, 7)
	if err != nil {
		t.Fatalf("MoveFolderWithAudit() error = %v", err)
	}
	if folderMoveDiff.Before.ParentID != source.ID || folderMoveDiff.After.ParentID != target.ID ||
		folderMoveDiff.After.LogicalPath != "/target/move-folder" {
		t.Fatalf("unexpected folder move audit diff: %#v", folderMoveDiff)
	}

	result, filesMoveDiff, err := service.MoveFilesWithAudit(&form.FileMove{IDs: []uint{moveFile.ID}, FolderID: target.ID})
	if err != nil {
		t.Fatalf("MoveFilesWithAudit() error = %v", err)
	}
	if result.Moved != 1 || len(filesMoveDiff.Before) != 1 || len(filesMoveDiff.After) != 1 ||
		filesMoveDiff.Before[0].FolderID != source.ID || filesMoveDiff.After[0].FolderID != target.ID ||
		filesMoveDiff.After[0].LogicalPath != "/target" {
		t.Fatalf("unexpected files move audit result=%#v diff=%#v", result, filesMoveDiff)
	}
}

func TestRunFileAuditTransactionRollsBackWhenAfterSnapshotFails(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db})
	file := newFileAuditTestFile("before.txt", 0, "/")
	if err := db.Create(&file).Error; err != nil {
		t.Fatalf("create file: %v", err)
	}

	expectedErr := errors.New("after snapshot failed")
	snapshotCalls := 0
	_, _, err := runFileAuditTransaction(
		service,
		func(tx *gorm.DB) error { return lockFileAuditRows(tx, []uint{file.ID}) },
		func(_ *FileResourceService) (string, error) {
			snapshotCalls++
			if snapshotCalls == 2 {
				return "", expectedErr
			}
			return "before", nil
		},
		func(txService *FileResourceService) (uint, error) {
			return file.ID, txService.db.Model(&model.UploadFiles{}).
				Where("id = ?", file.ID).
				Update("origin_name", "should-rollback.txt").Error
		},
	)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected after snapshot error, got %v", err)
	}
	var stored model.UploadFiles
	if err := db.First(&stored, file.ID).Error; err != nil {
		t.Fatalf("query file after rollback: %v", err)
	}
	if stored.OriginName != "before.txt" {
		t.Fatalf("mutation was not rolled back: %#v", stored)
	}
}

func TestSortedPositiveUniqueIDsUsesStableFolderLockOrder(t *testing.T) {
	got := sortedPositiveUniqueIDs([]uint{9, 3, 0, 9, 5})
	want := []uint{3, 5, 9}
	if len(got) != len(want) {
		t.Fatalf("lock ids = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("lock ids = %v, want %v", got, want)
		}
	}
}

func newFileAuditTestFile(name string, folderID uint, logicalPath string) model.UploadFiles {
	return model.UploadFiles{
		FolderID:      folderID,
		LogicalPath:   logicalPath,
		OriginName:    name,
		DisplayName:   name,
		Name:          name,
		UUID:          fmt.Sprintf("%032x", time.Now().UnixNano()),
		StorageDriver: model.StorageDriverLocal,
		StorageStatus: model.StorageStatusStored,
	}
}
