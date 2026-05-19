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
	S3                  S3Config        `json:"s3"`
	SignedURLTTLSeconds int             `json:"signed_url_ttl_seconds"`
	MaxFileSizeMB       int             `json:"max_file_size_mb"`
	AllowedMimeTypes    []string        `json:"allowed_mime_types"`
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

type S3Config struct {
	Endpoint        string `json:"endpoint"`
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	PublicDomain    string `json:"public_domain"`
	ForcePathStyle  bool   `json:"force_path_style"`
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

type Driver interface {
	Name() string
	Put(ctx context.Context, input PutInput) (PutResult, error)
	Open(ctx context.Context, bucket, objectKey string) (io.ReadCloser, error)
	Exists(ctx context.Context, bucket, objectKey string) (bool, error)
	Delete(ctx context.Context, bucket, objectKey string) error
	URL(bucket, objectKey string, isPublic bool) string
	SignedURL(ctx context.Context, bucket, objectKey string, ttl time.Duration) (string, error)
	PresignPutObject(ctx context.Context, bucket, objectKey string, contentType string, ttl time.Duration) (string, error)
	InitMultipartUpload(ctx context.Context, input MultipartUploadInput) (MultipartUploadResult, error)
	PresignMultipartPart(ctx context.Context, input MultipartPartInput) (MultipartPartResult, error)
	CompleteMultipartUpload(ctx context.Context, bucket, objectKey, uploadID string, parts []CompletePartInfo) error
	AbortMultipartUpload(ctx context.Context, bucket, objectKey, uploadID string) error
}

func DefaultConfig() Config {
	return Config{
		SignedURLTTLSeconds: 300,
		MaxFileSizeMB:       10,
		AllowedMimeTypes:    []string{},
	}
}
