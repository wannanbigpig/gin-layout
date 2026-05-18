package model

import "github.com/wannanbigpig/gin-layout/internal/pkg/utils"

const (
	StorageDriverLocal     = "local"
	StorageDriverAliyunOSS = "aliyun_oss"
	StorageDriverS3        = "s3"

	StorageStatusStored       = "stored"
	StorageStatusDeleteFailed = "delete_failed"
)

type UploadFiles struct {
	ContainsDeleteBaseModel
	FileObjectID     uint             `json:"file_object_id"`          // 物理对象ID
	UID              uint             `json:"uid"`                     // 用户ID
	FolderID         uint             `json:"folder_id"`               // 逻辑目录ID
	LogicalPath      string           `json:"logical_path"`            // 逻辑路径快照
	DisplayName      string           `json:"display_name"`            // 展示名称
	OriginName       string           `json:"origin_name"`             // 原始文件名
	Name             string           `json:"name"`                    // 存储的文件名（UUID+扩展名）
	Path             string           `json:"path"`                    // 文件相对路径（相对于storage/public或storage/private）
	Size             uint             `json:"size"`                    // 文件大小（字节）
	Ext              string           `json:"ext"`                     // 文件扩展名
	Hash             string           `json:"hash"`                    // 文件SHA256哈希值（用于去重）
	UUID             string           `json:"uuid"`                    // 文件UUID（用于URL访问，32位十六进制字符串，不带连字符）
	MimeType         string           `json:"mime_type"`               // MIME类型（如：image/jpeg, application/pdf）
	FileType         string           `json:"file_type"`               // 文件类型：image,pdf,word,excel,ppt,archive,text,audio,video,other
	IsPublic         uint8            `json:"is_public"`               // 是否公开访问：0否 1是
	StorageDriver    string           `json:"storage_driver"`          // 存储驱动：local,aliyun_oss,s3
	StorageBase      string           `json:"storage_base"`            // 存储基础位置
	Bucket           string           `json:"bucket"`                  // 存储桶
	StoragePath      string           `json:"storage_path"`            // 实际存储路径
	ObjectKey        string           `json:"object_key"`              // 对象 key
	ETag             string           `json:"etag" gorm:"column:etag"` // 对象 ETag
	StorageStatus    string           `json:"storage_status"`          // 存储状态
	UploadSource     string           `json:"upload_source"`           // 上传来源
	UploadScene      string           `json:"upload_scene"`            // 上传场景
	UploadStatus     string           `json:"upload_status"`           // 上传状态
	LastAccessedAt   utils.FormatDate `json:"last_accessed_at"`        // 最后访问时间
	DeletedBy        uint             `json:"deleted_by"`              // 删除人
	DeletedReason    string           `json:"deleted_reason"`          // 删除原因
	ReferenceCount   int64            `json:"reference_count" gorm:"-:all"`
	ObjectReuseCount int64            `json:"object_reuse_count" gorm:"-:all"`
	ObjectStatus     string           `json:"object_status" gorm:"-:all"`
}

func NewUploadFiles() *UploadFiles {
	return BindModel(&UploadFiles{})
}

// TableName 获取表名
func (m *UploadFiles) TableName() string {
	return "upload_files"
}

// Create 创建单条文件记录。
func (m *UploadFiles) Create() error {
	db, err := m.GetDB()
	if err != nil {
		return err
	}
	return db.Create(m).Error
}

const (
	UploadSourceBackend = "backend"
	UploadSourceDirect  = "direct"
	UploadSourceSystem  = "system"

	UploadStatusPending  = "pending"
	UploadStatusUploaded = "uploaded"
	UploadStatusFailed   = "failed"
)

type UploadFileObject struct {
	BaseModel
	StorageDriver string `json:"storage_driver"`
	StorageBase   string `json:"storage_base"`
	Bucket        string `json:"bucket"`
	StoragePath   string `json:"storage_path"`
	ObjectKey     string `json:"object_key"`
	Size          uint   `json:"size"`
	Hash          string `json:"hash"`
	MimeType      string `json:"mime_type"`
	ETag          string `json:"etag" gorm:"column:etag"`
	Status        string `json:"status"`
}

func NewUploadFileObject() *UploadFileObject {
	return BindModel(&UploadFileObject{})
}

func (m *UploadFileObject) TableName() string {
	return "upload_file_objects"
}

type UploadFileFolder struct {
	ContainsDeleteBaseModel
	ParentID    uint   `json:"parent_id"`
	Name        string `json:"name"`
	LogicalPath string `json:"logical_path"`
	Sort        int    `json:"sort"`
	CreatedBy   uint   `json:"created_by"`
	UpdatedBy   uint   `json:"updated_by"`
}

func NewUploadFileFolder() *UploadFileFolder {
	return BindModel(&UploadFileFolder{})
}

func (m *UploadFileFolder) TableName() string {
	return "upload_file_folders"
}

type UploadFileReference struct {
	BaseModel
	FileID     uint   `json:"file_id"`
	UUID       string `json:"uuid"`
	OwnerType  string `json:"owner_type"`
	OwnerID    uint   `json:"owner_id"`
	OwnerField string `json:"owner_field"`
}

func NewUploadFileReference() *UploadFileReference {
	return BindModel(&UploadFileReference{})
}

func (m *UploadFileReference) TableName() string {
	return "upload_file_references"
}
