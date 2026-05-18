package form

// FileResourceList 文件资源列表查询参数。
type FileResourceList struct {
	Paginate
	OriginName       string `form:"origin_name" json:"origin_name" binding:"omitempty"`
	UUID             string `form:"uuid" json:"uuid" binding:"omitempty"`
	MimeType         string `form:"mime_type" json:"mime_type" binding:"omitempty"`
	FileType         string `form:"file_type" json:"file_type" binding:"omitempty,oneof=image pdf word excel ppt archive text audio video"`
	IsPublic         *uint8 `form:"is_public" json:"is_public" binding:"omitempty,oneof=0 1"`
	FolderID         *uint  `form:"folder_id" json:"folder_id" binding:"omitempty"`
	IncludeSubfolder uint8  `form:"include_subfolder" json:"include_subfolder" binding:"omitempty,oneof=0 1"`
	StorageDriver    string `form:"storage_driver" json:"storage_driver" binding:"omitempty,oneof=local aliyun_oss s3"`
	StorageStatus    string `form:"storage_status" json:"storage_status" binding:"omitempty,oneof=stored delete_failed"`
	IsReferenced     *uint8 `form:"is_referenced" json:"is_referenced" binding:"omitempty,oneof=0 1"`
	IsDeleted        *uint8 `form:"is_deleted" json:"is_deleted" binding:"omitempty,oneof=0 1"`
	UID              uint   `form:"uid" json:"uid" binding:"omitempty,gt=0"`
	StartTime        string `form:"start_time" json:"start_time" binding:"omitempty"`
	EndTime          string `form:"end_time" json:"end_time" binding:"omitempty"`
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
	Driver      string `form:"driver" json:"driver" binding:"omitempty,oneof=local aliyun_oss s3"`
}

func NewFileUploadCredentialForm() *FileUploadCredential {
	return &FileUploadCredential{IsPublic: 1}
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
	StorageDriver string `form:"storage_driver" json:"storage_driver" binding:"omitempty,oneof=aliyun_oss s3"`
	Driver        string `form:"driver" json:"driver" binding:"omitempty,oneof=aliyun_oss s3"`
	Bucket        string `form:"bucket" json:"bucket" binding:"omitempty,max=128"`
	ObjectKey     string `form:"object_key" json:"object_key" binding:"omitempty,max=512"`
	ETag          string `form:"etag" json:"etag" binding:"omitempty,max=128"`
	UploadScene   string `form:"upload_scene" json:"upload_scene" binding:"omitempty,max=60"`
}

func NewFileUploadCompleteForm() *FileUploadComplete {
	return &FileUploadComplete{IsPublic: 1}
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
