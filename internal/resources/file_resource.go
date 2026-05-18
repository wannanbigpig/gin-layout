package resources

import (
	"strings"

	c "github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/internal/model"
	"github.com/wannanbigpig/gin-layout/internal/pkg/utils"
)

// FileResourceResources 文件资源响应结构。
type FileResourceResources struct {
	ID                uint             `json:"id"`
	FileObjectID      uint             `json:"file_object_id"`
	UID               uint             `json:"uid"`
	FolderID          uint             `json:"folder_id"`
	LogicalPath       string           `json:"logical_path"`
	DisplayName       string           `json:"display_name"`
	OriginName        string           `json:"origin_name"`
	Name              string           `json:"name"`
	Path              string           `json:"path"`
	Size              uint             `json:"size"`
	Ext               string           `json:"ext"`
	Hash              string           `json:"hash"`
	UUID              string           `json:"uuid"`
	MimeType          string           `json:"mime_type"`
	FileType          string           `json:"file_type"`
	IsPublic          uint8            `json:"is_public"`
	StorageDriver     string           `json:"storage_driver"`
	StorageBase       string           `json:"storage_base"`
	Bucket            string           `json:"bucket"`
	StoragePath       string           `json:"storage_path"`
	ObjectKey         string           `json:"object_key"`
	ETag              string           `json:"etag"`
	StorageStatus     string           `json:"storage_status"`
	StorageStatusName string           `json:"storage_status_name"`
	UploadSource      string           `json:"upload_source"`
	UploadSourceName  string           `json:"upload_source_name"`
	UploadScene       string           `json:"upload_scene"`
	UploadStatus      string           `json:"upload_status"`
	UploadStatusName  string           `json:"upload_status_name"`
	ReferenceCount    int64            `json:"reference_count"`
	ObjectReuseCount  int64            `json:"object_reuse_count"`
	ObjectStatus      string           `json:"object_status"`
	ObjectStatusName  string           `json:"object_status_name"`
	URL               string           `json:"url"`
	CreatedAt         utils.FormatDate `json:"created_at"`
	UpdatedAt         utils.FormatDate `json:"updated_at"`
	LastAccessedAt    utils.FormatDate `json:"last_accessed_at"`
	DeletedBy         uint             `json:"deleted_by"`
	DeletedReason     string           `json:"deleted_reason"`
	DeletedAt         uint             `json:"deleted_at"`
	References        any              `json:"references,omitempty"`
}

// FileResourceTransformer 文件资源转换器。
type FileResourceTransformer struct {
	BaseResources[*model.UploadFiles, *FileResourceResources]
}

func NewFileResourceTransformer() FileResourceTransformer {
	return FileResourceTransformer{
		BaseResources: BaseResources[*model.UploadFiles, *FileResourceResources]{
			NewResource: func() *FileResourceResources {
				return &FileResourceResources{}
			},
		},
	}
}

func (r *FileResourceResources) SetCustomFields(data *model.UploadFiles) {
	r.URL = buildFileResourceURL(data.UUID)
	r.DeletedAt = uint(data.DeletedAt)
	if r.DisplayName == "" {
		r.DisplayName = data.OriginName
	}
	r.StorageStatusName = fileStorageStatusName(data.StorageStatus)
	if r.ObjectStatus == "" {
		r.ObjectStatus = data.ObjectStatus
	}
	r.ObjectStatusName = fileStorageStatusName(r.ObjectStatus)
	r.UploadSourceName = fileUploadSourceName(data.UploadSource)
	r.UploadStatusName = fileUploadStatusName(data.UploadStatus)
}

func fileStorageStatusName(status string) string {
	switch status {
	case model.StorageStatusStored:
		return "已存储"
	case model.StorageStatusDeleteFailed:
		return "删除失败"
	case "uploading":
		return "上传中"
	case "missing":
		return "对象缺失"
	default:
		return status
	}
}

func fileUploadSourceName(source string) string {
	switch source {
	case model.UploadSourceBackend:
		return "后端上传"
	case model.UploadSourceDirect:
		return "前端直传"
	case model.UploadSourceSystem:
		return "系统生成"
	default:
		return source
	}
}

func fileUploadStatusName(status string) string {
	switch status {
	case model.UploadStatusPending:
		return "待完成"
	case model.UploadStatusUploaded:
		return "已上传"
	case model.UploadStatusFailed:
		return "上传失败"
	default:
		return status
	}
}

type FileFolderResources struct {
	ID          uint                   `json:"id"`
	ParentID    uint                   `json:"parent_id"`
	Name        string                 `json:"name"`
	LogicalPath string                 `json:"logical_path"`
	Sort        int                    `json:"sort"`
	FileCount   int64                  `json:"file_count"`
	TotalSize   int64                  `json:"total_size"`
	CreatedAt   utils.FormatDate       `json:"created_at"`
	UpdatedAt   utils.FormatDate       `json:"updated_at"`
	Children    []*FileFolderResources `json:"children,omitempty"`
}

type FileFolderTransformer struct {
	BaseResources[*model.UploadFileFolder, *FileFolderResources]
}

func NewFileFolderTransformer() FileFolderTransformer {
	return FileFolderTransformer{
		BaseResources: BaseResources[*model.UploadFileFolder, *FileFolderResources]{
			NewResource: func() *FileFolderResources {
				return &FileFolderResources{}
			},
		},
	}
}

type FileMoveResult struct {
	Total   int64 `json:"total"`
	Moved   int64 `json:"moved"`
	Skipped int64 `json:"skipped"`
}

type FileUploadCredentialResources struct {
	StorageDriver   string            `json:"storage_driver"`
	Driver          string            `json:"driver"`
	Bucket          string            `json:"bucket"`
	ObjectKey       string            `json:"object_key"`
	UploadURL       string            `json:"upload_url"`
	URL             string            `json:"url"`
	Method          string            `json:"method"`
	Headers         map[string]string `json:"headers"`
	ExpireAt        utils.FormatDate  `json:"expire_at"`
	Reuse           bool              `json:"reuse"`
	FileObjectID    uint              `json:"file_object_id"`
	Size            uint              `json:"size"`
	Hash            string            `json:"hash"`
	MimeType        string            `json:"mime_type"`
	ETag            string            `json:"etag"`
	ObjectStatus    string            `json:"object_status"`
	CompletePayload map[string]any    `json:"complete_payload"`
}

type FileReferenceResources struct {
	ID         uint             `json:"id"`
	FileID     uint             `json:"file_id"`
	UUID       string           `json:"uuid"`
	OwnerType  string           `json:"owner_type"`
	OwnerID    uint             `json:"owner_id"`
	OwnerField string           `json:"owner_field"`
	SourceName string           `json:"source_name"`
	FieldName  string           `json:"field_name"`
	CreatedAt  utils.FormatDate `json:"created_at"`
	UpdatedAt  utils.FormatDate `json:"updated_at"`
}

type FileReferenceTransformer struct {
	BaseResources[*model.UploadFileReference, *FileReferenceResources]
}

func NewFileReferenceTransformer() FileReferenceTransformer {
	return FileReferenceTransformer{
		BaseResources: BaseResources[*model.UploadFileReference, *FileReferenceResources]{
			NewResource: func() *FileReferenceResources {
				return &FileReferenceResources{}
			},
		},
	}
}

func (r *FileReferenceResources) SetCustomFields(data *model.UploadFileReference) {
	r.SourceName = fileReferenceSourceName(data.OwnerType)
	r.FieldName = fileReferenceFieldName(data.OwnerField)
}

func fileReferenceSourceName(ownerType string) string {
	switch ownerType {
	case "admin_user":
		return "管理员"
	default:
		return ownerType
	}
}

func fileReferenceFieldName(ownerField string) string {
	switch ownerField {
	case "avatar":
		return "头像"
	default:
		return ownerField
	}
}

func buildFileResourceURL(uuid string) string {
	if uuid == "" {
		return ""
	}
	baseURL := strings.TrimSuffix(c.GetConfig().BaseURL, "/")
	if baseURL == "" {
		return "/admin/v1/file/" + uuid
	}
	return baseURL + "/admin/v1/file/" + uuid
}
