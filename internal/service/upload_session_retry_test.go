package service

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/wannanbigpig/gin-layout/internal/filestorage"
	"github.com/wannanbigpig/gin-layout/internal/global"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/resources"
	"github.com/wannanbigpig/gin-layout/internal/validator/form"
)

func newUploadRetryTestService(db *gorm.DB, driver filestorage.Driver) *FileResourceService {
	return NewFileResourceServiceWithDeps(FileResourceServiceDeps{
		DB: db,
		ActiveStorageResolver: func(context.Context) (filestorage.Driver, filestorage.Config, string, error) {
			return driver, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, model.StorageDriverAliyunOSS, nil
		},
		StorageDriverResolver: func(context.Context, string) (filestorage.Driver, filestorage.Config, error) {
			return driver, filestorage.Config{AliyunOSS: filestorage.AliyunOSSConfig{Bucket: "assets"}}, nil
		},
	})
}

func multipartRetryParams(initResult *resources.MultipartInitResource, content []byte) *form.FileMultipartComplete {
	return &form.FileMultipartComplete{
		CompleteToken: initResult.CompleteToken,
		UploadID:      initResult.UploadID,
		Bucket:        initResult.Bucket,
		ObjectKey:     initResult.ObjectKey,
		OriginName:    "retry.txt",
		Size:          uint(len(content)),
		Hash:          testSHA256(content),
		MimeType:      "text/plain",
		Driver:        model.StorageDriverAliyunOSS,
		IsPublic:      global.Yes,
		Parts:         []form.MultipartCompletePart{{PartNumber: 1, ETag: "part-etag"}},
	}
}

func TestMultipartCompleteRetriesReadyObjectAfterHashReadError(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	content := []byte("retry multipart after read error")
	openCalls := 0
	completeCalls := 0
	driver := fakeFileResourceDriver{
		name: model.StorageDriverAliyunOSS,
		openFunc: func(_, _ string) ([]byte, error) {
			openCalls++
			if openCalls == 2 {
				return nil, stderrors.New("temporary object read failure")
			}
			return content, nil
		},
		onCompleteMultipart: func() { completeCalls++ },
	}
	service := newUploadRetryTestService(db, driver)
	initResult, err := service.MultipartInit(&form.FileMultipartInit{
		Hash: testSHA256(content), OriginName: "retry.txt", Size: int64(len(content)), MimeType: "text/plain", IsPublic: global.Yes, PartCount: 1,
	}, 8)
	if err != nil {
		t.Fatalf("multipart init failed: %v", err)
	}
	params := multipartRetryParams(initResult, content)
	if _, err := service.MultipartComplete(params, 8); err == nil {
		t.Fatal("expected temporary hash read error")
	}
	var session model.UploadSession
	if err := db.Where("complete_token_hash = ?", uploadCompleteTokenHash(initResult.CompleteToken)).First(&session).Error; err != nil {
		t.Fatalf("query upload session failed: %v", err)
	}
	if session.Status != model.UploadSessionStatusPending || session.UploadID != "" {
		t.Fatalf("expected retryable ready object marker, got status=%q upload_id=%q", session.Status, session.UploadID)
	}
	if _, err := service.MultipartComplete(params, 8); err != nil {
		t.Fatalf("retry ready multipart object failed: %v", err)
	}
	if completeCalls != 1 {
		t.Fatalf("expected storage multipart completion once, got %d", completeCalls)
	}
}

func TestMultipartCompleteRetriesReadyObjectAfterBusinessFailure(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	content := []byte("retry multipart after validation failure")
	completeCalls := 0
	driver := fakeFileResourceDriver{
		name:                model.StorageDriverAliyunOSS,
		content:             content,
		onCompleteMultipart: func() { completeCalls++ },
	}
	service := newUploadRetryTestService(db, driver)
	initResult, err := service.MultipartInit(&form.FileMultipartInit{
		Hash: testSHA256(content), OriginName: "retry.txt", Size: int64(len(content)), MimeType: "text/plain", IsPublic: global.Yes, PartCount: 1,
	}, 8)
	if err != nil {
		t.Fatalf("multipart init failed: %v", err)
	}
	params := multipartRetryParams(initResult, content)
	params.FileType = "pdf,word"
	if _, err := service.MultipartComplete(params, 8); err == nil {
		t.Fatal("expected invalid file_type")
	}
	params.FileType = "text"
	if _, err := service.MultipartComplete(params, 8); err != nil {
		t.Fatalf("retry after business failure failed: %v", err)
	}
	if completeCalls != 1 {
		t.Fatalf("expected storage multipart completion once, got %d", completeCalls)
	}
}

func TestMultipartCompleteRecoversLostCompletionResponse(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	content := []byte("multipart completion response lost")
	completeCalls := 0
	driver := fakeFileResourceDriver{
		name:                model.StorageDriverAliyunOSS,
		content:             content,
		exists:              true,
		completeErr:         stderrors.New("connection reset after completion"),
		onCompleteMultipart: func() { completeCalls++ },
	}
	service := newUploadRetryTestService(db, driver)
	initResult, err := service.MultipartInit(&form.FileMultipartInit{
		Hash: testSHA256(content), OriginName: "retry.txt", Size: int64(len(content)), MimeType: "text/plain", IsPublic: global.Yes, PartCount: 1,
	}, 8)
	if err != nil {
		t.Fatalf("multipart init failed: %v", err)
	}
	if _, err := service.MultipartComplete(multipartRetryParams(initResult, content), 8); err != nil {
		t.Fatalf("expected existing completed object to recover lost response: %v", err)
	}
	if completeCalls != 1 {
		t.Fatalf("expected one storage completion attempt, got %d", completeCalls)
	}
}

func TestCompleteDirectUploadUsesTrustedFinalETag(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	content := []byte("trusted final etag")
	driver := fakeFileResourceDriver{name: model.StorageDriverAliyunOSS, content: content}
	service := newUploadRetryTestService(db, driver)
	credential, err := service.UploadCredential(&form.FileUploadCredential{
		OriginName: "etag.txt", Size: int64(len(content)), Hash: testSHA256(content), MimeType: "text/plain", IsPublic: global.Yes,
	}, 7)
	if err != nil {
		t.Fatalf("upload credential failed: %v", err)
	}
	result, err := service.CompleteDirectUpload(&form.FileUploadComplete{
		CompleteToken: credential.CompleteToken, OriginName: "etag.txt", Size: uint(len(content)), Hash: testSHA256(content),
		MimeType: "text/plain", IsPublic: global.Yes, Driver: model.StorageDriverAliyunOSS, ETag: "client-spoofed-etag",
	}, 7)
	if err != nil {
		t.Fatalf("complete direct upload failed: %v", err)
	}
	if result.ETag != "final-etag" {
		t.Fatalf("expected trusted final ETag, got %q", result.ETag)
	}
	var object model.UploadFileObject
	if err := db.First(&object, result.FileObjectID).Error; err != nil {
		t.Fatalf("query file object failed: %v", err)
	}
	if object.ETag != "final-etag" {
		t.Fatalf("expected stored trusted final ETag, got %q", object.ETag)
	}
}

func TestDirectCompleteCannotCommitAfterSessionExpiry(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	content := []byte("expires while direct upload finalizes")
	deleted := make([]string, 0, 2)
	driver := fakeFileResourceDriver{
		name:    model.StorageDriverAliyunOSS,
		content: content,
		onPut: func(_, _ string) {
			if err := db.Model(&model.UploadSession{}).Where("upload_type = ?", model.UploadSessionTypeDirect).
				Update("expires_at", time.Now().Add(-time.Minute)).Error; err != nil {
				t.Fatalf("expire direct upload session: %v", err)
			}
		},
		onDelete: func(_, objectKey string) { deleted = append(deleted, objectKey) },
	}
	service := newUploadRetryTestService(db, driver)
	credential, err := service.UploadCredential(&form.FileUploadCredential{
		OriginName: "expired.txt", Size: int64(len(content)), Hash: testSHA256(content), MimeType: "text/plain", IsPublic: global.Yes,
	}, 7)
	if err != nil {
		t.Fatalf("upload credential failed: %v", err)
	}
	if _, err := service.CompleteDirectUpload(&form.FileUploadComplete{
		CompleteToken: credential.CompleteToken, OriginName: "expired.txt", Size: uint(len(content)), Hash: testSHA256(content),
		MimeType: "text/plain", IsPublic: global.Yes, Driver: model.StorageDriverAliyunOSS,
	}, 7); err == nil {
		t.Fatal("expected expired in-flight direct upload to fail")
	}
	var session model.UploadSession
	if err := db.Where("complete_token_hash = ?", uploadCompleteTokenHash(credential.CompleteToken)).First(&session).Error; err != nil {
		t.Fatalf("query upload session failed: %v", err)
	}
	if session.Status != model.UploadSessionStatusExpired {
		t.Fatalf("expected expired session, got %q", session.Status)
	}
	var fileCount int64
	if err := db.Model(&model.UploadFiles{}).Count(&fileCount).Error; err != nil || fileCount != 0 {
		t.Fatalf("expected no file commit after expiry, count=%d err=%v", fileCount, err)
	}
	if len(deleted) < 2 {
		t.Fatalf("expected final and temporary objects cleaned, got %v", deleted)
	}
}

func TestExpiredCompletingMultipartSessionIsAbortedAndCleaned(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	abortCalls := 0
	deleteCalls := 0
	driver := fakeFileResourceDriver{
		name: model.StorageDriverAliyunOSS,
		onAbortMultipart: func(_, _, _ string) {
			abortCalls++
		},
		onDelete: func(_, _ string) { deleteCalls++ },
	}
	service := newUploadRetryTestService(db, driver)
	token, session, err := createUploadSession(db, uploadSessionInput{
		UploadType: model.UploadSessionTypeMultipart, StorageDriver: model.StorageDriverAliyunOSS,
		Bucket: "assets", ObjectKey: "uploads/expired.txt", UploadID: "expired-upload", OwnerUID: 9,
		OriginName: "expired.txt", Size: 1, Hash: testSHA256([]byte("x")), MimeType: "text/plain", IsPublic: global.Yes,
		ExpiresAt: time.Now().Add(-time.Minute),
	})
	if err != nil {
		t.Fatalf("create upload session failed: %v", err)
	}
	if err := db.Model(&model.UploadSession{}).Where("id = ?", session.ID).Update("status", model.UploadSessionStatusCompleting).Error; err != nil {
		t.Fatalf("mark session completing: %v", err)
	}
	if err := service.MultipartAbort(context.Background(), &form.FileMultipartAbort{CompleteToken: token}, 9); err == nil {
		t.Fatal("expected expired session error")
	}
	if err := db.First(session, session.ID).Error; err != nil {
		t.Fatalf("reload upload session failed: %v", err)
	}
	if session.Status != model.UploadSessionStatusExpired || abortCalls != 1 || deleteCalls != 1 {
		t.Fatalf("expected expired multipart cleanup, status=%q aborts=%d deletes=%d", session.Status, abortCalls, deleteCalls)
	}
}

func TestExpiredLegacyReferencedSessionDoesNotDeleteObject(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	deleteCalls := 0
	driver := fakeFileResourceDriver{
		name:     model.StorageDriverAliyunOSS,
		onDelete: func(_, _ string) { deleteCalls++ },
	}
	service := newUploadRetryTestService(db, driver)
	token, session, err := createUploadSession(db, uploadSessionInput{
		UploadType: model.UploadSessionTypeDirect, StorageDriver: model.StorageDriverAliyunOSS,
		Bucket: "assets", ObjectKey: "uploads/referenced.txt", FileObjectID: 17, OwnerUID: 9,
		OriginName: "referenced.txt", Size: 1, Hash: testSHA256([]byte("x")), MimeType: "text/plain", IsPublic: global.Yes,
		ExpiresAt: time.Now().Add(-time.Minute),
	})
	if err != nil {
		t.Fatalf("create upload session failed: %v", err)
	}
	if _, err := service.CompleteDirectUpload(&form.FileUploadComplete{CompleteToken: token}, 9); err == nil {
		t.Fatal("expected expired legacy session error")
	}
	if err := db.First(session, session.ID).Error; err != nil {
		t.Fatalf("reload upload session failed: %v", err)
	}
	if session.Status != model.UploadSessionStatusExpired || deleteCalls != 0 {
		t.Fatalf("expected referenced object preserved, status=%q deletes=%d", session.Status, deleteCalls)
	}
}

func TestMultipartReadyCASFailureDoesNotDeletePossiblyReferencedObject(t *testing.T) {
	db := newFileResourceSQLiteDB(t)
	content := []byte("multipart CAS winner")
	deleteCalls := 0
	driver := fakeFileResourceDriver{
		name:    model.StorageDriverAliyunOSS,
		content: content,
		onCompleteMultipart: func() {
			if err := db.Model(&model.UploadSession{}).Where("upload_type = ?", model.UploadSessionTypeMultipart).
				Updates(map[string]any{"status": model.UploadSessionStatusCompleted, "file_id": 41, "file_object_id": 42}).Error; err != nil {
				t.Fatalf("simulate winning upload completion: %v", err)
			}
		},
		onDelete: func(_, _ string) { deleteCalls++ },
	}
	service := newUploadRetryTestService(db, driver)
	initResult, err := service.MultipartInit(&form.FileMultipartInit{
		Hash: testSHA256(content), OriginName: "retry.txt", Size: int64(len(content)), MimeType: "text/plain", IsPublic: global.Yes, PartCount: 1,
	}, 8)
	if err != nil {
		t.Fatalf("multipart init failed: %v", err)
	}
	if _, err := service.MultipartComplete(multipartRetryParams(initResult, content), 8); err == nil {
		t.Fatal("expected multipart ready CAS loss")
	}
	if deleteCalls != 0 {
		t.Fatalf("expected possibly referenced object to remain, deletes=%d", deleteCalls)
	}
}
