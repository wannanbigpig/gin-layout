package service

import (
	"context"
	"errors"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
)

func TestCleanupUnreferencedFileObjectDeletesPhysicalObjectAndRow(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	object := &model.UploadFileObject{
		StorageDriver: "custom_remote",
		Bucket:        "assets",
		StoragePath:   "generated/report.csv",
		ObjectKey:     "generated/report.csv",
		Status:        model.StorageStatusStored,
	}
	if err := db.Create(object).Error; err != nil {
		t.Fatal(err)
	}
	deleted := false
	driver := fakeFileResourceDriver{name: "custom_remote", onDelete: func(bucket, objectKey string) {
		deleted = bucket == "assets" && objectKey == "generated/report.csv"
	}}
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db})
	if err := service.cleanupUnreferencedFileObjectWithDriver(context.Background(), db, driver, object); err != nil {
		t.Fatal(err)
	}
	if !deleted {
		t.Fatal("physical object was not deleted")
	}
	var count int64
	if err := db.Model(&model.UploadFileObject{}).Where("id = ?", object.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("orphan object row was not deleted: %d", count)
	}
}

func TestCleanupUnreferencedFileObjectKeepsFailedDeletionForReconciliation(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	object := &model.UploadFileObject{
		StorageDriver: "custom_remote",
		Bucket:        "assets",
		StoragePath:   "generated/report.csv",
		ObjectKey:     "generated/report.csv",
		Status:        model.StorageStatusStored,
	}
	if err := db.Create(object).Error; err != nil {
		t.Fatal(err)
	}
	driver := fakeFileResourceDriver{name: "custom_remote", deleteErr: errors.New("provider unavailable")}
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db})
	if err := service.cleanupUnreferencedFileObjectWithDriver(context.Background(), db, driver, object); err == nil {
		t.Fatal("expected provider deletion failure")
	}
	var stored model.UploadFileObject
	if err := db.First(&stored, object.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.StorageStatusDeleteFailed {
		t.Fatalf("unexpected cleanup status %q", stored.Status)
	}
}
