package resources

import (
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
)

func TestFileReferenceTransformerAddsDisplayNames(t *testing.T) {
	result := NewFileReferenceTransformer().ToStruct(&model.UploadFileReference{
		FileID:     1,
		OwnerType:  "admin_user",
		OwnerID:    2,
		OwnerField: "avatar",
	})
	if result.SourceName != "管理员" {
		t.Fatalf("expected source_name 管理员, got %q", result.SourceName)
	}
	if result.FieldName != "头像" {
		t.Fatalf("expected field_name 头像, got %q", result.FieldName)
	}
}

func TestFileReferenceTransformerFallsBackToRawNames(t *testing.T) {
	result := NewFileReferenceTransformer().ToStruct(&model.UploadFileReference{
		OwnerType:  "custom_owner",
		OwnerField: "custom_field",
	})
	if result.SourceName != "custom_owner" {
		t.Fatalf("expected raw source_name, got %q", result.SourceName)
	}
	if result.FieldName != "custom_field" {
		t.Fatalf("expected raw field_name, got %q", result.FieldName)
	}
}

func TestFileResourceTransformerAddsStatusDisplayNames(t *testing.T) {
	result := NewFileResourceTransformer().ToStruct(&model.UploadFiles{
		UUID:          "1234567890abcdef1234567890abcdef",
		OriginName:    "avatar.png",
		FileType:      "image",
		MimeType:      "image/png",
		StorageStatus: model.StorageStatusStored,
		UploadSource:  model.UploadSourceBackend,
		UploadStatus:  model.UploadStatusUploaded,
	})
	if result.StorageStatusName != "已存储" {
		t.Fatalf("expected storage_status_name 已存储, got %q", result.StorageStatusName)
	}
	if result.UploadSourceName != "后端上传" {
		t.Fatalf("expected upload_source_name 后端上传, got %q", result.UploadSourceName)
	}
	if result.UploadStatusName != "已上传" {
		t.Fatalf("expected upload_status_name 已上传, got %q", result.UploadStatusName)
	}
	if result.URL != "/admin/v1/file/1234567890abcdef1234567890abcdef" {
		t.Fatalf("expected original url, got %q", result.URL)
	}
	if result.ThumbnailURL != "/admin/v1/file/1234567890abcdef1234567890abcdef?variant=thumb" {
		t.Fatalf("expected thumbnail url, got %q", result.ThumbnailURL)
	}
}

func TestFileResourceTransformerLeavesThumbnailURLBlankForNonImage(t *testing.T) {
	result := NewFileResourceTransformer().ToStruct(&model.UploadFiles{
		UUID:     "abcdef1234567890abcdef1234567890",
		FileType: "pdf",
		MimeType: "application/pdf",
	})
	if result.ThumbnailURL != "" {
		t.Fatalf("expected thumbnail_url empty for non-image, got %q", result.ThumbnailURL)
	}
}

func TestFileUploadBatchFailureCarriesCodeAndMessage(t *testing.T) {
	result := &FileUploadBatchFailure{ClientID: "c1", Code: 11006, Message: "failed"}
	if result.ClientID != "c1" || result.Code != 11006 || result.Message != "failed" {
		t.Fatalf("unexpected batch failure payload: %#v", result)
	}
}
