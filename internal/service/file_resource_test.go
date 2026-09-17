package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	stderrors "errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/plugin/soft_delete"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/filestorage"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

func TestFileResourceDeleteReturnsReferencedError(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	uuid := fmt.Sprintf("%032x", time.Now().UnixNano())
	originName := "test-referenced-" + uuid + ".jpg"

	file := model.UploadFiles{
		OriginName:    originName,
		Name:          uuid + ".jpg",
		UUID:          uuid,
		StorageDriver: model.StorageDriverLocal,
		StorageStatus: model.StorageStatusStored,
	}
	if err := db.Create(&file).Error; err != nil {
		t.Fatalf("create upload file failed: %v", err)
	}
	ref := model.UploadFileReference{
		FileID:     file.ID,
		UUID:       file.UUID,
		OwnerType:  "admin_user",
		OwnerID:    1,
		OwnerField: "avatar",
	}
	if err := db.Create(&ref).Error; err != nil {
		t.Fatalf("create upload file reference failed: %v", err)
	}

	err := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db}).Delete(file.ID, 1, "")
	var referencedErr *FileReferencedDeleteError
	if !stderrors.As(err, &referencedErr) {
		t.Fatalf("expected FileReferencedDeleteError, got %v", err)
	}
	businessErr := referencedErr.BusinessError()
	if businessErr == nil || businessErr.GetCode() != e.FileReferenced {
		t.Fatalf("expected FileReferenced business error, got %#v", businessErr)
	}
	if len(referencedErr.References) != 1 {
		t.Fatalf("expected one reference, got %d", len(referencedErr.References))
	}
	if referencedErr.References[0].SourceName != "管理员" || referencedErr.References[0].FieldName != "头像" {
		t.Fatalf("unexpected reference display fields: %#v", referencedErr.References[0])
	}

	var stored model.UploadFiles
	if err := db.First(&stored, file.ID).Error; err != nil {
		t.Fatalf("query upload file failed: %v", err)
	}
	if stored.DeletedAt != 0 {
		t.Fatalf("expected referenced file to remain undeleted, got deleted_at=%d", stored.DeletedAt)
	}
}

func TestFileResourceBatchDeleteReturnsStatsAndReferenceFailures(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db})

	deletable := model.UploadFiles{
		OriginName:    "deletable.txt",
		UUID:          fmt.Sprintf("%032x", time.Now().UnixNano()),
		StorageDriver: model.StorageDriverLocal,
		StorageStatus: model.StorageStatusStored,
	}
	if err := db.Create(&deletable).Error; err != nil {
		t.Fatalf("create deletable file failed: %v", err)
	}

	referenced := model.UploadFiles{
		OriginName:    "referenced.txt",
		UUID:          fmt.Sprintf("%032x", time.Now().UnixNano()+1),
		StorageDriver: model.StorageDriverLocal,
		StorageStatus: model.StorageStatusStored,
	}
	if err := db.Create(&referenced).Error; err != nil {
		t.Fatalf("create referenced file failed: %v", err)
	}
	if err := db.Create(&model.UploadFileReference{
		FileID:     referenced.ID,
		UUID:       referenced.UUID,
		OwnerType:  "admin_user",
		OwnerID:    1,
		OwnerField: "avatar",
	}).Error; err != nil {
		t.Fatalf("create referenced relation failed: %v", err)
	}

	result, err := service.BatchDelete([]uint{deletable.ID, referenced.ID}, 9, "batch")
	if err != nil {
		t.Fatalf("batch delete failed: %v", err)
	}
	if result.Total != 2 || result.Deleted != 1 || result.Failed != 1 {
		t.Fatalf("unexpected batch delete stats: %#v", result)
	}
	if len(result.Failures) != 1 || result.Failures[0].ID != referenced.ID {
		t.Fatalf("unexpected batch delete failures: %#v", result.Failures)
	}
	if result.Failures[0].Code != e.FileReferenced || len(result.Failures[0].References) != 1 {
		t.Fatalf("unexpected failure detail: %#v", result.Failures[0])
	}

	var deletableStored model.UploadFiles
	if err := db.Unscoped().First(&deletableStored, deletable.ID).Error; err != nil {
		t.Fatalf("query deletable file failed: %v", err)
	}
	if deletableStored.DeletedAt == 0 || deletableStored.DeletedBy != 9 || deletableStored.DeletedReason != "batch" {
		t.Fatalf("expected deletable file to be soft deleted, got %#v", deletableStored)
	}

	var referencedStored model.UploadFiles
	if err := db.First(&referencedStored, referenced.ID).Error; err != nil {
		t.Fatalf("query referenced file failed: %v", err)
	}
	if referencedStored.DeletedAt != 0 {
		t.Fatalf("expected referenced file to remain undeleted, got deleted_at=%d", referencedStored.DeletedAt)
	}
}

func TestFileResourceMoveFolderRejectsDescendant(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	root := model.UploadFileFolder{Name: "root", LogicalPath: "/root"}
	if err := db.Create(&root).Error; err != nil {
		t.Fatalf("create root folder failed: %v", err)
	}
	child := model.UploadFileFolder{ParentID: root.ID, Name: "child", LogicalPath: "/root/child"}
	if err := db.Create(&child).Error; err != nil {
		t.Fatalf("create child folder failed: %v", err)
	}

	_, err := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db}).MoveFolder(&form.FileFolderMove{ID: root.ID, ParentID: child.ID}, 1)
	if err == nil {
		t.Fatal("expected moving folder to descendant to fail")
	}
}

func TestFileResourceMoveFilesReturnsStatsAndUpdatesLogicalPath(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	folder := model.UploadFileFolder{Name: "docs", LogicalPath: "/docs"}
	if err := db.Create(&folder).Error; err != nil {
		t.Fatalf("create folder failed: %v", err)
	}
	file := model.UploadFiles{OriginName: "a.txt", UUID: fmt.Sprintf("%032x", time.Now().UnixNano()), LogicalPath: "/", StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored}
	if err := db.Create(&file).Error; err != nil {
		t.Fatalf("create upload file failed: %v", err)
	}

	result, err := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db}).MoveFiles(&form.FileMove{IDs: []uint{file.ID, file.ID + 1000}, FolderID: folder.ID})
	if err != nil {
		t.Fatalf("move files failed: %v", err)
	}
	if result.Total != 2 || result.Moved != 1 || result.Skipped != 1 {
		t.Fatalf("unexpected move stats: %#v", result)
	}
	var stored model.UploadFiles
	if err := db.First(&stored, file.ID).Error; err != nil {
		t.Fatalf("query moved file failed: %v", err)
	}
	if stored.FolderID != folder.ID || stored.LogicalPath != "/docs" {
		t.Fatalf("unexpected moved file folder/path: folder=%d path=%q", stored.FolderID, stored.LogicalPath)
	}
}

func TestFileResourceUpdateFileRenamesDisplayName(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	folder := model.UploadFileFolder{Name: "docs", LogicalPath: "/docs"}
	if err := db.Create(&folder).Error; err != nil {
		t.Fatalf("create folder failed: %v", err)
	}
	file := model.UploadFiles{
		FolderID:      folder.ID,
		OriginName:    "old.txt",
		DisplayName:   "old.txt",
		Name:          "physical-object-name",
		Path:          "physical/path",
		ObjectKey:     "physical/object-key",
		Ext:           "txt",
		UUID:          fmt.Sprintf("%032x", time.Now().UnixNano()),
		StorageDriver: model.StorageDriverLocal,
		StorageStatus: model.StorageStatusStored,
	}
	if err := db.Create(&file).Error; err != nil {
		t.Fatalf("create upload file failed: %v", err)
	}

	result, err := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db}).UpdateFile(&form.FileUpdate{ID: file.ID, OriginName: "new-name.pdf"})
	if err != nil {
		t.Fatalf("update file failed: %v", err)
	}
	resource, ok := result.(*resources.FileResourceResources)
	if !ok {
		t.Fatalf("expected file resource result, got %#v", result)
	}
	if resource.OriginName != "new-name.pdf" || resource.DisplayName != "new-name.pdf" || resource.Ext != "pdf" {
		t.Fatalf("unexpected renamed resource: %#v", resource)
	}

	var stored model.UploadFiles
	if err := db.First(&stored, file.ID).Error; err != nil {
		t.Fatalf("query renamed file failed: %v", err)
	}
	if stored.OriginName != "new-name.pdf" || stored.DisplayName != "new-name.pdf" || stored.Ext != "pdf" {
		t.Fatalf("unexpected renamed file: %#v", stored)
	}
	if stored.Name != "physical-object-name" || stored.Path != "physical/path" || stored.ObjectKey != "physical/object-key" {
		t.Fatalf("physical storage fields should not be renamed: %#v", stored)
	}
}

func TestFileResourceUpdateFileRejectsDuplicateName(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	folder := model.UploadFileFolder{Name: "docs", LogicalPath: "/docs"}
	if err := db.Create(&folder).Error; err != nil {
		t.Fatalf("create folder failed: %v", err)
	}
	files := []model.UploadFiles{
		{FolderID: folder.ID, OriginName: "a.txt", UUID: fmt.Sprintf("%032x", time.Now().UnixNano()), StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored},
		{FolderID: folder.ID, OriginName: "b.txt", UUID: fmt.Sprintf("%032x", time.Now().UnixNano()+1), StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored},
	}
	if err := db.Create(&files).Error; err != nil {
		t.Fatalf("create upload files failed: %v", err)
	}

	_, err := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db}).UpdateFile(&form.FileUpdate{ID: files[0].ID, OriginName: "b.txt"})
	var businessErr *e.BusinessError
	if !stderrors.As(err, &businessErr) || businessErr.GetCode() != e.FileNameExists {
		t.Fatalf("expected FileNameExists business error, got %v", err)
	}
}

func TestFileResourceUploadLocalRecordsLocationFields(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	config.Config.BasePath = t.TempDir()
	folder := model.UploadFileFolder{Name: "images", LogicalPath: "/images"}
	if err := db.Create(&folder).Error; err != nil {
		t.Fatalf("create folder failed: %v", err)
	}
	header := newMultipartFileHeader(t, "hello.txt", []byte("hello"))

	result, err := newLocalUploadTestService(db).UploadLocal([]*multipart.FileHeader{header}, &form.FileLocalUpload{FolderID: folder.ID, IsPublic: global.Yes, UploadScene: "test"}, 9)
	if err != nil {
		t.Fatalf("upload local failed: %v", err)
	}
	if result.Total != int64(1) || result.Success != int64(1) || result.Failed != int64(0) {
		t.Fatalf("unexpected upload summary: %#v", result)
	}
	var stored model.UploadFiles
	if len(result.Items) != 1 {
		t.Fatalf("expected one upload item, got %d", len(result.Items))
	}
	if err := db.First(&stored, result.Items[0].ID).Error; err != nil {
		t.Fatalf("query uploaded file failed: %v", err)
	}
	if stored.FolderID != folder.ID || stored.LogicalPath != "/images" || stored.DisplayName != "hello.txt" {
		t.Fatalf("unexpected logical fields: %#v", stored)
	}
	if stored.StorageDriver != model.StorageDriverLocal || stored.StorageBase == "" || stored.StoragePath == "" || stored.ObjectKey == "" {
		t.Fatalf("unexpected storage fields: %#v", stored)
	}
	if stored.UploadSource != model.UploadSourceBackend || stored.UploadScene != "test" || stored.UploadStatus != model.UploadStatusUploaded {
		t.Fatalf("unexpected upload fields: %#v", stored)
	}
	var object model.UploadFileObject
	if err := db.First(&object, stored.FileObjectID).Error; err != nil {
		t.Fatalf("query uploaded object failed: %v", err)
	}
	if object.OwnerUID != 0 || object.IsPublic != global.Yes {
		t.Fatalf("expected public local object owner=0/is_public=1, got %#v", object)
	}
}

func TestFileResourceUploadLocalReusesPhysicalObject(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	config.Config.BasePath = t.TempDir()
	service := newLocalUploadTestService(db)
	content := []byte("same content")

	first, err := service.UploadLocal([]*multipart.FileHeader{newMultipartFileHeader(t, "first.txt", content)}, &form.FileLocalUpload{IsPublic: global.Yes}, 1)
	if err != nil {
		t.Fatalf("first upload failed: %v", err)
	}
	second, err := service.UploadLocal([]*multipart.FileHeader{newMultipartFileHeader(t, "second.txt", content)}, &form.FileLocalUpload{IsPublic: global.Yes}, 2)
	if err != nil {
		t.Fatalf("second upload failed: %v", err)
	}
	if len(first.Items) != 1 || len(second.Items) != 1 {
		t.Fatalf("expected one item per upload, first=%d second=%d", len(first.Items), len(second.Items))
	}
	if first.Items[0].FileObjectID == 0 || first.Items[0].FileObjectID != second.Items[0].FileObjectID {
		t.Fatalf("expected same file object, first=%d second=%d", first.Items[0].FileObjectID, second.Items[0].FileObjectID)
	}
	if first.Items[0].ObjectKey != second.Items[0].ObjectKey {
		t.Fatalf("expected same object key, first=%q second=%q", first.Items[0].ObjectKey, second.Items[0].ObjectKey)
	}
	var objectCount int64
	if err := db.Model(&model.UploadFileObject{}).Count(&objectCount).Error; err != nil {
		t.Fatalf("count objects failed: %v", err)
	}
	if objectCount != 1 {
		t.Fatalf("expected one physical object, got %d", objectCount)
	}
	storedPath := filepath.Join(config.Config.BasePath, "storage/public", filepath.FromSlash(first.Items[0].ObjectKey))
	if _, err := os.Stat(storedPath); err != nil {
		t.Fatalf("expected reused physical file to exist: %v", err)
	}
}

func TestFileResourceUploadLocalReturnsPartialSuccessForMultipleFiles(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	config.Config.BasePath = t.TempDir()
	service := newLocalUploadTestService(db)

	successHeader := newMultipartFileHeader(t, "ok.txt", []byte("ok"))
	failedHeader := &multipart.FileHeader{Filename: "broken.txt"}

	result, err := service.UploadLocal([]*multipart.FileHeader{successHeader, failedHeader}, &form.FileLocalUpload{IsPublic: global.Yes, UploadScene: "test"}, 3)
	if err != nil {
		t.Fatalf("upload local should return batch result instead of error, got %v", err)
	}
	if result.Total != int64(2) || result.Success != int64(1) || result.Failed != int64(1) {
		t.Fatalf("unexpected partial upload summary: %#v", result)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected one successful item, got %d", len(result.Items))
	}
	if len(result.Failures) != 1 {
		t.Fatalf("expected one failure item, got %d", len(result.Failures))
	}
	if result.Failures[0].Index != 1 || result.Failures[0].OriginName != "broken.txt" || result.Failures[0].Message == "" {
		t.Fatalf("unexpected failure detail: %#v", result.Failures[0])
	}

	var count int64
	if err := db.Model(&model.UploadFiles{}).Count(&count).Error; err != nil {
		t.Fatalf("count uploaded files failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected only successful file to be stored, got %d", count)
	}
}

func TestFileResourceDestroyKeepsPhysicalObjectUntilLastRecord(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	basePath := t.TempDir()
	objectKey := "objects/shared.txt"
	physicalPath := filepath.Join(basePath, filepath.FromSlash(objectKey))
	if err := os.MkdirAll(filepath.Dir(physicalPath), 0o755); err != nil {
		t.Fatalf("create physical dir failed: %v", err)
	}
	if err := os.WriteFile(physicalPath, []byte("shared"), 0o644); err != nil {
		t.Fatalf("write physical file failed: %v", err)
	}
	object := model.UploadFileObject{
		StorageDriver: model.StorageDriverLocal,
		StorageBase:   basePath,
		Bucket:        "public",
		StoragePath:   objectKey,
		ObjectKey:     objectKey,
		Hash:          strings.Repeat("a", 64),
		Status:        model.StorageStatusStored,
	}
	if err := db.Create(&object).Error; err != nil {
		t.Fatalf("create object failed: %v", err)
	}
	first := model.UploadFiles{FileObjectID: object.ID, OriginName: "shared-a.txt", UUID: fmt.Sprintf("%032x", time.Now().UnixNano()), StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored}
	second := model.UploadFiles{FileObjectID: object.ID, OriginName: "shared-b.txt", UUID: fmt.Sprintf("%032x", time.Now().UnixNano()+1), StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("create first file failed: %v", err)
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("create second file failed: %v", err)
	}
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db})
	if err := service.Destroy(first.ID); err != nil {
		t.Fatalf("destroy first failed: %v", err)
	}
	if _, err := os.Stat(physicalPath); err != nil {
		t.Fatalf("expected physical file to remain after non-last destroy: %v", err)
	}
	var objectCount int64
	if err := db.Model(&model.UploadFileObject{}).Where("id = ?", object.ID).Count(&objectCount).Error; err != nil {
		t.Fatalf("count object failed: %v", err)
	}
	if objectCount != 1 {
		t.Fatalf("expected object to remain, got %d", objectCount)
	}
	if err := service.Destroy(second.ID); err != nil {
		t.Fatalf("destroy second failed: %v", err)
	}
	if _, err := os.Stat(physicalPath); !os.IsNotExist(err) {
		t.Fatalf("expected physical file removed after last destroy, err=%v", err)
	}
	if err := db.Model(&model.UploadFileObject{}).Where("id = ?", object.ID).Count(&objectCount).Error; err != nil {
		t.Fatalf("count object after destroy failed: %v", err)
	}
	if objectCount != 0 {
		t.Fatalf("expected object removed, got %d", objectCount)
	}
}

func TestFileResourceUploadCredentialDoesNotReuseUnverifiedHash(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	hash := strings.Repeat("b", 64)
	object := model.UploadFileObject{
		StorageDriver: model.StorageDriverAliyunOSS,
		Bucket:        "assets",
		StoragePath:   "uploads/existing.txt",
		ObjectKey:     "uploads/existing.txt",
		IsPublic:      global.Yes,
		Size:          12,
		Hash:          hash,
		MimeType:      "text/plain",
		ETag:          "etag-1",
		Status:        model.StorageStatusStored,
	}
	if err := db.Create(&object).Error; err != nil {
		t.Fatalf("create object failed: %v", err)
	}
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return fakeFileResourceDriver{name: model.StorageDriverAliyunOSS}, filestorage.Config{
				AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"},
			}, model.StorageDriverAliyunOSS, nil
		},
		StorageDriverResolver: func(_ context.Context, driverName string) (filestorage.Driver, filestorage.Config, error) {
			return fakeFileResourceDriver{name: driverName}, filestorage.Config{}, nil
		},
	})
	result, err := service.UploadCredential(&form.FileUploadCredential{
		OriginName: "existing.txt",
		Hash:       hash,
		MimeType:   "text/plain",
		Size:       12,
		IsPublic:   global.Yes,
	}, 1)
	if err != nil {
		t.Fatalf("upload credential failed: %v", err)
	}
	if result.Reuse || result.FileObjectID != 0 || result.ObjectKey == object.ObjectKey {
		t.Fatalf("unverified client hash must not reuse object: %#v", result)
	}
	if result.Method != "PUT" || result.Headers["x-oss-forbid-overwrite"] != "true" {
		t.Fatalf("expected non-overwriting direct upload credential: %#v", result)
	}
	if result.CompleteToken == "" || result.CompletePayload["complete_token"] != result.CompleteToken {
		t.Fatalf("expected credential to return complete_token, got %#v", result)
	}
}

func TestFileResourceUploadCredentialDoesNotReusePrivateObjectAcrossUsers(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	hash := strings.Repeat("1", 64)
	object := model.UploadFileObject{
		StorageDriver: model.StorageDriverAliyunOSS,
		Bucket:        "assets",
		StoragePath:   "uploads/private.txt",
		ObjectKey:     "uploads/private.txt",
		OwnerUID:      1,
		IsPublic:      global.No,
		Size:          12,
		Hash:          hash,
		MimeType:      "text/plain",
		Status:        model.StorageStatusStored,
	}
	if err := db.Create(&object).Error; err != nil {
		t.Fatalf("create private object failed: %v", err)
	}
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return fakeFileResourceDriver{name: model.StorageDriverAliyunOSS}, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, model.StorageDriverAliyunOSS, nil
		},
		StorageDriverResolver: func(_ context.Context, driverName string) (filestorage.Driver, filestorage.Config, error) {
			return fakeFileResourceDriver{name: driverName}, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, nil
		},
	})
	ownerCredential, err := service.UploadCredential(&form.FileUploadCredential{
		OriginName: "private.txt",
		Size:       12,
		Hash:       hash,
		MimeType:   "text/plain",
		IsPublic:   global.No,
	}, 1)
	if err != nil {
		t.Fatalf("owner credential failed: %v", err)
	}
	if ownerCredential.Reuse || ownerCredential.FileObjectID != 0 || ownerCredential.ObjectKey == object.ObjectKey {
		t.Fatalf("owner must upload before hash-based reuse, got %#v", ownerCredential)
	}
	otherCredential, err := service.UploadCredential(&form.FileUploadCredential{
		OriginName: "private.txt",
		Size:       12,
		Hash:       hash,
		MimeType:   "text/plain",
		IsPublic:   global.No,
	}, 2)
	if err != nil {
		t.Fatalf("other credential failed: %v", err)
	}
	if otherCredential.Reuse || otherCredential.FileObjectID != 0 || otherCredential.ObjectKey == object.ObjectKey {
		t.Fatalf("expected other user to receive fresh private credential, got %#v", otherCredential)
	}
}

func TestFileResourceUploadCredentialBatchReturnsPerItemResults(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	hash := strings.Repeat("b", 64)
	object := model.UploadFileObject{
		StorageDriver: model.StorageDriverAliyunOSS,
		Bucket:        "assets",
		StoragePath:   "uploads/existing.txt",
		ObjectKey:     "uploads/existing.txt",
		IsPublic:      global.Yes,
		Size:          12,
		Hash:          hash,
		MimeType:      "text/plain",
		ETag:          "etag-1",
		Status:        model.StorageStatusStored,
	}
	if err := db.Create(&object).Error; err != nil {
		t.Fatalf("create object failed: %v", err)
	}
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return fakeFileResourceDriver{name: "aliyun_oss"}, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, "aliyun_oss", nil
		},
		StorageDriverResolver: func(_ context.Context, driverName string) (filestorage.Driver, filestorage.Config, error) {
			return fakeFileResourceDriver{name: driverName}, filestorage.Config{}, nil
		},
	})

	result, err := service.UploadCredentialBatch(&form.FileUploadCredentialBatch{
		IsPublic: global.Yes,
		Items: []form.FileUploadCredentialBatchItem{
			{ClientID: "reuse", OriginName: "existing.txt", Size: 12, MimeType: "text/plain", Hash: hash},
			{ClientID: "fresh", OriginName: "fresh.txt", Size: 1, MimeType: "text/plain", Hash: strings.Repeat("c", 64)},
		},
	})
	if err != nil {
		t.Fatalf("upload credential batch failed: %v", err)
	}

	if result.Total != 2 || result.Success != 2 || result.Failed != 0 || len(result.Items) != 2 {
		t.Fatalf("unexpected batch credential stats: %#v", result)
	}
	if !result.Items[0].Success || result.Items[0].Data == nil || result.Items[0].Data.Reuse || result.Items[0].Data.FileObjectID != 0 {
		t.Fatalf("expected first credential item to require verified upload, got %#v", result.Items[0])
	}
	if !result.Items[1].Success || result.Items[1].Data == nil || result.Items[1].Data.Reuse || result.Items[1].Data.ObjectKey == "" {
		t.Fatalf("expected second credential item to return non-reuse credential payload, got %#v", result.Items[1])
	}
	if result.Items[0].Data.CompleteToken == "" || result.Items[1].Data.CompleteToken == "" {
		t.Fatalf("expected credential batch items to include complete_token, got %#v", result.Items)
	}
}

func TestFileBatchFailuresDoNotExposeTechnicalErrors(t *testing.T) {
	technicalErr := stderrors.New("dial tcp 10.0.0.8:3306: secret backend unavailable")
	wantMessage := e.NewBusinessError(e.ServerErr).GetMessage()

	uploadFailure := buildBatchFailure(2, "client-secret", technicalErr)
	if uploadFailure.Code != e.ServerErr || uploadFailure.Message != wantMessage {
		t.Fatalf("expected generic upload failure, got %#v", uploadFailure)
	}
	if strings.Contains(uploadFailure.Message, "10.0.0.8") || strings.Contains(uploadFailure.Message, "secret") {
		t.Fatalf("technical upload error leaked to client: %q", uploadFailure.Message)
	}

	localFailure := buildLocalUploadFailureItem(1, &multipart.FileHeader{Filename: "demo.txt"}, technicalErr)
	if localFailure.Code != e.ServerErr || localFailure.Message != wantMessage {
		t.Fatalf("expected generic local upload failure, got %#v", localFailure)
	}

	deleteFailure := buildFileBatchDeleteFailure(11, technicalErr)
	if deleteFailure.Code != e.ServerErr || deleteFailure.Message != wantMessage {
		t.Fatalf("expected generic delete failure, got %#v", deleteFailure)
	}

	destroyFailure := buildFileBatchDestroyFailure(12, technicalErr)
	if destroyFailure.Code != e.ServerErr || destroyFailure.Message != wantMessage {
		t.Fatalf("expected generic destroy failure, got %#v", destroyFailure)
	}
}

func TestFileBatchFailuresPreserveBusinessErrors(t *testing.T) {
	businessErr := e.NewBusinessError(e.FileTooLarge)

	uploadFailure := buildBatchFailure(0, "client-1", businessErr)
	if uploadFailure.Code != e.FileTooLarge || uploadFailure.Message != businessErr.GetMessage() {
		t.Fatalf("expected business upload failure, got %#v", uploadFailure)
	}

	deleteFailure := buildFileBatchDeleteFailure(1, businessErr)
	if deleteFailure.Code != e.FileTooLarge || deleteFailure.Message != businessErr.GetMessage() {
		t.Fatalf("expected business delete failure, got %#v", deleteFailure)
	}
}

func TestFileResourceCompleteDirectUploadRejectsMultiFileType(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	hash := strings.Repeat("f", 64)
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return fakeFileResourceDriver{name: model.StorageDriverAliyunOSS}, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, model.StorageDriverAliyunOSS, nil
		},
		StorageDriverResolver: func(_ context.Context, driverName string) (filestorage.Driver, filestorage.Config, error) {
			return fakeFileResourceDriver{name: driverName}, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, nil
		},
	})
	credential, err := service.UploadCredential(&form.FileUploadCredential{
		OriginName: "reuse.pdf",
		Size:       12,
		Hash:       hash,
		MimeType:   "application/pdf",
		IsPublic:   global.Yes,
	}, 1)
	if err != nil {
		t.Fatalf("upload credential failed: %v", err)
	}

	_, err = service.CompleteDirectUpload(&form.FileUploadComplete{
		CompleteToken: credential.CompleteToken,
		OriginName:    "reuse.pdf",
		Size:          12,
		Hash:          hash,
		MimeType:      "application/pdf",
		IsPublic:      global.Yes,
		Driver:        model.StorageDriverAliyunOSS,
		Bucket:        credential.Bucket,
		ObjectKey:     credential.ObjectKey,
		FileType:      "pdf,word",
	}, 1)
	if err == nil {
		t.Fatal("expected multi file_type direct complete to fail")
	}
}

func TestFileResourceCompleteDirectUploadRejectsMimeMismatch(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	hash := strings.Repeat("a", 64)
	driver := fakeFileResourceDriver{name: model.StorageDriverAliyunOSS, content: []byte("<html>alert(1)</html>")}
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return driver, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, model.StorageDriverAliyunOSS, nil
		},
		StorageDriverResolver: func(_ context.Context, driverName string) (filestorage.Driver, filestorage.Config, error) {
			return driver, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, nil
		},
	})
	credential, err := service.UploadCredential(&form.FileUploadCredential{
		OriginName: "avatar.jpg",
		Size:       21,
		Hash:       hash,
		MimeType:   "image/jpeg",
		IsPublic:   global.Yes,
	}, 1)
	if err != nil {
		t.Fatalf("upload credential failed: %v", err)
	}

	_, err = service.CompleteDirectUpload(&form.FileUploadComplete{
		CompleteToken: credential.CompleteToken,
		OriginName:    "avatar.jpg",
		Size:          21,
		Hash:          hash,
		MimeType:      "image/jpeg",
		IsPublic:      global.Yes,
		Driver:        model.StorageDriverAliyunOSS,
		Bucket:        credential.Bucket,
		ObjectKey:     credential.ObjectKey,
	}, 1)
	if err == nil {
		t.Fatal("expected MIME mismatch to be rejected")
	}
}

func TestFileResourceCompleteDirectUploadUsesSessionSizeForActualSizeCheck(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	hash := strings.Repeat("b", 64)
	driver := fakeStatFileResourceDriver{
		fakeFileResourceDriver: fakeFileResourceDriver{
			name:    model.StorageDriverAliyunOSS,
			content: []byte("plain text content"),
		},
		statSize: 9,
	}
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return driver, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, model.StorageDriverAliyunOSS, nil
		},
		StorageDriverResolver: func(_ context.Context, driverName string) (filestorage.Driver, filestorage.Config, error) {
			return driver, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, nil
		},
	})
	credential, err := service.UploadCredential(&form.FileUploadCredential{
		OriginName: "note.txt",
		Size:       12,
		Hash:       hash,
		MimeType:   "text/plain",
		IsPublic:   global.Yes,
	}, 1)
	if err != nil {
		t.Fatalf("upload credential failed: %v", err)
	}

	_, err = service.CompleteDirectUpload(&form.FileUploadComplete{
		CompleteToken: credential.CompleteToken,
		OriginName:    "note.txt",
		Size:          9, // 客户端伪造为实际大小；服务端必须仍按 session.Size=12 校验
		Hash:          hash,
		MimeType:      "text/plain",
		IsPublic:      global.Yes,
		Driver:        model.StorageDriverAliyunOSS,
		Bucket:        credential.Bucket,
		ObjectKey:     credential.ObjectKey,
	}, 1)
	if err == nil {
		t.Fatal("expected actual size mismatch against session size to be rejected")
	}
	var bizErr *e.BusinessError
	if !stderrors.As(err, &bizErr) || bizErr.GetCode() != e.InvalidParameter {
		t.Fatalf("expected InvalidParameter for size mismatch, got: %v", err)
	}
}

func TestFileResourceMimeSniffFailsClosedOnStorageErrors(t *testing.T) {
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		StorageDriverResolver: func(_ context.Context, driverName string) (filestorage.Driver, filestorage.Config, error) {
			return fakeFileResourceDriver{name: driverName, openErr: stderrors.New("open failed")}, filestorage.Config{}, nil
		},
	})
	if _, valid, err := service.sniffAndValidateMimeType(context.Background(), model.StorageDriverAliyunOSS, "assets", "bad.txt", "text/plain"); err == nil || valid {
		t.Fatalf("expected open error with valid=false, got valid=%v err=%v", valid, err)
	}

	service = NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		StorageDriverResolver: func(_ context.Context, driverName string) (filestorage.Driver, filestorage.Config, error) {
			return fakeFileResourceDriver{name: driverName, content: []byte("abc"), readErr: stderrors.New("read failed")}, filestorage.Config{}, nil
		},
	})
	if _, valid, err := service.sniffAndValidateMimeType(context.Background(), model.StorageDriverAliyunOSS, "assets", "bad.txt", "text/plain"); err == nil || valid {
		t.Fatalf("expected read error with valid=false, got valid=%v err=%v", valid, err)
	}
}

func TestFileResourceCompleteDirectUploadReuseUsesObjectFields(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	content := []byte("plain text content")
	object := model.UploadFileObject{
		StorageDriver: model.StorageDriverAliyunOSS,
		Bucket:        "assets",
		StoragePath:   "uploads/reuse.txt",
		ObjectKey:     "uploads/reuse.txt",
		IsPublic:      global.Yes,
		Size:          uint(len(content)),
		Hash:          testSHA256(content),
		MimeType:      "text/plain",
		ETag:          "etag-reuse",
		Status:        model.StorageStatusStored,
	}
	if err := db.Create(&object).Error; err != nil {
		t.Fatalf("create object failed: %v", err)
	}
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return fakeFileResourceDriver{name: model.StorageDriverAliyunOSS}, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, model.StorageDriverAliyunOSS, nil
		},
		StorageDriverResolver: func(_ context.Context, driverName string) (filestorage.Driver, filestorage.Config, error) {
			return fakeFileResourceDriver{name: driverName, content: content}, filestorage.Config{}, nil
		},
	})
	credential, err := service.UploadCredential(&form.FileUploadCredential{
		OriginName: "reuse.txt",
		Size:       int64(object.Size),
		Hash:       object.Hash,
		MimeType:   "text/plain",
		IsPublic:   global.Yes,
	}, 1)
	if err != nil {
		t.Fatalf("upload credential failed: %v", err)
	}

	result, err := service.CompleteDirectUpload(&form.FileUploadComplete{
		CompleteToken: credential.CompleteToken,
		OriginName:    "reuse.txt",
		Size:          object.Size,
		Hash:          object.Hash,
		MimeType:      object.MimeType,
		IsPublic:      global.Yes,
		Driver:        model.StorageDriverAliyunOSS,
		Bucket:        credential.Bucket,
		ObjectKey:     credential.ObjectKey,
	}, 1)
	if err != nil {
		t.Fatalf("direct complete reuse failed: %v", err)
	}
	if result.FileType != "text" || result.ObjectKey != object.ObjectKey || result.Size != object.Size || result.Hash != object.Hash || result.MimeType != object.MimeType {
		t.Fatalf("expected direct complete to derive fields from object, got %#v", result)
	}
}

func TestFileResourceCompleteDirectUploadRequiresValidCompleteToken(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	hash := strings.Repeat("8", 64)
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return fakeFileResourceDriver{name: model.StorageDriverAliyunOSS}, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, model.StorageDriverAliyunOSS, nil
		},
		StorageDriverResolver: func(_ context.Context, driverName string) (filestorage.Driver, filestorage.Config, error) {
			return fakeFileResourceDriver{name: driverName}, filestorage.Config{}, nil
		},
	})
	credential, err := service.UploadCredential(&form.FileUploadCredential{
		OriginName: "secure.txt",
		Size:       12,
		Hash:       hash,
		MimeType:   "text/plain",
		IsPublic:   global.Yes,
	}, 7)
	if err != nil {
		t.Fatalf("upload credential failed: %v", err)
	}
	base := form.FileUploadComplete{
		OriginName: "secure.txt",
		Size:       12,
		Hash:       hash,
		MimeType:   "text/plain",
		IsPublic:   global.Yes,
		Driver:     model.StorageDriverAliyunOSS,
		Bucket:     credential.Bucket,
		ObjectKey:  credential.ObjectKey,
	}
	if _, err := service.CompleteDirectUpload(&base, 7); err == nil {
		t.Fatal("expected missing complete_token to fail")
	}
	base.CompleteToken = "wrong-token"
	if _, err := service.CompleteDirectUpload(&base, 7); err == nil {
		t.Fatal("expected wrong complete_token to fail")
	}
	base.CompleteToken = credential.CompleteToken
	tampered := base
	tampered.Hash = strings.Repeat("9", 64)
	if _, err := service.CompleteDirectUpload(&tampered, 7); err == nil {
		t.Fatal("expected hash mismatch to fail")
	}
	tampered = base
	tampered.ObjectKey = "uploads/tampered.txt"
	if _, err := service.CompleteDirectUpload(&tampered, 7); err == nil {
		t.Fatal("expected object_key mismatch to fail")
	}
	if _, err := service.CompleteDirectUpload(&base, 8); err == nil {
		t.Fatal("expected cross-user complete_token use to fail")
	}
}

func TestFileResourceCompleteDirectUploadVerifiesHashAndIsolatesFinalObject(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	content := []byte("verified direct upload")
	driver := fakeFileResourceDriver{name: model.StorageDriverAliyunOSS, content: content}
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return driver, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, model.StorageDriverAliyunOSS, nil
		},
		StorageDriverResolver: func(context.Context, string) (filestorage.Driver, filestorage.Config, error) {
			return driver, filestorage.Config{}, nil
		},
	})
	credential, err := service.UploadCredential(&form.FileUploadCredential{
		OriginName: "verified.txt", Size: int64(len(content)), Hash: testSHA256(content), MimeType: "text/plain", IsPublic: global.Yes,
	}, 7)
	if err != nil {
		t.Fatalf("upload credential failed: %v", err)
	}
	result, err := service.CompleteDirectUpload(&form.FileUploadComplete{
		CompleteToken: credential.CompleteToken,
		OriginName:    "verified.txt",
		Size:          uint(len(content)),
		Hash:          testSHA256(content),
		MimeType:      "text/plain",
		IsPublic:      global.Yes,
		Driver:        model.StorageDriverAliyunOSS,
		Bucket:        credential.Bucket,
		ObjectKey:     credential.ObjectKey,
	}, 7)
	if err != nil {
		t.Fatalf("complete direct upload failed: %v", err)
	}
	if result.ObjectKey == credential.ObjectKey || result.Hash != testSHA256(content) {
		t.Fatalf("expected verified final object isolated from signed temp key, got %#v", result)
	}
}

func TestFileResourceCompleteDirectUploadRejectsActualHashMismatch(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	content := []byte("tampered direct upload")
	driver := fakeFileResourceDriver{name: model.StorageDriverAliyunOSS, content: content}
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return driver, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, model.StorageDriverAliyunOSS, nil
		},
		StorageDriverResolver: func(context.Context, string) (filestorage.Driver, filestorage.Config, error) {
			return driver, filestorage.Config{}, nil
		},
	})
	declaredHash := testSHA256([]byte("declared content"))
	credential, err := service.UploadCredential(&form.FileUploadCredential{
		OriginName: "tampered.txt", Size: int64(len(content)), Hash: declaredHash, MimeType: "text/plain", IsPublic: global.Yes,
	}, 7)
	if err != nil {
		t.Fatalf("upload credential failed: %v", err)
	}
	_, err = service.CompleteDirectUpload(&form.FileUploadComplete{
		CompleteToken: credential.CompleteToken, OriginName: "tampered.txt", Size: uint(len(content)), Hash: declaredHash,
		MimeType: "text/plain", IsPublic: global.Yes, Driver: model.StorageDriverAliyunOSS, Bucket: credential.Bucket, ObjectKey: credential.ObjectKey,
	}, 7)
	if err == nil {
		t.Fatal("expected actual hash mismatch to fail")
	}
	var session model.UploadSession
	if err := db.Where("complete_token_hash = ?", uploadCompleteTokenHash(credential.CompleteToken)).First(&session).Error; err != nil {
		t.Fatalf("query upload session failed: %v", err)
	}
	if session.Status != model.UploadSessionStatusExpired {
		t.Fatalf("expected rejected upload session expired, got %q", session.Status)
	}
	var count int64
	if err := db.Model(&model.UploadFiles{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("expected no file record after hash mismatch, count=%d err=%v", count, err)
	}
}

func TestFileResourceCompleteDirectUploadRestoresSessionAfterPutFailure(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	content := []byte("retryable direct upload")
	driver := fakeFileResourceDriver{name: model.StorageDriverAliyunOSS, content: content, putErr: stderrors.New("temporary put failure")}
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return driver, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, model.StorageDriverAliyunOSS, nil
		},
		StorageDriverResolver: func(context.Context, string) (filestorage.Driver, filestorage.Config, error) {
			return driver, filestorage.Config{}, nil
		},
	})
	credential, err := service.UploadCredential(&form.FileUploadCredential{
		OriginName: "retry.txt", Size: int64(len(content)), Hash: testSHA256(content), MimeType: "text/plain", IsPublic: global.Yes,
	}, 7)
	if err != nil {
		t.Fatalf("upload credential failed: %v", err)
	}
	_, err = service.CompleteDirectUpload(&form.FileUploadComplete{
		CompleteToken: credential.CompleteToken, OriginName: "retry.txt", Size: uint(len(content)), Hash: testSHA256(content),
		MimeType: "text/plain", IsPublic: global.Yes, Driver: model.StorageDriverAliyunOSS,
	}, 7)
	if err == nil {
		t.Fatal("expected temporary put failure")
	}
	var session model.UploadSession
	if err := db.Where("complete_token_hash = ?", uploadCompleteTokenHash(credential.CompleteToken)).First(&session).Error; err != nil {
		t.Fatalf("query upload session failed: %v", err)
	}
	if session.Status != model.UploadSessionStatusPending || session.ObjectKey != credential.ObjectKey {
		t.Fatalf("expected retryable pending session with original object, got status=%q object=%q", session.Status, session.ObjectKey)
	}
}

func TestFileResourceCompleteDirectUploadRevalidatesFinalObjectMime(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	malicious := []byte("<html>bad</html>")
	initialJPEG := make([]byte, len(malicious))
	copy(initialJPEG, []byte{0xff, 0xd8, 0xff, 0xe0})
	openCalls := 0
	driver := fakeFileResourceDriver{
		name: model.StorageDriverAliyunOSS,
		openFunc: func(_, _ string) ([]byte, error) {
			openCalls++
			if openCalls == 1 {
				return initialJPEG, nil
			}
			return malicious, nil
		},
	}
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return driver, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, model.StorageDriverAliyunOSS, nil
		},
		StorageDriverResolver: func(context.Context, string) (filestorage.Driver, filestorage.Config, error) {
			return driver, filestorage.Config{}, nil
		},
	})
	credential, err := service.UploadCredential(&form.FileUploadCredential{
		OriginName: "avatar.jpg", Size: int64(len(malicious)), Hash: testSHA256(malicious), MimeType: "image/jpeg", IsPublic: global.Yes,
	}, 7)
	if err != nil {
		t.Fatalf("upload credential failed: %v", err)
	}
	_, err = service.CompleteDirectUpload(&form.FileUploadComplete{
		CompleteToken: credential.CompleteToken, OriginName: "avatar.jpg", Size: uint(len(malicious)), Hash: testSHA256(malicious),
		MimeType: "image/jpeg", IsPublic: global.Yes, Driver: model.StorageDriverAliyunOSS,
	}, 7)
	if err == nil {
		t.Fatal("expected final object MIME mismatch to fail")
	}
	if openCalls < 3 {
		t.Fatalf("expected temp validation, copy, and final validation reads, got %d", openCalls)
	}
	var session model.UploadSession
	if err := db.Where("complete_token_hash = ?", uploadCompleteTokenHash(credential.CompleteToken)).First(&session).Error; err != nil {
		t.Fatalf("query upload session failed: %v", err)
	}
	if session.Status != model.UploadSessionStatusPending || session.ObjectKey != credential.ObjectKey {
		t.Fatalf("expected retryable original session after final validation failure, got status=%q object=%q", session.Status, session.ObjectKey)
	}
}

func TestFinalizeDirectUploadObjectUsesSessionObjectCAS(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	content := []byte("cas protected upload")
	driver := fakeFileResourceDriver{name: model.StorageDriverAliyunOSS, content: content}
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return driver, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, model.StorageDriverAliyunOSS, nil
		},
		StorageDriverResolver: func(context.Context, string) (filestorage.Driver, filestorage.Config, error) {
			return driver, filestorage.Config{}, nil
		},
	})
	credential, err := service.UploadCredential(&form.FileUploadCredential{
		OriginName: "cas.txt", Size: int64(len(content)), Hash: testSHA256(content), MimeType: "text/plain", IsPublic: global.Yes,
	}, 7)
	if err != nil {
		t.Fatalf("upload credential failed: %v", err)
	}
	params := &form.FileUploadComplete{
		CompleteToken: credential.CompleteToken, OriginName: "cas.txt", Size: uint(len(content)), Hash: testSHA256(content),
		MimeType: "text/plain", IsPublic: global.Yes, Driver: model.StorageDriverAliyunOSS,
	}
	target, err := service.claimDirectUploadMimeSniffTarget(context.Background(), params, 7)
	if err != nil {
		t.Fatalf("claim direct upload failed: %v", err)
	}
	first, reject, err := service.finalizeDirectUploadObject(context.Background(), db, target)
	if err != nil || reject {
		t.Fatalf("first finalize failed: reject=%v err=%v", reject, err)
	}
	if first.objectKey == target.objectKey {
		t.Fatal("expected first finalize to replace temporary object key")
	}
	if _, reject, err = service.finalizeDirectUploadObject(context.Background(), db, target); err == nil || reject {
		t.Fatalf("expected stale target CAS failure without integrity rejection, reject=%v err=%v", reject, err)
	}
	var session model.UploadSession
	if err := db.First(&session, target.sessionID).Error; err != nil {
		t.Fatalf("query upload session failed: %v", err)
	}
	if session.ObjectKey != first.objectKey || session.Status != model.UploadSessionStatusCompleting {
		t.Fatalf("stale finalizer changed winner session: status=%q object=%q winner=%q", session.Status, session.ObjectKey, first.objectKey)
	}
}

func TestFileResourceCompleteDirectUploadBatchReturnsPerItemResults(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	content := []byte("plain text content")
	object := model.UploadFileObject{
		StorageDriver: model.StorageDriverAliyunOSS,
		Bucket:        "assets",
		StoragePath:   "uploads/reuse.txt",
		ObjectKey:     "uploads/reuse.txt",
		IsPublic:      global.Yes,
		Size:          uint(len(content)),
		Hash:          testSHA256(content),
		MimeType:      "text/plain",
		ETag:          "etag-reuse",
		Status:        model.StorageStatusStored,
	}
	if err := db.Create(&object).Error; err != nil {
		t.Fatalf("create object failed: %v", err)
	}
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return fakeFileResourceDriver{name: model.StorageDriverAliyunOSS}, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, model.StorageDriverAliyunOSS, nil
		},
		StorageDriverResolver: func(_ context.Context, driverName string) (filestorage.Driver, filestorage.Config, error) {
			return fakeFileResourceDriver{name: driverName, content: content}, filestorage.Config{}, nil
		},
	})
	okCredential, err := service.UploadCredential(&form.FileUploadCredential{
		OriginName: "reuse.txt",
		Size:       int64(object.Size),
		Hash:       object.Hash,
		MimeType:   object.MimeType,
		IsPublic:   global.Yes,
	}, 1)
	if err != nil {
		t.Fatalf("create ok credential failed: %v", err)
	}
	badCredential, err := service.UploadCredential(&form.FileUploadCredential{
		OriginName: "reuse.pdf",
		Size:       int64(object.Size),
		Hash:       object.Hash,
		MimeType:   object.MimeType,
		IsPublic:   global.Yes,
	}, 1)
	if err != nil {
		t.Fatalf("create bad credential failed: %v", err)
	}

	result, err := service.CompleteDirectUploadBatch(&form.FileUploadCompleteBatch{
		IsPublic: global.Yes,
		Driver:   model.StorageDriverAliyunOSS,
		Items: []form.FileUploadCompleteBatchItem{
			{ClientID: "ok", CompleteToken: okCredential.CompleteToken, OriginName: "reuse.txt", Size: object.Size, Hash: object.Hash, MimeType: object.MimeType, Bucket: okCredential.Bucket, ObjectKey: okCredential.ObjectKey},
			{ClientID: "bad", CompleteToken: badCredential.CompleteToken, OriginName: "reuse.pdf", Size: object.Size, Hash: object.Hash, MimeType: object.MimeType, Bucket: badCredential.Bucket, ObjectKey: badCredential.ObjectKey, FileType: "pdf,word"},
		},
	}, 1)
	if err != nil {
		t.Fatalf("direct upload complete batch failed: %v", err)
	}

	if result.Total != 2 || result.Success != 1 || result.Failed != 1 || len(result.Items) != 2 {
		t.Fatalf("unexpected batch complete stats: %#v", result)
	}
	if !result.Items[0].Success || result.Items[0].Data == nil || result.Items[0].Data.FileObjectID != object.ID {
		t.Fatalf("expected first complete item to succeed, got %#v", result.Items[0])
	}
	if result.Items[1].Success || result.Items[1].Error == nil {
		t.Fatalf("expected second complete item to fail, got %#v", result.Items[1])
	}
}

func TestFileResourceListTrimsMultiFileTypes(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	rows := []model.UploadFiles{
		{OriginName: "a.pdf", UUID: fmt.Sprintf("%032x", time.Now().UnixNano()), FileType: "pdf", StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored},
		{OriginName: "b.docx", UUID: fmt.Sprintf("%032x", time.Now().UnixNano()+1), FileType: "word", StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored},
		{OriginName: "c.png", UUID: fmt.Sprintf("%032x", time.Now().UnixNano()+2), FileType: "image", StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("create upload files failed: %v", err)
	}

	result, err := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db}).List(&form.FileResourceList{
		Paginate: form.Paginate{Page: 1, PerPage: 10},
		FileType: "pdf, word",
	})
	if err != nil {
		t.Fatalf("list files failed: %v", err)
	}
	if result.Total != 2 || len(result.Data) != 2 {
		t.Fatalf("expected two pdf/word files, total=%d len=%d data=%#v", result.Total, len(result.Data), result.Data)
	}
}

func TestFileResourceListAllowsOtherFileType(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	other := model.UploadFiles{OriginName: "a.bin", UUID: fmt.Sprintf("%032x", time.Now().UnixNano()), FileType: "other", StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("create other file failed: %v", err)
	}

	result, err := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db}).List(&form.FileResourceList{
		Paginate: form.Paginate{Page: 1, PerPage: 10},
		FileType: "other",
	})
	if err != nil {
		t.Fatalf("list files failed: %v", err)
	}
	if result.Total != 1 || len(result.Data) != 1 {
		t.Fatalf("expected one other file, total=%d len=%d data=%#v", result.Total, len(result.Data), result.Data)
	}
}

func TestFileResourceMultipartInitDoesNotReuseUnverifiedHash(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	hash := strings.Repeat("c", 64)
	object := model.UploadFileObject{
		StorageDriver: model.StorageDriverAliyunOSS,
		Bucket:        "assets",
		StoragePath:   "uploads/existing.pdf",
		ObjectKey:     "uploads/existing.pdf",
		IsPublic:      global.Yes,
		Size:          32,
		Hash:          hash,
		MimeType:      "application/pdf",
		ETag:          "etag-reuse",
		Status:        model.StorageStatusStored,
	}
	if err := db.Create(&object).Error; err != nil {
		t.Fatalf("create object failed: %v", err)
	}
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return fakeFileResourceDriver{name: "aliyun_oss"}, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, "aliyun_oss", nil
		},
		StorageDriverResolver: func(_ context.Context, driverName string) (filestorage.Driver, filestorage.Config, error) {
			return fakeFileResourceDriver{name: driverName}, filestorage.Config{}, nil
		},
	})

	result, err := service.MultipartInit(&form.FileMultipartInit{
		Hash:       hash,
		OriginName: "existing.pdf",
		Size:       32,
		MimeType:   "application/pdf",
		IsPublic:   global.Yes,
		PartCount:  2,
	})
	if err != nil {
		t.Fatalf("multipart init failed: %v", err)
	}
	if result.Reuse || result.FileObjectID != 0 || result.UploadID == "" || len(result.Parts) != 2 || result.ObjectKey == object.ObjectKey {
		t.Fatalf("unverified client hash must not reuse multipart object: %#v", result)
	}
	if result.CompleteToken == "" || result.CompletePayload["complete_token"] != result.CompleteToken {
		t.Fatalf("unexpected complete payload: %#v", result.CompletePayload)
	}
}

func TestFileResourceMultipartInitLocalDriverUnsupported(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	hash := strings.Repeat("c", 64)
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return fakeFileResourceDriver{name: "local"}, filestorage.Config{}, model.StorageDriverLocal, nil
		},
	})

	_, err := service.MultipartInit(&form.FileMultipartInit{
		Hash:       hash,
		OriginName: "existing.pdf",
		Size:       32,
		MimeType:   "application/pdf",
		IsPublic:   global.Yes,
		PartCount:  2,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var be *e.BusinessError
	if !stderrors.As(err, &be) {
		t.Fatalf("expected BusinessError, got %T: %v", err, err)
	}
	if be.GetCode() != e.LocalStorageMultipartUnsupported {
		t.Fatalf("expected code %d, got %d", e.LocalStorageMultipartUnsupported, be.GetCode())
	}
}

func TestFileResourceMultipartCompleteReuseCreatesUploadRecord(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	content := []byte("plain text content")
	hash := testSHA256(content)
	object := model.UploadFileObject{
		StorageDriver: model.StorageDriverAliyunOSS,
		Bucket:        "assets",
		StoragePath:   "uploads/reuse.txt",
		ObjectKey:     "uploads/reuse.txt",
		IsPublic:      global.Yes,
		Size:          uint(len(content)),
		Hash:          hash,
		MimeType:      "text/plain",
		ETag:          "etag-reuse",
		Status:        model.StorageStatusStored,
	}
	if err := db.Create(&object).Error; err != nil {
		t.Fatalf("create object failed: %v", err)
	}
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return fakeFileResourceDriver{name: model.StorageDriverAliyunOSS}, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, model.StorageDriverAliyunOSS, nil
		},
		StorageDriverResolver: func(_ context.Context, driverName string) (filestorage.Driver, filestorage.Config, error) {
			return fakeFileResourceDriver{name: driverName, content: content}, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, nil
		},
	})
	initResult, err := service.MultipartInit(&form.FileMultipartInit{
		Hash:       hash,
		OriginName: "reuse.txt",
		Size:       int64(len(content)),
		MimeType:   "text/plain",
		IsPublic:   global.Yes,
		PartCount:  2,
	}, 8)
	if err != nil {
		t.Fatalf("multipart init failed: %v", err)
	}

	result, err := service.MultipartComplete(&form.FileMultipartComplete{
		CompleteToken: initResult.CompleteToken,
		OriginName:    "reuse.txt",
		Size:          uint(len(content)),
		Hash:          hash,
		MimeType:      "text/plain",
		IsPublic:      global.Yes,
		Bucket:        initResult.Bucket,
		ObjectKey:     initResult.ObjectKey,
		UploadID:      initResult.UploadID,
		Parts:         []form.MultipartCompletePart{{PartNumber: 1, ETag: "etag"}, {PartNumber: 2, ETag: "etag-2"}},
	}, 8)
	if err != nil {
		t.Fatalf("multipart complete reuse failed: %v", err)
	}
	if result.FileObjectID != object.ID || result.FileType != "text" || result.ObjectKey != object.ObjectKey {
		t.Fatalf("unexpected reuse complete result: %#v", result)
	}
}

func TestFileResourceMultipartCompleteRequiresValidCompleteToken(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	hash := strings.Repeat("f", 64)
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return fakeFileResourceDriver{name: model.StorageDriverAliyunOSS}, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, model.StorageDriverAliyunOSS, nil
		},
		StorageDriverResolver: func(_ context.Context, driverName string) (filestorage.Driver, filestorage.Config, error) {
			return fakeFileResourceDriver{name: driverName}, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, nil
		},
	})
	initResult, err := service.MultipartInit(&form.FileMultipartInit{
		Hash:       hash,
		OriginName: "secure.txt",
		Size:       21,
		MimeType:   "text/plain",
		IsPublic:   global.Yes,
		PartCount:  1,
	}, 8)
	if err != nil {
		t.Fatalf("multipart init failed: %v", err)
	}
	base := form.FileMultipartComplete{
		UploadID:   initResult.UploadID,
		Bucket:     initResult.Bucket,
		ObjectKey:  initResult.ObjectKey,
		OriginName: "secure.txt",
		Size:       21,
		Hash:       hash,
		MimeType:   "text/plain",
		Driver:     model.StorageDriverAliyunOSS,
		IsPublic:   global.Yes,
		Parts:      []form.MultipartCompletePart{{PartNumber: 1, ETag: "etag"}},
	}
	if _, err := service.MultipartComplete(&base, 8); err == nil {
		t.Fatal("expected missing complete_token to fail")
	}
	base.CompleteToken = "wrong-token"
	if _, err := service.MultipartComplete(&base, 8); err == nil {
		t.Fatal("expected wrong complete_token to fail")
	}
	base.CompleteToken = initResult.CompleteToken
	if _, err := service.MultipartComplete(&base, 9); err == nil {
		t.Fatal("expected cross-user complete_token use to fail")
	}
}

func TestFileResourceMultipartCompleteDoesNotCompleteStorageTwiceForSameToken(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	content := []byte("plain text content")
	hash := testSHA256(content)
	var completeCalls int
	driver := fakeFileResourceDriver{
		name:    model.StorageDriverAliyunOSS,
		content: content,
		onCompleteMultipart: func() {
			completeCalls++
		},
	}
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return driver, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, model.StorageDriverAliyunOSS, nil
		},
		StorageDriverResolver: func(_ context.Context, driverName string) (filestorage.Driver, filestorage.Config, error) {
			return driver, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, nil
		},
	})
	initResult, err := service.MultipartInit(&form.FileMultipartInit{
		Hash:       hash,
		OriginName: "secure.txt",
		Size:       21,
		MimeType:   "text/plain",
		IsPublic:   global.Yes,
		PartCount:  1,
	}, 8)
	if err != nil {
		t.Fatalf("multipart init failed: %v", err)
	}
	params := &form.FileMultipartComplete{
		CompleteToken: initResult.CompleteToken,
		UploadID:      initResult.UploadID,
		Bucket:        initResult.Bucket,
		ObjectKey:     initResult.ObjectKey,
		OriginName:    "secure.txt",
		Size:          21,
		Hash:          hash,
		MimeType:      "text/plain",
		Driver:        model.StorageDriverAliyunOSS,
		IsPublic:      global.Yes,
		Parts:         []form.MultipartCompletePart{{PartNumber: 1, ETag: "etag"}},
	}
	if _, err := service.MultipartComplete(params, 8); err != nil {
		t.Fatalf("first multipart complete failed: %v", err)
	}
	if _, err := service.MultipartComplete(params, 8); err == nil {
		t.Fatal("expected reused complete_token to fail")
	}
	if completeCalls != 1 {
		t.Fatalf("expected storage complete once, got %d", completeCalls)
	}
}

func TestFileResourceMultipartAbortUsesOwnedSessionMetadata(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	var abortedBucket, abortedKey, abortedID string
	driver := fakeFileResourceDriver{
		name: model.StorageDriverAliyunOSS,
		onAbortMultipart: func(bucket, objectKey, uploadID string) {
			abortedBucket, abortedKey, abortedID = bucket, objectKey, uploadID
		},
	}
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		StorageDriverResolver: func(context.Context, string) (filestorage.Driver, filestorage.Config, error) {
			return driver, filestorage.Config{}, nil
		},
	})
	token, _, err := createUploadSession(db, uploadSessionInput{
		UploadType: model.UploadSessionTypeMultipart, StorageDriver: model.StorageDriverAliyunOSS, Bucket: "trusted-bucket",
		ObjectKey: "trusted/object.txt", UploadID: "trusted-upload", OwnerUID: 9, OriginName: "object.txt", Size: 1,
		Hash: testSHA256([]byte("x")), MimeType: "text/plain", ExpiresAt: time.Now().Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("create upload session failed: %v", err)
	}
	if err := service.MultipartAbort(context.Background(), &form.FileMultipartAbort{CompleteToken: token}, 8); err == nil {
		t.Fatal("expected cross-user abort to fail")
	}
	if err := service.MultipartAbort(context.Background(), &form.FileMultipartAbort{CompleteToken: token}, 9); err != nil {
		t.Fatalf("owned multipart abort failed: %v", err)
	}
	if abortedBucket != "trusted-bucket" || abortedKey != "trusted/object.txt" || abortedID != "trusted-upload" {
		t.Fatalf("abort did not use trusted session metadata: %q %q %q", abortedBucket, abortedKey, abortedID)
	}
	var session model.UploadSession
	if err := db.Where("complete_token_hash = ?", uploadCompleteTokenHash(token)).First(&session).Error; err != nil {
		t.Fatalf("query upload session failed: %v", err)
	}
	if session.Status != model.UploadSessionStatusExpired {
		t.Fatalf("expected aborted session expired, got %q", session.Status)
	}
}

func TestFileResourceMultipartCompleteRejectsActualHashMismatch(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	content := []byte("tampered multipart content")
	driver := fakeFileResourceDriver{name: model.StorageDriverAliyunOSS, content: content}
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return driver, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, model.StorageDriverAliyunOSS, nil
		},
		StorageDriverResolver: func(context.Context, string) (filestorage.Driver, filestorage.Config, error) {
			return driver, filestorage.Config{}, nil
		},
	})
	declaredHash := testSHA256([]byte("declared multipart content"))
	initResult, err := service.MultipartInit(&form.FileMultipartInit{
		Hash: declaredHash, OriginName: "tampered.txt", Size: int64(len(content)), MimeType: "text/plain", IsPublic: global.Yes, PartCount: 1,
	}, 9)
	if err != nil {
		t.Fatalf("multipart init failed: %v", err)
	}
	_, err = service.MultipartComplete(&form.FileMultipartComplete{
		CompleteToken: initResult.CompleteToken, UploadID: initResult.UploadID, Bucket: initResult.Bucket, ObjectKey: initResult.ObjectKey,
		OriginName: "tampered.txt", Size: uint(len(content)), Hash: declaredHash, MimeType: "text/plain", IsPublic: global.Yes,
		Driver: model.StorageDriverAliyunOSS, Parts: []form.MultipartCompletePart{{PartNumber: 1, ETag: "etag"}},
	}, 9)
	if err == nil {
		t.Fatal("expected actual multipart hash mismatch to fail")
	}
	var session model.UploadSession
	if err := db.Where("complete_token_hash = ?", uploadCompleteTokenHash(initResult.CompleteToken)).First(&session).Error; err != nil {
		t.Fatalf("query upload session failed: %v", err)
	}
	if session.Status != model.UploadSessionStatusExpired {
		t.Fatalf("expected rejected multipart session expired, got %q", session.Status)
	}
}

func TestFileResourceMultipartCompleteUsesSessionSizeForActualSizeCheck(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	hash := strings.Repeat("d", 64)
	driver := fakeStatFileResourceDriver{
		fakeFileResourceDriver: fakeFileResourceDriver{
			name:                model.StorageDriverAliyunOSS,
			content:             []byte("plain text content"),
			onCompleteMultipart: func() {},
		},
		statSize: 9,
	}
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return driver, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, model.StorageDriverAliyunOSS, nil
		},
		StorageDriverResolver: func(_ context.Context, driverName string) (filestorage.Driver, filestorage.Config, error) {
			return driver, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, nil
		},
	})
	initResult, err := service.MultipartInit(&form.FileMultipartInit{
		OriginName: "note.txt",
		Size:       12,
		MimeType:   "text/plain",
		Hash:       hash,
		IsPublic:   global.Yes,
	}, 8)
	if err != nil {
		t.Fatalf("multipart init failed: %v", err)
	}

	_, err = service.MultipartComplete(&form.FileMultipartComplete{
		CompleteToken: initResult.CompleteToken,
		UploadID:      initResult.UploadID,
		OriginName:    "note.txt",
		Size:          9, // 客户端伪造为实际大小；服务端必须仍按 session.Size=12 校验
		Hash:          hash,
		MimeType:      "text/plain",
		IsPublic:      global.Yes,
		Driver:        model.StorageDriverAliyunOSS,
		Bucket:        initResult.Bucket,
		ObjectKey:     initResult.ObjectKey,
		Parts:         []form.MultipartCompletePart{{PartNumber: 1, ETag: "etag"}},
	}, 8)
	if err == nil {
		t.Fatal("expected multipart actual size mismatch against session size to be rejected")
	}
	var bizErr *e.BusinessError
	if !stderrors.As(err, &bizErr) || bizErr.GetCode() != e.InvalidParameter {
		t.Fatalf("expected InvalidParameter for size mismatch, got: %v", err)
	}
}

func TestFileResourceMultipartCompleteRestoresPendingWhenStorageCompleteFails(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	hash := strings.Repeat("a", 64)
	driver := fakeFileResourceDriver{
		name:        model.StorageDriverAliyunOSS,
		content:     []byte("plain text content"),
		completeErr: stderrors.New("storage complete failed"),
	}
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return driver, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, model.StorageDriverAliyunOSS, nil
		},
		StorageDriverResolver: func(_ context.Context, driverName string) (filestorage.Driver, filestorage.Config, error) {
			return driver, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, nil
		},
	})
	initResult, err := service.MultipartInit(&form.FileMultipartInit{
		Hash:       hash,
		OriginName: "retry.txt",
		Size:       21,
		MimeType:   "text/plain",
		IsPublic:   global.Yes,
		PartCount:  1,
	}, 8)
	if err != nil {
		t.Fatalf("multipart init failed: %v", err)
	}

	_, err = service.MultipartComplete(&form.FileMultipartComplete{
		CompleteToken: initResult.CompleteToken,
		UploadID:      initResult.UploadID,
		Bucket:        initResult.Bucket,
		ObjectKey:     initResult.ObjectKey,
		OriginName:    "retry.txt",
		Size:          21,
		Hash:          hash,
		MimeType:      "text/plain",
		Driver:        model.StorageDriverAliyunOSS,
		IsPublic:      global.Yes,
		Parts:         []form.MultipartCompletePart{{PartNumber: 1, ETag: "etag"}},
	}, 8)
	if err == nil {
		t.Fatal("expected storage complete error")
	}

	var session model.UploadSession
	if err := db.Where("complete_token_hash = ?", uploadCompleteTokenHash(initResult.CompleteToken)).First(&session).Error; err != nil {
		t.Fatalf("query upload session failed: %v", err)
	}
	if session.Status != model.UploadSessionStatusPending {
		t.Fatalf("expected upload session restored to pending, got %q", session.Status)
	}
}

func TestFileResourceMultipartCompleteRejectsMultiFileType(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	content := []byte("plain text content")
	object := model.UploadFileObject{
		StorageDriver: model.StorageDriverAliyunOSS,
		Bucket:        "assets",
		StoragePath:   "uploads/reuse.pdf",
		ObjectKey:     "uploads/reuse.pdf",
		IsPublic:      global.Yes,
		Size:          uint(len(content)),
		Hash:          testSHA256(content),
		MimeType:      "application/pdf",
		Status:        model.StorageStatusStored,
	}
	if err := db.Create(&object).Error; err != nil {
		t.Fatalf("create object failed: %v", err)
	}
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return fakeFileResourceDriver{name: model.StorageDriverAliyunOSS}, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, model.StorageDriverAliyunOSS, nil
		},
		StorageDriverResolver: func(_ context.Context, driverName string) (filestorage.Driver, filestorage.Config, error) {
			return fakeFileResourceDriver{name: driverName, content: content}, filestorage.Config{}, nil
		},
	})
	initResult, err := service.MultipartInit(&form.FileMultipartInit{
		Hash:       object.Hash,
		OriginName: "reuse.pdf",
		Size:       int64(len(content)),
		MimeType:   "application/pdf",
		IsPublic:   global.Yes,
		PartCount:  1,
	}, 8)
	if err != nil {
		t.Fatalf("multipart init failed: %v", err)
	}

	_, err = service.MultipartComplete(&form.FileMultipartComplete{
		CompleteToken: initResult.CompleteToken,
		OriginName:    "reuse.pdf",
		Size:          uint(len(content)),
		Hash:          object.Hash,
		MimeType:      "application/pdf",
		IsPublic:      global.Yes,
		Bucket:        initResult.Bucket,
		ObjectKey:     initResult.ObjectKey,
		UploadID:      initResult.UploadID,
		Parts:         []form.MultipartCompletePart{{PartNumber: 1, ETag: "etag"}},
		FileType:      "pdf,word",
	}, 8)
	if err == nil {
		t.Fatal("expected multipart complete multi file_type to fail")
	}
}

func TestFileResourceMultipartCompleteRejectsInvalidParts(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db})
	token, _, err := createUploadSession(db, uploadSessionInput{
		UploadType:    model.UploadSessionTypeMultipart,
		StorageDriver: model.StorageDriverAliyunOSS,
		Bucket:        "assets",
		ObjectKey:     "uploads/a.txt",
		UploadID:      "upload-1",
		OwnerUID:      1,
		OriginName:    "a.txt",
		Size:          12,
		Hash:          strings.Repeat("e", 64),
		MimeType:      "text/plain",
		IsPublic:      global.Yes,
		ExpiresAt:     time.Now().Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("create upload session failed: %v", err)
	}
	base := form.FileMultipartComplete{
		CompleteToken: token,
		UploadID:      "upload-1",
		Bucket:        "assets",
		ObjectKey:     "uploads/a.txt",
		OriginName:    "a.txt",
		Size:          12,
		Hash:          strings.Repeat("e", 64),
		MimeType:      "text/plain",
		IsPublic:      global.Yes,
	}
	cases := []struct {
		name  string
		parts []form.MultipartCompletePart
	}{
		{name: "empty", parts: nil},
		{name: "blank etag", parts: []form.MultipartCompletePart{{PartNumber: 1, ETag: " "}}},
		{name: "duplicate", parts: []form.MultipartCompletePart{{PartNumber: 1, ETag: "a"}, {PartNumber: 1, ETag: "b"}}},
		{name: "not continuous", parts: []form.MultipartCompletePart{{PartNumber: 2, ETag: "b"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params := base
			params.Parts = tc.parts
			if _, err := service.MultipartComplete(&params, 1); err == nil {
				t.Fatal("expected invalid multipart parts to fail")
			}
		})
	}
}

func TestFileResourceDeletePhysicalUsesFileStorageDriver(t *testing.T) {
	calls := make([]string, 0, 1)
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		StorageDriverResolver: func(_ context.Context, driverName string) (filestorage.Driver, filestorage.Config, error) {
			calls = append(calls, driverName)
			return fakeFileResourceDriver{name: driverName}, filestorage.Config{}, nil
		},
	})
	err := service.deletePhysicalFile(&model.UploadFiles{
		Bucket:    "bucket",
		ObjectKey: "old/object.txt",
	})
	if err != nil {
		t.Fatalf("delete physical file failed: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected resolver to use file storage driver, got %#v", calls)
	}
}

func TestFileResourceListFolderIDNilReturnsAllFiles(t *testing.T) {
	db := newFileResourceSQLiteDB(t)

	root := model.UploadFileFolder{Name: "root", LogicalPath: "/root"}
	if err := db.Create(&root).Error; err != nil {
		t.Fatalf("create root folder failed: %v", err)
	}
	child := model.UploadFileFolder{ParentID: root.ID, Name: "child", LogicalPath: "/root/child"}
	if err := db.Create(&child).Error; err != nil {
		t.Fatalf("create child folder failed: %v", err)
	}

	rows := []model.UploadFiles{
		{OriginName: "root.txt", UUID: fmt.Sprintf("%032x", time.Now().UnixNano()), FolderID: 0, StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored},
		{OriginName: "logo.png", UUID: fmt.Sprintf("%032x", time.Now().UnixNano()+1), FolderID: root.ID, StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored},
		{OriginName: "leaf.txt", UUID: fmt.Sprintf("%032x", time.Now().UnixNano()+2), FolderID: child.ID, StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("create upload files failed: %v", err)
	}

	result, err := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db}).List(&form.FileResourceList{
		Paginate: form.Paginate{Page: 1, PerPage: 10},
	})
	if err != nil {
		t.Fatalf("list files failed: %v", err)
	}
	if result.Total != 3 || len(result.Data) != 3 {
		t.Fatalf("expected nil FolderID to return all files, total=%d len=%d", result.Total, len(result.Data))
	}
}

func TestFileResourceListFolderIDZeroReturnsOnlyRootFiles(t *testing.T) {
	db := newFileResourceSQLiteDB(t)

	folder := model.UploadFileFolder{Name: "images", LogicalPath: "/images"}
	if err := db.Create(&folder).Error; err != nil {
		t.Fatalf("create folder failed: %v", err)
	}

	rootFile := model.UploadFiles{OriginName: "root.txt", UUID: fmt.Sprintf("%032x", time.Now().UnixNano()), FolderID: 0, StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored}
	logo := model.UploadFiles{OriginName: "logo.png", UUID: fmt.Sprintf("%032x", time.Now().UnixNano()+1), FolderID: folder.ID, StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored}
	if err := db.Create(&rootFile).Error; err != nil {
		t.Fatalf("create root file failed: %v", err)
	}
	if err := db.Create(&logo).Error; err != nil {
		t.Fatalf("create folder file failed: %v", err)
	}

	zero := uint(0)
	result, err := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db}).List(&form.FileResourceList{
		Paginate:         form.Paginate{Page: 1, PerPage: 10},
		FolderID:         &zero,
		IncludeSubfolder: global.No,
	})
	if err != nil {
		t.Fatalf("list files failed: %v", err)
	}
	if result.Total != 1 || len(result.Data) != 1 {
		t.Fatalf("expected only root files, total=%d len=%d", result.Total, len(result.Data))
	}
	for _, item := range result.Data {
		row, ok := item.(*resources.FileResourceResources)
		if !ok {
			t.Fatalf("unexpected item type: %T", item)
		}
		if row.OriginName == "logo.png" {
			t.Fatalf("expected logo.png to be filtered out, got %#v", row)
		}
	}
}

func TestFileResourceListFolderIDExactMatchExcludesSubfolders(t *testing.T) {
	db := newFileResourceSQLiteDB(t)

	parent := model.UploadFileFolder{Name: "parent", LogicalPath: "/parent"}
	if err := db.Create(&parent).Error; err != nil {
		t.Fatalf("create parent folder failed: %v", err)
	}
	child := model.UploadFileFolder{ParentID: parent.ID, Name: "child", LogicalPath: "/parent/child"}
	if err := db.Create(&child).Error; err != nil {
		t.Fatalf("create child folder failed: %v", err)
	}

	parentFile := model.UploadFiles{OriginName: "parent.txt", UUID: fmt.Sprintf("%032x", time.Now().UnixNano()), FolderID: parent.ID, StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored}
	childFile := model.UploadFiles{OriginName: "child.txt", UUID: fmt.Sprintf("%032x", time.Now().UnixNano()+1), FolderID: child.ID, StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored}
	rootFile := model.UploadFiles{OriginName: "root.txt", UUID: fmt.Sprintf("%032x", time.Now().UnixNano()+2), FolderID: 0, StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored}
	if err := db.Create(&parentFile).Error; err != nil {
		t.Fatalf("create parent file failed: %v", err)
	}
	if err := db.Create(&childFile).Error; err != nil {
		t.Fatalf("create child file failed: %v", err)
	}
	if err := db.Create(&rootFile).Error; err != nil {
		t.Fatalf("create root file failed: %v", err)
	}

	folderID := parent.ID
	result, err := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db}).List(&form.FileResourceList{
		Paginate:         form.Paginate{Page: 1, PerPage: 10},
		FolderID:         &folderID,
		IncludeSubfolder: global.No,
	})
	if err != nil {
		t.Fatalf("list files failed: %v", err)
	}
	if result.Total != 1 || len(result.Data) != 1 {
		t.Fatalf("expected exact-match to return only parent's own files, total=%d len=%d", result.Total, len(result.Data))
	}
	row, ok := result.Data[0].(*resources.FileResourceResources)
	if !ok {
		t.Fatalf("unexpected item type: %T", result.Data[0])
	}
	if row.OriginName != "parent.txt" {
		t.Fatalf("expected parent.txt, got %q", row.OriginName)
	}
}

func TestFileResourceListFolderIDIncludeSubfolderJoinsDescendants(t *testing.T) {
	db := newFileResourceSQLiteDB(t)

	parent := model.UploadFileFolder{Name: "parent", LogicalPath: "/parent"}
	if err := db.Create(&parent).Error; err != nil {
		t.Fatalf("create parent folder failed: %v", err)
	}
	child := model.UploadFileFolder{ParentID: parent.ID, Name: "child", LogicalPath: "/parent/child"}
	if err := db.Create(&child).Error; err != nil {
		t.Fatalf("create child folder failed: %v", err)
	}
	grandChild := model.UploadFileFolder{ParentID: child.ID, Name: "grand", LogicalPath: "/parent/child/grand"}
	if err := db.Create(&grandChild).Error; err != nil {
		t.Fatalf("create grand child folder failed: %v", err)
	}
	other := model.UploadFileFolder{Name: "other", LogicalPath: "/other"}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("create other folder failed: %v", err)
	}

	parentFile := model.UploadFiles{OriginName: "parent.txt", UUID: fmt.Sprintf("%032x", time.Now().UnixNano()), FolderID: parent.ID, StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored}
	childFile := model.UploadFiles{OriginName: "child.txt", UUID: fmt.Sprintf("%032x", time.Now().UnixNano()+1), FolderID: child.ID, StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored}
	grandFile := model.UploadFiles{OriginName: "grand.txt", UUID: fmt.Sprintf("%032x", time.Now().UnixNano()+2), FolderID: grandChild.ID, StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored}
	otherFile := model.UploadFiles{OriginName: "other.txt", UUID: fmt.Sprintf("%032x", time.Now().UnixNano()+3), FolderID: other.ID, StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored}
	rootFile := model.UploadFiles{OriginName: "root.txt", UUID: fmt.Sprintf("%032x", time.Now().UnixNano()+4), FolderID: 0, StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored}
	if err := db.Create(&parentFile).Error; err != nil {
		t.Fatalf("create parent file failed: %v", err)
	}
	if err := db.Create(&childFile).Error; err != nil {
		t.Fatalf("create child file failed: %v", err)
	}
	if err := db.Create(&grandFile).Error; err != nil {
		t.Fatalf("create grand file failed: %v", err)
	}
	if err := db.Create(&otherFile).Error; err != nil {
		t.Fatalf("create other file failed: %v", err)
	}
	if err := db.Create(&rootFile).Error; err != nil {
		t.Fatalf("create root file failed: %v", err)
	}

	folderID := parent.ID
	result, err := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db}).List(&form.FileResourceList{
		Paginate:         form.Paginate{Page: 1, PerPage: 10},
		FolderID:         &folderID,
		IncludeSubfolder: global.Yes,
	})
	if err != nil {
		t.Fatalf("list files failed: %v", err)
	}
	if result.Total != 3 || len(result.Data) != 3 {
		t.Fatalf("expected include-subfolder to return parent+child+grand files, total=%d len=%d", result.Total, len(result.Data))
	}
	wanted := map[string]bool{"parent.txt": false, "child.txt": false, "grand.txt": false}
	for _, item := range result.Data {
		row, ok := item.(*resources.FileResourceResources)
		if !ok {
			t.Fatalf("unexpected item type: %T", item)
		}
		if _, expected := wanted[row.OriginName]; !expected {
			t.Fatalf("unexpected file in result: %q", row.OriginName)
		}
		wanted[row.OriginName] = true
	}
	for name, hit := range wanted {
		if !hit {
			t.Fatalf("expected file %q to be present in result", name)
		}
	}
}

func TestFolderStatsCountsDescendants(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db})

	parent := model.UploadFileFolder{Name: "parent", LogicalPath: "/parent"}
	if err := db.Create(&parent).Error; err != nil {
		t.Fatalf("create parent folder failed: %v", err)
	}
	child := model.UploadFileFolder{ParentID: parent.ID, Name: "child", LogicalPath: "/parent/child"}
	if err := db.Create(&child).Error; err != nil {
		t.Fatalf("create child folder failed: %v", err)
	}
	grandChild := model.UploadFileFolder{ParentID: child.ID, Name: "grand", LogicalPath: "/parent/child/grand"}
	if err := db.Create(&grandChild).Error; err != nil {
		t.Fatalf("create grand child folder failed: %v", err)
	}

	files := []model.UploadFiles{
		{OriginName: "parent.txt", UUID: fmt.Sprintf("%032x", time.Now().UnixNano()), FolderID: parent.ID, Size: 100, StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored},
		{OriginName: "child.txt", UUID: fmt.Sprintf("%032x", time.Now().UnixNano()+1), FolderID: child.ID, Size: 200, StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored},
		{OriginName: "grand.txt", UUID: fmt.Sprintf("%032x", time.Now().UnixNano()+2), FolderID: grandChild.ID, Size: 300, StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored},
		{
			OriginName:    "deleted.txt",
			UUID:          fmt.Sprintf("%032x", time.Now().UnixNano()+3),
			FolderID:      child.ID,
			Size:          400,
			StorageDriver: model.StorageDriverLocal,
			StorageStatus: model.StorageStatusStored,
			ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{
				DeletedAt: soft_delete.DeletedAt(time.Now().Unix()),
			},
		},
	}
	if err := db.Create(&files).Error; err != nil {
		t.Fatalf("create upload files failed: %v", err)
	}

	stats, err := service.FolderStats(parent.ID)
	if err != nil {
		t.Fatalf("FolderStats failed: %v", err)
	}
	if stats.FileCount != 3 || stats.ChildFolderCount != 2 || stats.TotalSize != 600 {
		t.Fatalf("expected recursive file_count=3 child_folder_count=2, got %#v", stats)
	}
}

func TestDeleteFolderAndRestoreFileRestoresParentFolders(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db})

	parent := model.UploadFileFolder{Name: "parent", LogicalPath: "/parent"}
	if err := db.Create(&parent).Error; err != nil {
		t.Fatalf("create parent folder failed: %v", err)
	}
	child := model.UploadFileFolder{ParentID: parent.ID, Name: "child", LogicalPath: "/parent/child"}
	if err := db.Create(&child).Error; err != nil {
		t.Fatalf("create child folder failed: %v", err)
	}
	file := model.UploadFiles{
		OriginName:    "avatar.png",
		UUID:          fmt.Sprintf("%032x", time.Now().UnixNano()),
		FolderID:      child.ID,
		StorageDriver: model.StorageDriverLocal,
		StorageStatus: model.StorageStatusStored,
	}
	if err := db.Create(&file).Error; err != nil {
		t.Fatalf("create upload file failed: %v", err)
	}

	if err := service.DeleteFolder(parent.ID, 1, "test delete"); err != nil {
		t.Fatalf("DeleteFolder failed: %v", err)
	}
	var deletedParent, deletedChild model.UploadFileFolder
	if err := db.Unscoped().First(&deletedParent, parent.ID).Error; err != nil {
		t.Fatalf("query deleted parent failed: %v", err)
	}
	if err := db.Unscoped().First(&deletedChild, child.ID).Error; err != nil {
		t.Fatalf("query deleted child failed: %v", err)
	}
	if deletedParent.DeletedAt == 0 || deletedChild.DeletedAt == 0 {
		t.Fatalf("expected parent and child folders to be soft deleted, parent=%d child=%d", deletedParent.DeletedAt, deletedChild.DeletedAt)
	}
	var deletedFile model.UploadFiles
	if err := db.Unscoped().First(&deletedFile, file.ID).Error; err != nil {
		t.Fatalf("query deleted file failed: %v", err)
	}
	if deletedFile.DeletedAt == 0 {
		t.Fatal("expected file to be soft deleted")
	}
	if deletedFile.DeletedBy != 1 || deletedFile.DeletedReason != "test delete" {
		t.Fatalf("expected folder delete to write file audit fields, got %#v", deletedFile)
	}

	if err := service.Restore(file.ID); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}
	var restoredParent, restoredChild model.UploadFileFolder
	if err := db.First(&restoredParent, parent.ID).Error; err != nil {
		t.Fatalf("expected parent folder restored: %v", err)
	}
	if err := db.First(&restoredChild, child.ID).Error; err != nil {
		t.Fatalf("expected child folder restored: %v", err)
	}
	var restoredFile model.UploadFiles
	if err := db.First(&restoredFile, file.ID).Error; err != nil {
		t.Fatalf("expected file restored: %v", err)
	}
	if restoredFile.DeletedAt != 0 || restoredFile.FolderID != child.ID {
		t.Fatalf("unexpected restored file: %#v", restoredFile)
	}
}

func TestDeleteFolderRejectsReferencedDescendantFile(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db})

	parent := model.UploadFileFolder{Name: "parent", LogicalPath: "/parent"}
	if err := db.Create(&parent).Error; err != nil {
		t.Fatalf("create parent folder failed: %v", err)
	}
	child := model.UploadFileFolder{ParentID: parent.ID, Name: "child", LogicalPath: "/parent/child"}
	if err := db.Create(&child).Error; err != nil {
		t.Fatalf("create child folder failed: %v", err)
	}
	file := model.UploadFiles{
		OriginName:    "avatar.png",
		UUID:          fmt.Sprintf("%032x", time.Now().UnixNano()),
		FolderID:      child.ID,
		StorageDriver: model.StorageDriverLocal,
		StorageStatus: model.StorageStatusStored,
	}
	if err := db.Create(&file).Error; err != nil {
		t.Fatalf("create upload file failed: %v", err)
	}
	if err := db.Create(&model.UploadFileReference{
		FileID:     file.ID,
		UUID:       file.UUID,
		OwnerType:  "admin_user",
		OwnerID:    1,
		OwnerField: "avatar",
	}).Error; err != nil {
		t.Fatalf("create file reference failed: %v", err)
	}

	err := service.DeleteFolder(parent.ID, 9, "folder delete")
	var referencedErr *FileReferencedDeleteError
	if !stderrors.As(err, &referencedErr) {
		t.Fatalf("expected FileReferencedDeleteError, got %v", err)
	}
	if len(referencedErr.References) != 1 {
		t.Fatalf("expected one reference detail, got %#v", referencedErr.References)
	}
	var storedFile model.UploadFiles
	if err := db.First(&storedFile, file.ID).Error; err != nil {
		t.Fatalf("expected referenced file to remain active: %v", err)
	}
	if storedFile.DeletedAt != 0 || storedFile.DeletedBy != 0 || storedFile.DeletedReason != "" {
		t.Fatalf("expected referenced file untouched, got %#v", storedFile)
	}
	var storedParent model.UploadFileFolder
	if err := db.First(&storedParent, parent.ID).Error; err != nil {
		t.Fatalf("expected parent folder to remain active: %v", err)
	}
}

func newFileResourceSQLiteDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db failed: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	execFileResourceTestSchema(t, db)
	return db
}

func execFileResourceTestSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	statements := []string{
		`CREATE TABLE upload_file_folders (
			id integer primary key autoincrement,
			parent_id integer not null default 0,
			name text not null default '',
			logical_path text not null default '/',
			sort integer not null default 0,
			is_hidden integer not null default 0,
			created_by integer not null default 0,
			updated_by integer not null default 0,
			created_at datetime,
			updated_at datetime,
			deleted_at integer not null default 0
		)`,
		`CREATE TABLE upload_files (
			id integer primary key autoincrement,
			file_object_id integer not null default 0,
			uid integer not null default 0,
			folder_id integer not null default 0,
			logical_path text not null default '/',
			display_name text not null default '',
			origin_name text not null default '',
			name text not null default '',
			path text not null default '',
			size integer not null default 0,
			ext text not null default '',
			hash text not null default '',
			uuid text not null default '',
			mime_type text not null default '',
			file_type text not null default '',
			is_public integer not null default 0,
			storage_driver text not null default 'local',
			storage_base text not null default '',
			bucket text not null default '',
			storage_path text not null default '',
			object_key text not null default '',
			etag text not null default '',
			storage_status text not null default 'stored',
			upload_source text not null default '',
			upload_scene text not null default '',
			upload_status text not null default '',
			last_accessed_at datetime,
			deleted_by integer not null default 0,
			deleted_reason text not null default '',
			expires_at datetime,
			created_at datetime,
			updated_at datetime,
			deleted_at integer not null default 0
		)`,
		`CREATE TABLE upload_file_objects (
			id integer primary key autoincrement,
			storage_driver text not null default 'local',
			storage_base text not null default '',
			bucket text not null default '',
			storage_path text not null default '',
			object_key text not null default '',
			owner_uid integer not null default 0,
			is_public integer not null default 0,
			size integer not null default 0,
			hash text not null default '',
			mime_type text not null default '',
			etag text not null default '',
			status text not null default 'stored',
			created_at datetime,
			updated_at datetime
		)`,
		`CREATE TABLE upload_sessions (
			id integer primary key autoincrement,
			complete_token_hash text not null default '',
			upload_type text not null default '',
			status text not null default 'pending',
			storage_driver text not null default 'local',
			storage_base text not null default '',
			bucket text not null default '',
			storage_path text not null default '',
			object_key text not null default '',
			upload_id text not null default '',
			file_object_id integer not null default 0,
			file_id integer not null default 0,
			owner_uid integer not null default 0,
			folder_id integer not null default 0,
			origin_name text not null default '',
			size integer not null default 0,
			hash text not null default '',
			mime_type text not null default '',
			etag text not null default '',
			is_public integer not null default 0,
			upload_scene text not null default '',
			expires_at datetime,
			completed_at datetime,
			created_at datetime,
			updated_at datetime
		)`,
		`CREATE TABLE upload_file_references (
			id integer primary key autoincrement,
			file_id integer not null default 0,
			uuid text not null default '',
			owner_type text not null default '',
			owner_id integer not null default 0,
			owner_field text not null default '',
			created_at datetime,
			updated_at datetime
		)`,
		`CREATE TABLE admin_user (
			id integer primary key autoincrement,
			nickname text not null default '',
			username text not null default '',
			deleted_at integer not null default 0
		)`,
		// 与生产迁移 uniq_folder_name_deleted 对齐：同目录同名活跃记录唯一。
		`CREATE UNIQUE INDEX uniq_folder_name_deleted ON upload_files (folder_id, origin_name, deleted_at)`,
		`CREATE UNIQUE INDEX uniq_upload_session_token_hash ON upload_sessions (complete_token_hash)`,
		`CREATE UNIQUE INDEX uniq_file_object_scope_hash ON upload_file_objects (storage_driver, bucket, hash, owner_uid, is_public)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create test schema failed: %v", err)
		}
	}
}

func newMultipartFileHeader(t *testing.T, filename string, content []byte) *multipart.FileHeader {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("files", filename)
	if err != nil {
		t.Fatalf("create multipart file failed: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write multipart file failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer failed: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, "/", body)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(1024); err != nil {
		t.Fatalf("parse multipart form failed: %v", err)
	}
	return req.MultipartForm.File["files"][0]
}

type fakeFileResourceDriver struct {
	name                string
	content             []byte
	openErr             error
	putErr              error
	readErr             error
	exists              bool
	existsErr           error
	completeErr         error
	openFunc            func(bucket, objectKey string) ([]byte, error)
	onPut               func(bucket, objectKey string)
	onDelete            func(bucket, objectKey string)
	onCompleteMultipart func()
	onAbortMultipart    func(bucket, objectKey, uploadID string)
}

func testSHA256(content []byte) string {
	sum := sha256.Sum256(content)
	return fmt.Sprintf("%x", sum[:])
}

func (d fakeFileResourceDriver) Name() string { return d.name }

func (d fakeFileResourceDriver) Put(_ context.Context, input filestorage.PutInput) (filestorage.PutResult, error) {
	if input.Reader != nil {
		if _, err := io.Copy(io.Discard, input.Reader); err != nil {
			return filestorage.PutResult{}, err
		}
	}
	if d.putErr != nil {
		return filestorage.PutResult{}, d.putErr
	}
	if d.onPut != nil {
		d.onPut(input.Bucket, input.ObjectKey)
	}
	return filestorage.PutResult{Bucket: input.Bucket, ObjectKey: input.ObjectKey, ETag: "final-etag"}, nil
}

func (d fakeFileResourceDriver) Open(_ context.Context, bucket, objectKey string) (io.ReadCloser, error) {
	if d.openErr != nil {
		return nil, d.openErr
	}
	content := d.content
	if d.openFunc != nil {
		var err error
		content, err = d.openFunc(bucket, objectKey)
		if err != nil {
			return nil, err
		}
	}
	if content == nil {
		content = []byte("plain text content")
	}
	if d.readErr != nil {
		return io.NopCloser(&errorAfterPartialReader{content: content, err: d.readErr}), nil
	}
	return io.NopCloser(bytes.NewReader(content)), nil
}

type errorAfterPartialReader struct {
	content []byte
	err     error
	done    bool
}

func (r *errorAfterPartialReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, r.err
	}
	r.done = true
	return copy(p, r.content), r.err
}

func (d fakeFileResourceDriver) Exists(context.Context, string, string) (bool, error) {
	return d.exists, d.existsErr
}

func (d fakeFileResourceDriver) Delete(_ context.Context, bucket, objectKey string) error {
	if d.onDelete != nil {
		d.onDelete(bucket, objectKey)
	}
	return nil
}

type fakeStatFileResourceDriver struct {
	fakeFileResourceDriver
	statSize int64
	statErr  error
}

func (d fakeStatFileResourceDriver) Stat(context.Context, string, string) (filestorage.ObjectStat, error) {
	if d.statErr != nil {
		return filestorage.ObjectStat{}, d.statErr
	}
	return filestorage.ObjectStat{Size: d.statSize}, nil
}

func (d fakeFileResourceDriver) URL(string, string, bool) string {
	return ""
}

func (d fakeFileResourceDriver) SignedURL(context.Context, string, string, time.Duration) (string, error) {
	return "", nil
}

func (d fakeFileResourceDriver) PresignPutObject(context.Context, string, string, string, time.Duration) (string, error) {
	return "", nil
}

func (d fakeFileResourceDriver) InitMultipartUpload(_ context.Context, input filestorage.MultipartUploadInput) (filestorage.MultipartUploadResult, error) {
	return filestorage.MultipartUploadResult{
		UploadID:  "upload-id",
		Bucket:    input.Bucket,
		ObjectKey: input.ObjectKey,
	}, nil
}

func (d fakeFileResourceDriver) PresignMultipartPart(context.Context, filestorage.MultipartPartInput) (filestorage.MultipartPartResult, error) {
	return filestorage.MultipartPartResult{}, nil
}

func (d fakeFileResourceDriver) CompleteMultipartUpload(context.Context, string, string, string, []filestorage.CompletePartInfo) error {
	if d.onCompleteMultipart != nil {
		d.onCompleteMultipart()
	}
	if d.completeErr != nil {
		return d.completeErr
	}
	return nil
}

func (d fakeFileResourceDriver) AbortMultipartUpload(_ context.Context, bucket, objectKey, uploadID string) error {
	if d.onAbortMultipart != nil {
		d.onAbortMultipart(bucket, objectKey, uploadID)
	}
	return nil
}

func TestFileResourceService_BatchRestore(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db})

	deletedAt := soft_delete.DeletedAt(time.Now().Unix())
	parent := model.UploadFileFolder{
		Name:        "restore-parent",
		LogicalPath: "/restore-parent",
		ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{
			DeletedAt: deletedAt,
		},
	}
	if err := db.Create(&parent).Error; err != nil {
		t.Fatalf("create parent folder failed: %v", err)
	}
	child := model.UploadFileFolder{
		ParentID:    parent.ID,
		Name:        "restore-child",
		LogicalPath: "/restore-parent/restore-child",
		ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{
			DeletedAt: deletedAt,
		},
	}
	if err := db.Create(&child).Error; err != nil {
		t.Fatalf("create child folder failed: %v", err)
	}

	// 创建已软删除的文件
	file1 := model.UploadFiles{
		OriginName:    "restore1.jpg",
		UUID:          "restore1-uuid-00000000000000000000",
		FolderID:      child.ID,
		StorageDriver: model.StorageDriverLocal,
		StorageStatus: model.StorageStatusStored,
		ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{
			DeletedAt: deletedAt,
		},
		DeletedBy:     1,
		DeletedReason: "soft delete",
	}
	file2 := model.UploadFiles{
		OriginName:    "restore2.jpg",
		UUID:          "restore2-uuid-00000000000000000000",
		FolderID:      child.ID,
		StorageDriver: model.StorageDriverLocal,
		StorageStatus: model.StorageStatusStored,
		ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{
			DeletedAt: deletedAt,
		},
		DeletedBy:     1,
		DeletedReason: "soft delete",
	}
	if err := db.Create(&file1).Error; err != nil {
		t.Fatalf("create file1 failed: %v", err)
	}
	if err := db.Create(&file2).Error; err != nil {
		t.Fatalf("create file2 failed: %v", err)
	}

	// 批量恢复
	err := service.BatchRestore([]uint{file1.ID, file2.ID})
	if err != nil {
		t.Fatalf("BatchRestore failed: %v", err)
	}

	// 检查数据库记录
	var res1, res2 model.UploadFiles
	if err := db.First(&res1, file1.ID).Error; err != nil {
		t.Fatalf("query file1 failed: %v", err)
	}
	if err := db.First(&res2, file2.ID).Error; err != nil {
		t.Fatalf("query file2 failed: %v", err)
	}

	if res1.DeletedAt != 0 || res1.DeletedBy != 0 || res1.DeletedReason != "" {
		t.Fatalf("file1 not restored correctly: %+v", res1)
	}
	if res2.DeletedAt != 0 || res2.DeletedBy != 0 || res2.DeletedReason != "" {
		t.Fatalf("file2 not restored correctly: %+v", res2)
	}
	var restoredParent, restoredChild model.UploadFileFolder
	if err := db.First(&restoredParent, parent.ID).Error; err != nil {
		t.Fatalf("expected parent folder restored: %v", err)
	}
	if err := db.First(&restoredChild, child.ID).Error; err != nil {
		t.Fatalf("expected child folder restored: %v", err)
	}
}

func TestFileResourceService_BatchDestroy(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	// 用 mock 的 storage driver 规避物理文件删除的报错
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		StorageDriverResolver: func(ctx context.Context, driver string) (filestorage.Driver, filestorage.Config, error) {
			return fakeFileResourceDriver{name: driver}, filestorage.Config{}, nil
		},
	})

	// 创建已软删除的文件
	file1 := model.UploadFiles{
		OriginName:    "destroy1.jpg",
		UUID:          "destroy1-uuid-00000000000000000000",
		StorageDriver: model.StorageDriverLocal,
		StorageStatus: model.StorageStatusStored,
		ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{
			DeletedAt: soft_delete.DeletedAt(time.Now().Unix()),
		},
	}
	file2 := model.UploadFiles{
		OriginName:    "destroy2.jpg",
		UUID:          "destroy2-uuid-00000000000000000000",
		StorageDriver: model.StorageDriverLocal,
		StorageStatus: model.StorageStatusStored,
		ContainsDeleteBaseModel: model.ContainsDeleteBaseModel{
			DeletedAt: soft_delete.DeletedAt(time.Now().Unix()),
		},
	}
	if err := db.Create(&file1).Error; err != nil {
		t.Fatalf("create file1 failed: %v", err)
	}
	if err := db.Create(&file2).Error; err != nil {
		t.Fatalf("create file2 failed: %v", err)
	}

	// 批量硬删除
	res, err := service.BatchDestroy([]uint{file1.ID, file2.ID})
	if err != nil {
		t.Fatalf("BatchDestroy failed: %v", err)
	}

	if res.Destroyed != 2 || res.Failed != 0 {
		t.Fatalf("expected destroyed=2 failed=0, got destroyed=%d failed=%d", res.Destroyed, res.Failed)
	}

	// 检查数据库记录
	var count int64
	db.Unscoped().Model(&model.UploadFiles{}).Where("id IN ?", []uint{file1.ID, file2.ID}).Count(&count)
	if count != 0 {
		t.Fatalf("expected 0 files, got %d files remaining", count)
	}
}

func TestFileResourceUploadLimits(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	config.Config.BasePath = t.TempDir()

	// 1. 测试文件过大限制校验 (MaxFileSizeMB = 1)
	serviceLarge := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return fakeFileResourceDriver{name: "aliyun_oss"}, filestorage.Config{
				MaxFileSizeMB: 1,
			}, "aliyun_oss", nil
		},
		StorageDriverResolver: func(_ context.Context, driverName string) (filestorage.Driver, filestorage.Config, error) {
			return fakeFileResourceDriver{name: driverName}, filestorage.Config{
				MaxFileSizeMB: 1,
			}, nil
		},
	})

	// 1a. 本地上传，大小超过 1MB (1024*1024 + 1 bytes)
	largeData := make([]byte, 1024*1024+1)
	headerLarge := newMultipartFileHeader(t, "large.txt", largeData)
	_, err := serviceLarge.UploadLocal([]*multipart.FileHeader{headerLarge}, &form.FileLocalUpload{IsPublic: global.Yes}, 1)
	if err == nil {
		t.Fatal("expected local upload of large file to fail")
	}
	var bizErr *e.BusinessError
	if !stderrors.As(err, &bizErr) || bizErr.GetCode() != e.FileTooLarge {
		t.Fatalf("expected FileTooLarge (11012) error for local upload, got: %v", err)
	}

	// 1b. 直传凭证，大小超过 1MB
	_, err = serviceLarge.UploadCredential(&form.FileUploadCredential{
		OriginName: "large.txt",
		Size:       1024*1024 + 1,
		MimeType:   "text/plain",
	})
	if err == nil {
		t.Fatal("expected credential of large file to fail")
	}
	if !stderrors.As(err, &bizErr) || bizErr.GetCode() != e.FileTooLarge {
		t.Fatalf("expected FileTooLarge (11012) error for credential, got: %v", err)
	}

	// 1c. 分片上传初始化，大小超过 1MB
	_, err = serviceLarge.MultipartInit(&form.FileMultipartInit{
		OriginName: "large.txt",
		Size:       1024*1024 + 1,
		MimeType:   "text/plain",
		Hash:       strings.Repeat("a", 64),
	})
	if err == nil {
		t.Fatal("expected multipart init of large file to fail")
	}
	if !stderrors.As(err, &bizErr) || bizErr.GetCode() != e.FileTooLarge {
		t.Fatalf("expected FileTooLarge (11012) error for multipart init, got: %v", err)
	}

	// 2. 测试不支持的 MIME 类型 (AllowedMimeTypes = []string{"image/*"})
	serviceMime := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return fakeFileResourceDriver{name: "aliyun_oss"}, filestorage.Config{
				MaxFileSizeMB:    10,
				AllowedMimeTypes: []string{"image/*"},
			}, "aliyun_oss", nil
		},
		StorageDriverResolver: func(_ context.Context, driverName string) (filestorage.Driver, filestorage.Config, error) {
			return fakeFileResourceDriver{name: driverName}, filestorage.Config{
				MaxFileSizeMB:    10,
				AllowedMimeTypes: []string{"image/*"},
			}, nil
		},
	})

	// 2a. 本地上传不支持的 MIME 类型 (text/plain)
	headerText := newMultipartFileHeader(t, "test.txt", []byte("hello"))
	_, err = serviceMime.UploadLocal([]*multipart.FileHeader{headerText}, &form.FileLocalUpload{IsPublic: global.Yes}, 1)
	if err == nil {
		t.Fatal("expected local upload of disallowed mime to fail")
	}
	if !stderrors.As(err, &bizErr) || bizErr.GetCode() != e.FileMimeTypeNotAllowed {
		t.Fatalf("expected FileMimeTypeNotAllowed (11013) error for local upload, got: %v", err)
	}

	// 2b. 直传凭证不支持的 MIME 类型
	_, err = serviceMime.UploadCredential(&form.FileUploadCredential{
		OriginName: "test.txt",
		Size:       100,
		MimeType:   "text/plain",
	})
	if err == nil {
		t.Fatal("expected credential of disallowed mime to fail")
	}
	if !stderrors.As(err, &bizErr) || bizErr.GetCode() != e.FileMimeTypeNotAllowed {
		t.Fatalf("expected FileMimeTypeNotAllowed (11013) error for credential, got: %v", err)
	}

	// 2c. 分片上传初始化不支持的 MIME 类型
	_, err = serviceMime.MultipartInit(&form.FileMultipartInit{
		OriginName: "test.txt",
		Size:       100,
		MimeType:   "text/plain",
		Hash:       strings.Repeat("b", 64),
	})
	if err == nil {
		t.Fatal("expected multipart init of disallowed mime to fail")
	}
	if !stderrors.As(err, &bizErr) || bizErr.GetCode() != e.FileMimeTypeNotAllowed {
		t.Fatalf("expected FileMimeTypeNotAllowed (11013) error for multipart init, got: %v", err)
	}

	// 3. 测试允许的通配符场景 (*/*)
	servicePass := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return fakeFileResourceDriver{name: "aliyun_oss"}, filestorage.Config{
				MaxFileSizeMB:    10,
				AllowedMimeTypes: []string{"*/*"},
			}, "aliyun_oss", nil
		},
		StorageDriverResolver: func(_ context.Context, driverName string) (filestorage.Driver, filestorage.Config, error) {
			return fakeFileResourceDriver{name: driverName}, filestorage.Config{
				MaxFileSizeMB:    10,
				AllowedMimeTypes: []string{"*/*"},
			}, nil
		},
	})

	// 3a. 本地上传通过通配符通过校验
	_, err = servicePass.UploadLocal([]*multipart.FileHeader{headerText}, &form.FileLocalUpload{IsPublic: global.Yes}, 1)
	if err != nil {
		t.Fatalf("expected local upload to succeed with wildcard, got: %v", err)
	}

	// 3a-1. 默认白名单下，浏览器/客户端未提供可靠 MIME 时允许通过安全扩展名推导
	serviceDefault := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return fakeFileResourceDriver{name: "aliyun_oss"}, filestorage.DefaultConfig(), "aliyun_oss", nil
		},
		StorageDriverResolver: func(_ context.Context, driverName string) (filestorage.Driver, filestorage.Config, error) {
			return fakeFileResourceDriver{name: driverName}, filestorage.DefaultConfig(), nil
		},
	})
	headerOctetText := newMultipartFileHeader(t, "test.txt", []byte("hello"))
	headerOctetText.Header.Set("Content-Type", "application/octet-stream")
	_, err = serviceDefault.UploadLocal([]*multipart.FileHeader{headerOctetText}, &form.FileLocalUpload{IsPublic: global.Yes}, 1)
	if err != nil {
		t.Fatalf("expected .txt upload with octet-stream to pass default whitelist, got: %v", err)
	}

	// 3b. 直传凭证通过通配符校验
	_, err = servicePass.UploadCredential(&form.FileUploadCredential{
		OriginName: "test.txt",
		Size:       100,
		Hash:       strings.Repeat("c", 64),
		MimeType:   "text/plain",
	})
	if err != nil {
		t.Fatalf("expected credential to succeed with wildcard, got: %v", err)
	}

	// 3c. 分片上传初始化通过通配符校验
	_, err = servicePass.MultipartInit(&form.FileMultipartInit{
		OriginName: "test.txt",
		Size:       100,
		MimeType:   "text/plain",
		Hash:       strings.Repeat("c", 64),
	})
	if err != nil {
		t.Fatalf("expected multipart init to succeed with wildcard, got: %v", err)
	}

	// 4. 测试允许的后缀类型限制校验 (AllowedExtensions = []string{".jpg", "png"})
	serviceExt := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return fakeFileResourceDriver{name: "aliyun_oss"}, filestorage.Config{
				MaxFileSizeMB:     10,
				AllowedExtensions: []string{".jpg", "png"},
			}, "aliyun_oss", nil
		},
		StorageDriverResolver: func(_ context.Context, driverName string) (filestorage.Driver, filestorage.Config, error) {
			return fakeFileResourceDriver{name: driverName}, filestorage.Config{
				MaxFileSizeMB:     10,
				AllowedExtensions: []string{".jpg", "png"},
			}, nil
		},
	})

	// 4a. 本地上传不支持的后缀 (test.txt)
	headerText = newMultipartFileHeader(t, "test.txt", []byte("hello"))
	_, err = serviceExt.UploadLocal([]*multipart.FileHeader{headerText}, &form.FileLocalUpload{IsPublic: global.Yes}, 1)
	if err == nil {
		t.Fatal("expected local upload of disallowed extension to fail")
	}
	if !stderrors.As(err, &bizErr) || bizErr.GetCode() != e.FileMimeTypeNotAllowed {
		t.Fatalf("expected FileMimeTypeNotAllowed (11013) error for local upload, got: %v", err)
	}

	// 4b. 本地上传支持的后缀 (.jpg)
	headerJpg := newMultipartFileHeader(t, "test.jpg", []byte("hello"))
	_, err = serviceExt.UploadLocal([]*multipart.FileHeader{headerJpg}, &form.FileLocalUpload{IsPublic: global.Yes}, 1)
	if err != nil {
		t.Fatalf("expected local upload of allowed extension (.jpg) to succeed, got: %v", err)
	}

	// 4c. 本地上传支持的后缀 (png)
	headerPng := newMultipartFileHeader(t, "test.png", []byte("hello"))
	_, err = serviceExt.UploadLocal([]*multipart.FileHeader{headerPng}, &form.FileLocalUpload{IsPublic: global.Yes}, 1)
	if err != nil {
		t.Fatalf("expected local upload of allowed extension (png) to succeed, got: %v", err)
	}

	// 5. 测试允许的后缀通配符 (*)
	serviceExtWildcard := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return fakeFileResourceDriver{name: "aliyun_oss"}, filestorage.Config{
				MaxFileSizeMB:     10,
				AllowedExtensions: []string{"*"},
			}, "aliyun_oss", nil
		},
		StorageDriverResolver: func(_ context.Context, driverName string) (filestorage.Driver, filestorage.Config, error) {
			return fakeFileResourceDriver{name: driverName}, filestorage.Config{
				MaxFileSizeMB:     10,
				AllowedExtensions: []string{"*"},
			}, nil
		},
	})

	// 5a. 本地上传通过通配符通过校验
	_, err = serviceExtWildcard.UploadLocal([]*multipart.FileHeader{headerText}, &form.FileLocalUpload{IsPublic: global.Yes}, 1)
	if err != nil {
		t.Fatalf("expected local upload to succeed with wildcard (*), got: %v", err)
	}

	// 6. 测试防篡改/后缀与MIME强校验拦截 (AllowedExtensions = []string{".jpg", "png"})
	// 6a. 上传后缀为 .jpg，但 MIME 为 application/x-msdownload 恶意文件的直传凭证
	_, err = serviceExt.UploadCredential(&form.FileUploadCredential{
		OriginName: "test.jpg",
		Size:       100,
		MimeType:   "application/x-msdownload",
	})
	if err == nil {
		t.Fatal("expected credential of modified filename extension to fail")
	}
	if !stderrors.As(err, &bizErr) || bizErr.GetCode() != e.FileMimeTypeNotAllowed {
		t.Fatalf("expected FileMimeTypeNotAllowed (11013) error for credential extension tamper, got: %v", err)
	}

	// 6b. 本地上传后缀为 .jpg，但 MIME 为 application/x-msdownload 的恶意文件
	headerTampered := newMultipartFileHeader(t, "test.jpg", []byte("hello"))
	headerTampered.Header.Set("Content-Type", "application/x-msdownload")
	_, err = serviceExt.UploadLocal([]*multipart.FileHeader{headerTampered}, &form.FileLocalUpload{IsPublic: global.Yes}, 1)
	if err == nil {
		t.Fatal("expected local upload of modified filename extension to fail")
	}
	if !stderrors.As(err, &bizErr) || bizErr.GetCode() != e.FileMimeTypeNotAllowed {
		t.Fatalf("expected FileMimeTypeNotAllowed (11013) error for local upload extension tamper, got: %v", err)
	}
}

func TestFileResourceUploadOptionsExposeOnlyUploadCapabilities(t *testing.T) {
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return fakeFileResourceDriver{name: model.StorageDriverAliyunOSS}, filestorage.Config{
				MaxFileSizeMB:     25,
				AllowedExtensions: []string{".png", ".pdf"},
				AllowedMimeTypes:  []string{"image/png", "application/pdf"},
				AliyunOSS: filestorage.AliyunOSSConfig{
					Bucket:          "secret-bucket",
					AccessKeySecret: "must-not-be-exposed",
				},
			}, model.StorageDriverAliyunOSS, nil
		},
	})
	options, err := service.UploadOptions(context.Background())
	if err != nil {
		t.Fatalf("upload options failed: %v", err)
	}
	if options.ActiveDriver != model.StorageDriverAliyunOSS || options.MaxFileSizeMB != 25 || !options.SupportsDirectUpload || !options.SupportsMultipartUpload {
		t.Fatalf("unexpected upload options: %#v", options)
	}
	if len(options.AllowedExtensions) != 2 || len(options.AllowedMimeTypes) != 2 {
		t.Fatalf("expected allowlists in upload options: %#v", options)
	}
}

func TestFileResourceDefaultWhitelistAllowsOctetStreamForKnownExtensions(t *testing.T) {
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return fakeFileResourceDriver{name: model.StorageDriverAliyunOSS}, filestorage.DefaultConfig(), model.StorageDriverAliyunOSS, nil
		},
	})

	tests := []struct {
		name     string
		filename string
		mimeType string
	}{
		{name: "txt empty mime", filename: "readme.txt", mimeType: ""},
		{name: "png octet stream", filename: "avatar.png", mimeType: "application/octet-stream"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := service.validateFileLimit(tt.filename, 12, tt.mimeType); err != nil {
				t.Fatalf("expected %s with mime %q to pass default whitelist, got %v", tt.filename, tt.mimeType, err)
			}
		})
	}
}

func TestFileResourceDefaultWhitelistRejectsExecutableExtension(t *testing.T) {
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return fakeFileResourceDriver{name: model.StorageDriverAliyunOSS}, filestorage.DefaultConfig(), model.StorageDriverAliyunOSS, nil
		},
	})

	err := service.validateFileLimit("payload.exe", 12, "application/octet-stream")
	if err == nil {
		t.Fatal("expected executable extension to be rejected by default whitelist")
	}
	var bizErr *e.BusinessError
	if !stderrors.As(err, &bizErr) || bizErr.GetCode() != e.FileMimeTypeNotAllowed {
		t.Fatalf("expected FileMimeTypeNotAllowed for executable extension, got %v", err)
	}
}

func TestEnsureExportTempFolder(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db})

	// 第一次调用：应创建 system_temp 目录，is_hidden=1
	folderID, err := service.ensureExportTempFolder(context.Background())
	if err != nil {
		t.Fatalf("first ensureExportTempFolder failed: %v", err)
	}
	if folderID == 0 {
		t.Fatal("expected non-zero folder ID")
	}
	var folder model.UploadFileFolder
	if err := db.First(&folder, folderID).Error; err != nil {
		t.Fatalf("query created folder failed: %v", err)
	}
	if folder.Name != SystemTempFolderName {
		t.Fatalf("expected folder name %q, got %q", SystemTempFolderName, folder.Name)
	}
	if folder.ParentID != 0 {
		t.Fatalf("expected parent_id=0, got %d", folder.ParentID)
	}
	if folder.IsHidden != 1 {
		t.Fatalf("expected is_hidden=1, got %d", folder.IsHidden)
	}
	if folder.LogicalPath != "/"+SystemTempFolderName {
		t.Fatalf("expected logical_path /%s, got %q", SystemTempFolderName, folder.LogicalPath)
	}

	// 第二次调用：应返回同一目录
	folderID2, err := service.ensureExportTempFolder(context.Background())
	if err != nil {
		t.Fatalf("second ensureExportTempFolder failed: %v", err)
	}
	if folderID2 != folderID {
		t.Fatalf("expected same folder ID on second call, got %d vs %d", folderID2, folderID)
	}
}

func TestEnsureExportTempFolderSelfHealsHiddenFlag(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db})

	// 手动创建一个 is_hidden=0 的 system_temp 目录（模拟历史数据）
	existing := model.UploadFileFolder{
		ParentID:    0,
		Name:        SystemTempFolderName,
		LogicalPath: "/" + SystemTempFolderName,
		IsHidden:    0,
		CreatedBy:   0,
		UpdatedBy:   0,
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("create existing folder failed: %v", err)
	}

	// 调用 ensureExportTempFolder，应自愈 is_hidden=1
	folderID, err := service.ensureExportTempFolder(context.Background())
	if err != nil {
		t.Fatalf("ensureExportTempFolder failed: %v", err)
	}
	if folderID != existing.ID {
		t.Fatalf("expected same folder ID, got %d vs %d", folderID, existing.ID)
	}
	var folder model.UploadFileFolder
	if err := db.First(&folder, folderID).Error; err != nil {
		t.Fatalf("query folder failed: %v", err)
	}
	if folder.IsHidden != 1 {
		t.Fatalf("expected is_hidden=1 after self-heal, got %d", folder.IsHidden)
	}
}

func TestFolderTreeExcludesHiddenByDefault(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db})

	visible := model.UploadFileFolder{Name: "visible", LogicalPath: "/visible", IsHidden: 0}
	hidden := model.UploadFileFolder{Name: "hidden", LogicalPath: "/hidden", IsHidden: 1}
	if err := db.Create(&visible).Error; err != nil {
		t.Fatalf("create visible folder failed: %v", err)
	}
	if err := db.Create(&hidden).Error; err != nil {
		t.Fatalf("create hidden folder failed: %v", err)
	}

	// 默认不包含隐藏
	result, err := service.FolderTree(false)
	if err != nil {
		t.Fatalf("FolderTree failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 folder (excluding hidden), got %d", len(result))
	}
	if result[0].Name != "visible" {
		t.Fatalf("expected visible folder, got %q", result[0].Name)
	}
}

func TestFolderTreeIncludesHiddenWhenRequested(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db})

	visible := model.UploadFileFolder{Name: "visible", LogicalPath: "/visible", IsHidden: 0}
	hidden := model.UploadFileFolder{Name: "hidden", LogicalPath: "/hidden", IsHidden: 1}
	if err := db.Create(&visible).Error; err != nil {
		t.Fatalf("create visible folder failed: %v", err)
	}
	if err := db.Create(&hidden).Error; err != nil {
		t.Fatalf("create hidden folder failed: %v", err)
	}

	// 显式包含隐藏
	result, err := service.FolderTree(true)
	if err != nil {
		t.Fatalf("FolderTree failed: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 folders (including hidden), got %d", len(result))
	}
}

func TestFolderTreeFillsCreatorName(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db})

	if err := db.Exec("INSERT INTO admin_user (id, nickname, username) VALUES (?, ?, ?)", 11, "超级管理员", "admin").Error; err != nil {
		t.Fatalf("create admin user failed: %v", err)
	}
	folder := model.UploadFileFolder{Name: "folder", LogicalPath: "/folder", CreatedBy: 11, UpdatedBy: 11}
	if err := db.Create(&folder).Error; err != nil {
		t.Fatalf("create folder failed: %v", err)
	}

	result, err := service.FolderTree(false)
	if err != nil {
		t.Fatalf("FolderTree failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 folder, got %d", len(result))
	}
	if result[0].CreatorName != "超级管理员" || result[0].CreatorUsername != "admin" {
		t.Fatalf("unexpected creator info: name=%q username=%q", result[0].CreatorName, result[0].CreatorUsername)
	}
	if result[0].UploaderName != "超级管理员" || result[0].UploaderUsername != "admin" {
		t.Fatalf("unexpected uploader fallback: name=%q username=%q", result[0].UploaderName, result[0].UploaderUsername)
	}
}

func TestFileResourceListExcludesFilesInHiddenFolderByDefault(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db})

	hiddenFolder := model.UploadFileFolder{Name: "hidden", LogicalPath: "/hidden", IsHidden: 1}
	visibleFolder := model.UploadFileFolder{Name: "visible", LogicalPath: "/visible", IsHidden: 0}
	if err := db.Create(&hiddenFolder).Error; err != nil {
		t.Fatalf("create hidden folder failed: %v", err)
	}
	if err := db.Create(&visibleFolder).Error; err != nil {
		t.Fatalf("create visible folder failed: %v", err)
	}

	hiddenFile := model.UploadFiles{
		OriginName:    "hidden.txt",
		UUID:          fmt.Sprintf("%032x", time.Now().UnixNano()),
		FolderID:      hiddenFolder.ID,
		StorageDriver: model.StorageDriverLocal,
		StorageStatus: model.StorageStatusStored,
	}
	visibleFile := model.UploadFiles{
		OriginName:    "visible.txt",
		UUID:          fmt.Sprintf("%032x", time.Now().UnixNano()+1),
		FolderID:      visibleFolder.ID,
		StorageDriver: model.StorageDriverLocal,
		StorageStatus: model.StorageStatusStored,
	}
	if err := db.Create(&hiddenFile).Error; err != nil {
		t.Fatalf("create hidden file failed: %v", err)
	}
	if err := db.Create(&visibleFile).Error; err != nil {
		t.Fatalf("create visible file failed: %v", err)
	}

	// 默认排除隐藏目录下的文件
	result, err := service.List(&form.FileResourceList{
		Paginate: form.Paginate{Page: 1, PerPage: 10},
	})
	if err != nil {
		t.Fatalf("list files failed: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("expected 1 file (excluding hidden folder), got %d", result.Total)
	}
	item, ok := result.Data[0].(*resources.FileResourceResources)
	if !ok {
		t.Fatalf("unexpected item type: %T", result.Data[0])
	}
	if item.OriginName != "visible.txt" {
		t.Fatalf("expected visible.txt, got %q", item.OriginName)
	}
}

func TestFileResourceListIncludesHiddenFolderFilesWhenRequested(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db})

	hiddenFolder := model.UploadFileFolder{Name: "hidden", LogicalPath: "/hidden", IsHidden: 1}
	if err := db.Create(&hiddenFolder).Error; err != nil {
		t.Fatalf("create hidden folder failed: %v", err)
	}

	hiddenFile := model.UploadFiles{
		OriginName:    "hidden.txt",
		UUID:          fmt.Sprintf("%032x", time.Now().UnixNano()),
		FolderID:      hiddenFolder.ID,
		StorageDriver: model.StorageDriverLocal,
		StorageStatus: model.StorageStatusStored,
	}
	if err := db.Create(&hiddenFile).Error; err != nil {
		t.Fatalf("create hidden file failed: %v", err)
	}

	includeHidden := uint8(1)
	result, err := service.List(&form.FileResourceList{
		Paginate:      form.Paginate{Page: 1, PerPage: 10},
		IncludeHidden: &includeHidden,
	})
	if err != nil {
		t.Fatalf("list files failed: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("expected 1 file (include_hidden=1), got %d", result.Total)
	}
	item, ok := result.Data[0].(*resources.FileResourceResources)
	if !ok {
		t.Fatalf("unexpected item type: %T", result.Data[0])
	}
	if item.OriginName != "hidden.txt" {
		t.Fatalf("expected hidden.txt, got %q", item.OriginName)
	}
}

func newLocalUploadTestService(db *gorm.DB) *FileResourceService {
	settings := StorageSettings{ActiveDriver: model.StorageDriverLocal, Config: filestorage.DefaultConfig()}
	applyLocalStorageDefaults(&settings)
	return NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(ctx context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			driver, err := buildStorageDriver(ctx, settings.ActiveDriver, settings.Config)
			return driver, settings.Config, settings.ActiveDriver, err
		},
	})
}
