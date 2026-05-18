package service

import (
	"bytes"
	"context"
	stderrors "errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/data"
	"github.com/wannanbigpig/gin-layout/internal/filestorage"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	e "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
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

func currentConfigFileResourceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path failed")
	}
	projectRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	if err := config.InitConfig(filepath.Join(projectRoot, "config.yaml")); err != nil {
		t.Fatalf("init current config failed: %v", err)
	}
	if err := data.InitData(); err != nil {
		t.Fatalf("init current configured data failed: %v", err)
	}
	db, err := model.GetDB()
	if err != nil {
		t.Fatalf("get current configured db failed: %v", err)
	}
	return db
}

func cleanupFileResourceTestData(t *testing.T, db *gorm.DB, uuid string) {
	t.Helper()

	if err := db.Where("uuid = ?", uuid).Delete(&model.UploadFileReference{}).Error; err != nil {
		t.Fatalf("cleanup upload file references failed: %v", err)
	}
	if err := db.Unscoped().Where("uuid = ?", uuid).Delete(&model.UploadFiles{}).Error; err != nil {
		t.Fatalf("cleanup upload files failed: %v", err)
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

func TestFileResourceUploadLocalRecordsLocationFields(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	config.Config.BasePath = t.TempDir()
	folder := model.UploadFileFolder{Name: "images", LogicalPath: "/images"}
	if err := db.Create(&folder).Error; err != nil {
		t.Fatalf("create folder failed: %v", err)
	}
	header := newMultipartFileHeader(t, "hello.txt", []byte("hello"))

	result, err := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db}).UploadLocal([]*multipart.FileHeader{header}, &form.FileLocalUpload{FolderID: folder.ID, IsPublic: global.Yes, UploadScene: "test"}, 9)
	if err != nil {
		t.Fatalf("upload local failed: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected one upload result, got %d", len(result))
	}
	var stored model.UploadFiles
	if err := db.First(&stored, result[0].ID).Error; err != nil {
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
}

func TestFileResourceUploadLocalReusesPhysicalObject(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	config.Config.BasePath = t.TempDir()
	service := NewFileResourceServiceWithDeps(FileResourceServiceDeps{DB: db})
	content := []byte("same content")

	first, err := service.UploadLocal([]*multipart.FileHeader{newMultipartFileHeader(t, "first.txt", content)}, &form.FileLocalUpload{IsPublic: global.Yes}, 1)
	if err != nil {
		t.Fatalf("first upload failed: %v", err)
	}
	second, err := service.UploadLocal([]*multipart.FileHeader{newMultipartFileHeader(t, "second.txt", content)}, &form.FileLocalUpload{IsPublic: global.No}, 2)
	if err != nil {
		t.Fatalf("second upload failed: %v", err)
	}
	if first[0].FileObjectID == 0 || first[0].FileObjectID != second[0].FileObjectID {
		t.Fatalf("expected same file object, first=%d second=%d", first[0].FileObjectID, second[0].FileObjectID)
	}
	if first[0].ObjectKey != second[0].ObjectKey {
		t.Fatalf("expected same object key, first=%q second=%q", first[0].ObjectKey, second[0].ObjectKey)
	}
	var objectCount int64
	if err := db.Model(&model.UploadFileObject{}).Count(&objectCount).Error; err != nil {
		t.Fatalf("count objects failed: %v", err)
	}
	if objectCount != 1 {
		t.Fatalf("expected one physical object, got %d", objectCount)
	}
	storedPath := filepath.Join(config.Config.BasePath, "storage/public", filepath.FromSlash(first[0].ObjectKey))
	if _, err := os.Stat(storedPath); err != nil {
		t.Fatalf("expected reused physical file to exist: %v", err)
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
	first := model.UploadFiles{FileObjectID: object.ID, UUID: fmt.Sprintf("%032x", time.Now().UnixNano()), StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored}
	second := model.UploadFiles{FileObjectID: object.ID, UUID: fmt.Sprintf("%032x", time.Now().UnixNano()+1), StorageDriver: model.StorageDriverLocal, StorageStatus: model.StorageStatusStored}
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

func TestFileResourceUploadCredentialReturnsReuse(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	hash := strings.Repeat("b", 64)
	object := model.UploadFileObject{
		StorageDriver: model.StorageDriverS3,
		Bucket:        "assets",
		StoragePath:   "uploads/existing.txt",
		ObjectKey:     "uploads/existing.txt",
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
			return fakeFileResourceDriver{name: model.StorageDriverLocal}, filestorage.Config{
				S3: filestorage.S3Config{Bucket: "assets"},
			}, model.StorageDriverLocal, nil
		},
	})
	result, err := service.UploadCredential(&form.FileUploadCredential{
		Driver:   model.StorageDriverS3,
		Hash:     hash,
		MimeType: "text/plain",
		Size:     12,
	})
	if err != nil {
		t.Fatalf("upload credential failed: %v", err)
	}
	if !result.Reuse || result.FileObjectID != object.ID || result.ObjectKey != object.ObjectKey {
		t.Fatalf("unexpected reuse credential: %#v", result)
	}
	if result.UploadURL != "" || result.Method != "" {
		t.Fatalf("reuse credential should not require direct upload: %#v", result)
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
		StorageDriver: model.StorageDriverS3,
		Bucket:        "bucket",
		ObjectKey:     "old/object.txt",
	})
	if err != nil {
		t.Fatalf("delete physical file failed: %v", err)
	}
	if len(calls) != 1 || calls[0] != model.StorageDriverS3 {
		t.Fatalf("expected resolver to use file storage driver, got %#v", calls)
	}
}

func newFileResourceSQLiteDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
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
			size integer not null default 0,
			hash text not null default '',
			mime_type text not null default '',
			etag text not null default '',
			status text not null default 'stored',
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
	name string
}

func (d fakeFileResourceDriver) Name() string { return d.name }

func (d fakeFileResourceDriver) Put(context.Context, filestorage.PutInput) (filestorage.PutResult, error) {
	return filestorage.PutResult{}, nil
}

func (d fakeFileResourceDriver) Open(context.Context, string, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(nil)), nil
}

func (d fakeFileResourceDriver) Exists(context.Context, string, string) (bool, error) {
	return true, nil
}

func (d fakeFileResourceDriver) Delete(context.Context, string, string) error {
	return nil
}

func (d fakeFileResourceDriver) URL(string, string, bool) string {
	return ""
}

func (d fakeFileResourceDriver) SignedURL(context.Context, string, string, time.Duration) (string, error) {
	return "", nil
}
