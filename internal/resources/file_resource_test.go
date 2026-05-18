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
		OriginName:    "avatar.png",
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
}
