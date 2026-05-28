package form

// FileResourceList 文件资源列表查询参数。
type FileResourceList struct {
	Paginate
	OriginName       string `form:"origin_name" json:"origin_name" binding:"omitempty"`
	UUID             string `form:"uuid" json:"uuid" binding:"omitempty"`
	MimeType         string `form:"mime_type" json:"mime_type" binding:"omitempty"`
	FileType         string `form:"file_type" json:"file_type" binding:"omitempty,file_type_or_multi"`
	IsPublic         *uint8 `form:"is_public" json:"is_public" binding:"omitempty,oneof=0 1"`
	FolderID         *uint  `form:"folder_id" json:"folder_id" binding:"omitempty"`
	IncludeSubfolder uint8  `form:"include_subfolder" json:"include_subfolder" binding:"omitempty,oneof=0 1"`
	StorageDriver    string `form:"storage_driver" json:"storage_driver" binding:"omitempty,oneof=local aliyun_oss"`
	StorageStatus    string `form:"storage_status" json:"storage_status" binding:"omitempty,oneof=stored delete_failed"`
	IsReferenced     *uint8 `form:"is_referenced" json:"is_referenced" binding:"omitempty,oneof=0 1"`
	IsDeleted        *uint8 `form:"is_deleted" json:"is_deleted" binding:"omitempty,oneof=0 1"`
	UID              uint   `form:"uid" json:"uid" binding:"omitempty,gt=0"`
	StartTime        string `form:"start_time" json:"start_time" binding:"omitempty"`
	EndTime          string `form:"end_time" json:"end_time" binding:"omitempty"`
	// Limit 仅在导出场景生效，控制单次导出最多记录数。
	Limit int `form:"limit" json:"limit" binding:"omitempty,min=1,max=50000"`
}

func NewFileResourceListQuery() *FileResourceList {
	return &FileResourceList{}
}

// FileResourceID 文件资源 ID 参数。
type FileResourceID struct {
	ID            uint   `form:"id" json:"id" binding:"required,gt=0"`
	DeletedReason string `form:"deleted_reason" json:"deleted_reason" binding:"omitempty,max=255"`
}

func NewFileResourceIDForm() *FileResourceID {
	return &FileResourceID{}
}

type FileBatchDelete struct {
	IDs           []uint `form:"ids" json:"ids" binding:"required,min=1,dive,gt=0"`
	DeletedReason string `form:"deleted_reason" json:"deleted_reason" binding:"omitempty,max=255"`
}

func NewFileBatchDeleteForm() *FileBatchDelete {
	return &FileBatchDelete{}
}

type FileFolderCreate struct {
	ParentID uint   `form:"parent_id" json:"parent_id" binding:"omitempty"`
	Name     string `form:"name" json:"name" binding:"required,max=120"`
}

func NewFileFolderCreateForm() *FileFolderCreate {
	return &FileFolderCreate{}
}

type FileFolderUpdate struct {
	ID   uint   `form:"id" json:"id" binding:"required,gt=0"`
	Name string `form:"name" json:"name" binding:"required,max=120"`
}

func NewFileFolderUpdateForm() *FileFolderUpdate {
	return &FileFolderUpdate{}
}

type FileFolderDelete struct {
	ID uint `form:"id" json:"id" binding:"required,gt=0"`
}

func NewFileFolderDeleteForm() *FileFolderDelete {
	return &FileFolderDelete{}
}

type FileFolderMove struct {
	ID             uint `form:"id" json:"id" binding:"required,gt=0"`
	ParentID       uint `form:"parent_id" json:"parent_id" binding:"omitempty"`
	TargetParentID uint `form:"target_parent_id" json:"target_parent_id" binding:"omitempty"`
}

func NewFileFolderMoveForm() *FileFolderMove {
	return &FileFolderMove{}
}

type FileMove struct {
	IDs      []uint `form:"ids" json:"ids" binding:"required,dive,gt=0"`
	FolderID uint   `form:"folder_id" json:"folder_id" binding:"omitempty"`
}

func NewFileMoveForm() *FileMove {
	return &FileMove{}
}

type FileLocalUpload struct {
	FolderID    uint   `form:"folder_id" json:"folder_id" binding:"omitempty"`
	IsPublic    uint8  `form:"is_public" json:"is_public" binding:"omitempty,oneof=0 1"`
	UploadScene string `form:"upload_scene" json:"upload_scene" binding:"omitempty,max=60"`
}

func NewFileLocalUploadForm() *FileLocalUpload {
	return &FileLocalUpload{IsPublic: 1}
}

type FileUploadCredential struct {
	FolderID    uint   `form:"folder_id" json:"folder_id" binding:"omitempty"`
	FileName    string `form:"file_name" json:"file_name" binding:"omitempty,max=255"`
	OriginName  string `form:"origin_name" json:"origin_name" binding:"omitempty,max=255"`
	MimeType    string `form:"mime_type" json:"mime_type" binding:"omitempty,max=100"`
	Size        int64  `form:"size" json:"size" binding:"omitempty,gte=0"`
	Hash        string `form:"hash" json:"hash" binding:"omitempty,len=64"`
	IsPublic    uint8  `form:"is_public" json:"is_public" binding:"omitempty,oneof=0 1"`
	UploadScene string `form:"upload_scene" json:"upload_scene" binding:"omitempty,max=60"`
	Driver      string `form:"driver" json:"driver" binding:"omitempty,oneof=local aliyun_oss"`
}

func NewFileUploadCredentialForm() *FileUploadCredential {
	return &FileUploadCredential{IsPublic: 1}
}

type FileUploadCredentialBatch struct {
	IsPublic    uint8                           `form:"is_public" json:"is_public" binding:"omitempty,oneof=0 1"`
	UploadScene string                          `form:"upload_scene" json:"upload_scene" binding:"omitempty,max=60"`
	Driver      string                          `form:"driver" json:"driver" binding:"omitempty,oneof=local aliyun_oss"`
	Items       []FileUploadCredentialBatchItem `form:"items" json:"items" binding:"required,min=1,dive"`
}

type FileUploadCredentialBatchItem struct {
	ClientID    string `form:"client_id" json:"client_id" binding:"required,max=100"`
	FolderID    uint   `form:"folder_id" json:"folder_id" binding:"omitempty"`
	FileName    string `form:"file_name" json:"file_name" binding:"omitempty,max=255"`
	OriginName  string `form:"origin_name" json:"origin_name" binding:"required,max=255"`
	MimeType    string `form:"mime_type" json:"mime_type" binding:"omitempty,max=100"`
	Size        int64  `form:"size" json:"size" binding:"required,gte=0"`
	Hash        string `form:"hash" json:"hash" binding:"required,len=64"`
	IsPublic    *uint8 `form:"is_public" json:"is_public" binding:"omitempty,oneof=0 1"`
	UploadScene string `form:"upload_scene" json:"upload_scene" binding:"omitempty,max=60"`
	Driver      string `form:"driver" json:"driver" binding:"omitempty,oneof=local aliyun_oss"`
}

func NewFileUploadCredentialBatchForm() *FileUploadCredentialBatch {
	return &FileUploadCredentialBatch{IsPublic: 1}
}

type FileUploadComplete struct {
	FolderID      uint   `form:"folder_id" json:"folder_id" binding:"omitempty"`
	Reuse         bool   `form:"reuse" json:"reuse" binding:"omitempty"`
	FileObjectID  uint   `form:"file_object_id" json:"file_object_id" binding:"omitempty,gt=0"`
	OriginName    string `form:"origin_name" json:"origin_name" binding:"required,max=255"`
	DisplayName   string `form:"display_name" json:"display_name" binding:"omitempty,max=255"`
	Name          string `form:"name" json:"name" binding:"omitempty,max=255"`
	Size          uint   `form:"size" json:"size" binding:"omitempty"`
	Ext           string `form:"ext" json:"ext" binding:"omitempty,max=20"`
	Hash          string `form:"hash" json:"hash" binding:"omitempty,max=64"`
	UUID          string `form:"uuid" json:"uuid" binding:"omitempty,len=32"`
	MimeType      string `form:"mime_type" json:"mime_type" binding:"omitempty,max=100"`
	FileType      string `form:"file_type" json:"file_type" binding:"omitempty,oneof=image pdf word excel ppt archive text audio video other"`
	IsPublic      uint8  `form:"is_public" json:"is_public" binding:"omitempty,oneof=0 1"`
	StorageDriver string `form:"storage_driver" json:"storage_driver" binding:"omitempty,oneof=aliyun_oss"`
	Driver        string `form:"driver" json:"driver" binding:"omitempty,oneof=aliyun_oss"`
	Bucket        string `form:"bucket" json:"bucket" binding:"omitempty,max=128"`
	ObjectKey     string `form:"object_key" json:"object_key" binding:"omitempty,max=512"`
	ETag          string `form:"etag" json:"etag" binding:"omitempty,max=128"`
	UploadScene   string `form:"upload_scene" json:"upload_scene" binding:"omitempty,max=60"`
}

func NewFileUploadCompleteForm() *FileUploadComplete {
	return &FileUploadComplete{IsPublic: 1}
}

type FileUploadCompleteBatch struct {
	IsPublic    uint8                         `form:"is_public" json:"is_public" binding:"omitempty,oneof=0 1"`
	UploadScene string                        `form:"upload_scene" json:"upload_scene" binding:"omitempty,max=60"`
	Driver      string                        `form:"driver" json:"driver" binding:"omitempty,oneof=aliyun_oss"`
	Items       []FileUploadCompleteBatchItem `form:"items" json:"items" binding:"required,min=1,dive"`
}

type FileUploadCompleteBatchItem struct {
	ClientID      string `form:"client_id" json:"client_id" binding:"required,max=100"`
	FolderID      uint   `form:"folder_id" json:"folder_id" binding:"omitempty"`
	Reuse         bool   `form:"reuse" json:"reuse" binding:"omitempty"`
	FileObjectID  uint   `form:"file_object_id" json:"file_object_id" binding:"omitempty,gt=0"`
	OriginName    string `form:"origin_name" json:"origin_name" binding:"required,max=255"`
	DisplayName   string `form:"display_name" json:"display_name" binding:"omitempty,max=255"`
	Name          string `form:"name" json:"name" binding:"omitempty,max=255"`
	Size          uint   `form:"size" json:"size" binding:"omitempty"`
	Ext           string `form:"ext" json:"ext" binding:"omitempty,max=20"`
	Hash          string `form:"hash" json:"hash" binding:"omitempty,max=64"`
	UUID          string `form:"uuid" json:"uuid" binding:"omitempty,len=32"`
	MimeType      string `form:"mime_type" json:"mime_type" binding:"omitempty,max=100"`
	FileType      string `form:"file_type" json:"file_type" binding:"omitempty,oneof=image pdf word excel ppt archive text audio video other"`
	IsPublic      *uint8 `form:"is_public" json:"is_public" binding:"omitempty,oneof=0 1"`
	StorageDriver string `form:"storage_driver" json:"storage_driver" binding:"omitempty,oneof=aliyun_oss"`
	Driver        string `form:"driver" json:"driver" binding:"omitempty,oneof=aliyun_oss"`
	Bucket        string `form:"bucket" json:"bucket" binding:"omitempty,max=128"`
	ObjectKey     string `form:"object_key" json:"object_key" binding:"omitempty,max=512"`
	ETag          string `form:"etag" json:"etag" binding:"omitempty,max=128"`
	UploadScene   string `form:"upload_scene" json:"upload_scene" binding:"omitempty,max=60"`
}

func NewFileUploadCompleteBatchForm() *FileUploadCompleteBatch {
	return &FileUploadCompleteBatch{IsPublic: 1}
}

type FileReferenceList struct {
	Paginate
	ID         uint   `form:"id" json:"id" binding:"omitempty,gt=0"`
	FileID     uint   `form:"file_id" json:"file_id" binding:"omitempty,gt=0"`
	UUID       string `form:"uuid" json:"uuid" binding:"omitempty"`
	OwnerType  string `form:"owner_type" json:"owner_type" binding:"omitempty,max=60"`
	OwnerID    uint   `form:"owner_id" json:"owner_id" binding:"omitempty,gt=0"`
	OwnerField string `form:"owner_field" json:"owner_field" binding:"omitempty,max=60"`
}

func NewFileReferenceListQuery() *FileReferenceList {
	return &FileReferenceList{}
}

// FileMultipartInit 分片上传初始化参数。
type FileMultipartInit struct {
	Hash       string `form:"hash" json:"hash" binding:"required,len=64"`
	OriginName string `form:"origin_name" json:"origin_name" binding:"required,max=255"`
	Size       int64  `form:"size" json:"size" binding:"required,gt=0"`
	MimeType   string `form:"mime_type" json:"mime_type" binding:"omitempty,max=100"`
	FolderID   uint   `form:"folder_id" json:"folder_id" binding:"omitempty"`
	IsPublic   uint8  `form:"is_public" json:"is_public" binding:"omitempty,oneof=0 1"`
	Driver     string `form:"driver" json:"driver" binding:"omitempty,oneof=aliyun_oss"`
	ChunkSize  int    `form:"chunk_size" json:"chunk_size" binding:"omitempty,gt=0"`
	PartCount  int    `form:"part_count" json:"part_count" binding:"omitempty,gt=0"`
}

func NewFileMultipartInitForm() *FileMultipartInit {
	return &FileMultipartInit{IsPublic: 1}
}

// FileMultipartComplete 分片上传完成参数。
type FileMultipartComplete struct {
	UploadID     string                  `form:"upload_id" json:"upload_id" binding:"omitempty"`
	Reuse        bool                    `form:"reuse" json:"reuse" binding:"omitempty"`
	FileObjectID uint                    `form:"file_object_id" json:"file_object_id" binding:"omitempty,gt=0"`
	Bucket       string                  `form:"bucket" json:"bucket" binding:"omitempty"`
	ObjectKey    string                  `form:"object_key" json:"object_key" binding:"omitempty"`
	OriginName   string                  `form:"origin_name" json:"origin_name" binding:"required,max=255"`
	Size         uint                    `form:"size" json:"size" binding:"omitempty"`
	Hash         string                  `form:"hash" json:"hash" binding:"omitempty,max=64"`
	MimeType     string                  `form:"mime_type" json:"mime_type" binding:"omitempty,max=100"`
	FileType     string                  `form:"file_type" json:"file_type" binding:"omitempty,oneof=image pdf word excel ppt archive text audio video other"`
	FolderID     uint                    `form:"folder_id" json:"folder_id" binding:"omitempty"`
	IsPublic     uint8                   `form:"is_public" json:"is_public" binding:"omitempty,oneof=0 1"`
	Driver       string                  `form:"driver" json:"driver" binding:"omitempty,oneof=aliyun_oss"`
	Parts        []MultipartCompletePart `form:"parts" json:"parts" binding:"omitempty,dive"`
}

// MultipartCompletePart 分片完成信息。
type MultipartCompletePart struct {
	PartNumber int32  `json:"part_number" binding:"required,gt=0"`
	ETag       string `json:"etag" binding:"required"`
}

func NewFileMultipartCompleteForm() *FileMultipartComplete {
	return &FileMultipartComplete{IsPublic: 1}
}

// FileMultipartAbort 分片上传中止参数。
type FileMultipartAbort struct {
	UploadID  string `form:"upload_id" json:"upload_id" binding:"required"`
	Bucket    string `form:"bucket" json:"bucket" binding:"required"`
	ObjectKey string `form:"object_key" json:"object_key" binding:"required"`
	Driver    string `form:"driver" json:"driver" binding:"omitempty,oneof=aliyun_oss"`
}

func NewFileMultipartAbortForm() *FileMultipartAbort {
	return &FileMultipartAbort{}
}

type FileBatchRestore struct {
	IDs []uint `form:"ids" json:"ids" binding:"required,min=1,dive,gt=0"`
}

func NewFileBatchRestoreForm() *FileBatchRestore {
	return &FileBatchRestore{}
}

type FileBatchDestroy struct {
	IDs []uint `form:"ids" json:"ids" binding:"required,min=1,dive,gt=0"`
}

func NewFileBatchDestroyForm() *FileBatchDestroy {
	return &FileBatchDestroy{}
}
