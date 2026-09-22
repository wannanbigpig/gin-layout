package service

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/filestorage"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"gorm.io/gorm"
)

func TestCreateFromReaderCleansUploadedObjectWhenLogicalFileCreationFails(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	deleteCalls := 0
	driver := fakeFileResourceDriver{
		name: model.StorageDriverAliyunOSS,
		onDelete: func(_, _ string) {
			deleteCalls++
		},
	}
	svc := newGeneratedFileTestService(db, driver)

	_, err := svc.CreateFromReader(context.Background(), ServerGeneratedFileInput{
		UID:         7,
		FolderID:    999,
		OriginName:  "report.csv",
		MimeType:    "text/csv",
		UploadScene: "export",
		Reader:      bytes.NewBufferString("id,name\n1,test\n"),
	})
	if err == nil {
		t.Fatal("expected missing target folder to fail after upload")
	}
	if deleteCalls != 1 {
		t.Fatalf("expected uploaded object cleanup, got %d delete calls", deleteCalls)
	}
	var objectCount int64
	if err := db.Model(&model.UploadFileObject{}).Count(&objectCount).Error; err != nil {
		t.Fatalf("count file objects failed: %v", err)
	}
	if objectCount != 0 {
		t.Fatalf("expected cleaned object row, got %d", objectCount)
	}
}

func TestCreateFromReaderMarksObjectWhenCompensationDeleteFails(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	driver := fakeFileResourceDriver{
		name:      model.StorageDriverAliyunOSS,
		deleteErr: errors.New("object store unavailable"),
	}
	svc := newGeneratedFileTestService(db, driver)

	_, err := svc.CreateFromReader(context.Background(), ServerGeneratedFileInput{
		UID:         7,
		FolderID:    999,
		OriginName:  "report.csv",
		MimeType:    "text/csv",
		UploadScene: "export",
		Reader:      bytes.NewBufferString("id,name\n1,test\n"),
	})
	if err == nil {
		t.Fatal("expected missing target folder to fail after upload")
	}
	var object model.UploadFileObject
	if err := db.First(&object).Error; err != nil {
		t.Fatalf("expected failed cleanup marker: %v", err)
	}
	if object.Status != model.StorageStatusDeleteFailed {
		t.Fatalf("expected delete_failed marker, got %#v", object)
	}
}

func newGeneratedFileTestService(db *gorm.DB, driver filestorage.Driver) *FileResourceService {
	config := filestorage.DefaultConfig()
	config.AliyunOSS.Bucket = "private-test"
	return NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return driver, config, model.StorageDriverAliyunOSS, nil
		},
		StorageDriverResolver: func(context.Context, string) (filestorage.Driver, filestorage.Config, error) {
			return driver, config, nil
		},
	})
}
