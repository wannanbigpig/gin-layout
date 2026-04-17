package model

type UploadFiles struct {
	BaseModel
	UID        uint   `json:"uid"`         // 用户ID
	OriginName string `json:"origin_name"` // 原始文件名
	Name       string `json:"name"`        // 存储的文件名（UUID+扩展名）
	Path       string `json:"path"`        // 文件相对路径（相对于storage/public或storage/private）
	Size       uint   `json:"size"`        // 文件大小（字节）
	Ext        string `json:"ext"`         // 文件扩展名
	Hash       string `json:"hash"`        // 文件SHA256哈希值（用于去重）
	UUID       string `json:"uuid"`        // 文件UUID（用于URL访问，32位十六进制字符串，不带连字符）
	MimeType   string `json:"mime_type"`   // MIME类型（如：image/jpeg, application/pdf）
	IsPublic   uint8  `json:"is_public"`   // 是否公开访问：0否 1是
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
