package filestorage

import (
	"context"
	"io"
	"time"
)

const MaskPlaceholder = "******"

type Config struct {
	Local               LocalConfig     `json:"local"`
	AliyunOSS           AliyunOSSConfig `json:"aliyun_oss"`
	SignedURLTTLSeconds int             `json:"signed_url_ttl_seconds"`
	MaxFileSizeMB       int             `json:"max_file_size_mb"`
	AllowedMimeTypes    []string        `json:"allowed_mime_types"`
	AllowedExtensions   []string        `json:"allowed_extensions"`
	ExportTempFileTTL   int             `json:"export_temp_file_ttl_days"`
}

type LocalConfig struct {
	BasePath        string `json:"base_path"`
	PublicBasePath  string `json:"public_base_path"`
	PrivateBasePath string `json:"private_base_path"`
}

type AliyunOSSConfig struct {
	Endpoint         string `json:"endpoint"`
	Region           string `json:"region"`
	Bucket           string `json:"bucket"`
	AccessKeyID      string `json:"access_key_id"`
	AccessKeySecret  string `json:"access_key_secret"`
	PublicDomain     string `json:"public_domain"`
	InternalEndpoint string `json:"internal_endpoint"`
	ForcePathStyle   bool   `json:"force_path_style"`
}


type PutInput struct {
	Bucket      string
	ObjectKey   string
	Reader      io.Reader
	Size        int64
	ContentType string
}

type PutResult struct {
	Bucket    string
	ObjectKey string
	ETag      string
}

// MultipartUploadInput 分片上传初始化参数。
type MultipartUploadInput struct {
	Bucket      string
	ObjectKey   string
	ContentType string
	PartCount   int
}

// MultipartUploadResult 分片上传初始化结果。
type MultipartUploadResult struct {
	UploadID  string
	Bucket    string
	ObjectKey string
}

// MultipartPartInput 分片签名参数。
type MultipartPartInput struct {
	Bucket      string
	ObjectKey   string
	UploadID    string
	PartNumber  int32
	ContentType string
	TTL         time.Duration // 分片签名有效期
}

// MultipartPartResult 分片签名结果。
type MultipartPartResult struct {
	PartNumber int32
	UploadURL  string
	Headers    map[string]string
}

// CompletePartInfo 完成分片信息。
type CompletePartInfo struct {
	PartNumber int32
	ETag       string
}

// Driver 描述所有存储驱动必须实现的基础能力。
type Driver interface {
	Name() string
	Put(ctx context.Context, input PutInput) (PutResult, error)
	Open(ctx context.Context, bucket, objectKey string) (io.ReadCloser, error)
	Exists(ctx context.Context, bucket, objectKey string) (bool, error)
	Delete(ctx context.Context, bucket, objectKey string) error
	URL(bucket, objectKey string, isPublic bool) string
	SignedURL(ctx context.Context, bucket, objectKey string, ttl time.Duration) (string, error)
	PresignPutObject(ctx context.Context, bucket, objectKey string, contentType string, ttl time.Duration) (string, error)
}

// ObjectStat 对象元数据。
type ObjectStat struct {
	Size        int64
	ContentType string
}

// StatCapable 支持查询对象元数据的驱动。
// 调用方通过类型断言判断驱动是否支持，否则视为不支持。
type StatCapable interface {
	Stat(ctx context.Context, bucket, objectKey string) (ObjectStat, error)
}

// MultipartCapable 描述支持分片上传的可选驱动能力。
// 调用方通过类型断言判断驱动是否支持，否则视为不支持。
type MultipartCapable interface {
	InitMultipartUpload(ctx context.Context, input MultipartUploadInput) (MultipartUploadResult, error)
	PresignMultipartPart(ctx context.Context, input MultipartPartInput) (MultipartPartResult, error)
	CompleteMultipartUpload(ctx context.Context, bucket, objectKey, uploadID string, parts []CompletePartInfo) error
	AbortMultipartUpload(ctx context.Context, bucket, objectKey, uploadID string) error
}

func DefaultConfig() Config {
	return Config{
		SignedURLTTLSeconds: 300,
		MaxFileSizeMB:       10,
		AllowedExtensions: []string{
			".jpg", ".jpeg", ".png", ".gif", ".webp",
			".pdf", ".txt", ".csv", ".xlsx", ".docx",
		},
		AllowedMimeTypes: []string{
			"image/jpeg", "image/png", "image/gif", "image/webp",
			"application/pdf", "text/plain", "text/csv",
			"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		},
		ExportTempFileTTL: 7,
	}
}
