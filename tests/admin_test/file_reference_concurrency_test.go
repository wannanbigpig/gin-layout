package admin_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/service"
)

func TestFileDeleteWaitsForConcurrentReferenceCommit(t *testing.T) {
	requireWritableDB(t)
	db, err := model.GetDB()
	if err != nil {
		t.Fatal(err)
	}
	file := model.UploadFiles{UUID: fmt.Sprintf("%032x", time.Now().UnixNano()), OriginName: uniqueTestName("reference_lock"), StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored}
	if err := db.Create(&file).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Where("file_id = ?", file.ID).Delete(&model.UploadFileReference{}).Error; err != nil {
			t.Error(err)
		}
		if err := db.Unscoped().Delete(&model.UploadFiles{}, file.ID).Error; err != nil {
			t.Error(err)
		}
	})
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	if err := service.NewFileReferenceService(tx).BindReference("/admin/v1/file/"+file.UUID, "review_test", file.ID, "attachment"); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- service.NewFileResourceServiceWithDeps(service.FileResourceServiceDeps{DB: db}).Delete(file.ID, 1, "")
	}()
	select {
	case err := <-done:
		t.Fatalf("delete must wait for pending reference transaction, got %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		var referenced *service.FileReferencedDeleteError
		if !errors.As(err, &referenced) {
			t.Fatalf("expected committed reference to prevent deletion, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("delete did not finish after reference commit")
	}
	var stored model.UploadFiles
	if err := db.First(&stored, file.ID).Error; err != nil {
		t.Fatalf("referenced file disappeared: %v", err)
	}
}
