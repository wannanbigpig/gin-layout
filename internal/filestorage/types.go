package filestorage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
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
	// Extra 原位保留未内置到当前版本的供应商配置。序列化时这些键仍位于
	// config 顶层，因此 local / aliyun_oss 的既有 API 契约不变。
	Extra map[string]json.RawMessage `json:"-"`
}

var builtinConfigKeys = map[string]struct{}{
	"local": {}, "aliyun_oss": {}, "signed_url_ttl_seconds": {}, "max_file_size_mb": {},
	"allowed_mime_types": {}, "allowed_extensions": {}, "export_temp_file_ttl_days": {},
}

func (c *Config) UnmarshalJSON(data []byte) error {
	type configAlias Config
	var known configAlias
	if err := json.Unmarshal(data, &known); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for key := range builtinConfigKeys {
		delete(raw, key)
	}
	*c = Config(known)
	if len(raw) > 0 {
		c.Extra = cloneRawConfig(raw)
	}
	return nil
}

func (c Config) MarshalJSON() ([]byte, error) {
	type configAlias Config
	known, err := json.Marshal(configAlias(c))
	if err != nil {
		return nil, err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(known, &object); err != nil {
		return nil, err
	}
	for key, value := range c.Extra {
		if _, reserved := builtinConfigKeys[key]; reserved {
			continue
		}
		if !json.Valid(value) {
			return nil, fmt.Errorf("invalid storage provider config %q", key)
		}
		object[key] = append(json.RawMessage(nil), value...)
	}
	return json.Marshal(object)
}

// DecodeProviderConfig 将未内置供应商的原始配置解码到供应商自己的强类型结构。
func (c Config) DecodeProviderConfig(name string, target any) error {
	raw, ok := c.Extra[strings.TrimSpace(name)]
	if !ok || len(raw) == 0 {
		return errors.New("storage provider config not found: " + strings.TrimSpace(name))
	}
	if target == nil {
		return errors.New("storage provider config target is nil")
	}
	return json.Unmarshal(raw, target)
}

// SetProviderConfig 写入未内置供应商配置，不要求修改 Config 的字段定义。
func (c *Config) SetProviderConfig(name string, value any) error {
	if c == nil {
		return errors.New("storage config is nil")
	}
	name = strings.TrimSpace(name)
	if !validDriverName(name) {
		return errors.New("invalid storage provider name: " + name)
	}
	if _, reserved := builtinConfigKeys[name]; reserved {
		return errors.New("built-in storage provider must use its typed config: " + name)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	extra := cloneRawConfig(c.Extra)
	if extra == nil {
		extra = make(map[string]json.RawMessage)
	}
	extra[name] = raw
	c.Extra = extra
	return nil
}

// ProviderString / SetProviderString 供 SensitiveField 回调安全读写自定义供应商的字符串凭据。
func (c Config) ProviderString(provider, field string) string {
	var object map[string]json.RawMessage
	if err := c.DecodeProviderConfig(provider, &object); err != nil {
		return ""
	}
	var value string
	if err := json.Unmarshal(object[strings.TrimSpace(field)], &value); err != nil {
		return ""
	}
	return value
}

func (c *Config) SetProviderString(provider, field, value string) {
	if c == nil {
		return
	}
	provider = strings.TrimSpace(provider)
	field = strings.TrimSpace(field)
	var object map[string]json.RawMessage
	if err := c.DecodeProviderConfig(provider, &object); err != nil || object == nil {
		object = make(map[string]json.RawMessage)
	}
	encoded, _ := json.Marshal(value)
	object[field] = encoded
	_ = c.SetProviderConfig(provider, object)
}

func cloneRawConfig(source map[string]json.RawMessage) map[string]json.RawMessage {
	cloned := make(map[string]json.RawMessage, len(source))
	for key, value := range source {
		cloned[key] = append(json.RawMessage(nil), value...)
	}
	return cloned
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
}

// PresignedUpload 描述浏览器直传所需的完整请求。不同供应商可以使用不同的
// HTTP 方法、签名请求头或表单字段，业务层不应拼接供应商专属参数。
type PresignedUpload struct {
	URL      string
	Method   string
	Headers  map[string]string
	FormData map[string]string
}

// DirectUploadCapable 描述支持浏览器直传的可选驱动能力。
type DirectUploadCapable interface {
	PresignUpload(ctx context.Context, bucket, objectKey string, contentType string, ttl time.Duration) (PresignedUpload, error)
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

// ImageThumbnailCapable 描述支持由对象存储生成图片缩略图链接的可选能力。
type ImageThumbnailCapable interface {
	SignedImageThumbnailURL(ctx context.Context, bucket, objectKey string, ttl time.Duration) (string, error)
}

// SensitiveField 描述供应商配置中的敏感字段。Get/Set 只操作 Config 的副本，
// 配置服务据此统一完成保留占位符、清空、加密、掩码、查看和审计脱敏。
type SensitiveField struct {
	Name  string
	Label string
	Get   func(Config) string
	Set   func(*Config, string)
}

type DriverDefinition struct {
	Name            string
	Build           func(context.Context, Config) (Driver, error)
	Validate        func(Config) error
	ResolveBucket   func(Config, bool) string
	Capabilities    DriverCapabilities
	SensitiveFields []SensitiveField
}

// DriverCapabilities 是无需实例化驱动即可查询的能力声明，用于请求参数校验和
// 管理端展示。运行时仍以 DirectUploadCapable、MultipartCapable 等接口为准。
type DriverCapabilities struct {
	DirectUpload    bool
	MultipartUpload bool
	ImageThumbnail  bool
}

type Registry struct {
	mu          sync.RWMutex
	definitions map[string]DriverDefinition
}

func NewRegistry() *Registry {
	return &Registry{definitions: make(map[string]DriverDefinition)}
}

func (r *Registry) Register(definition DriverDefinition) error {
	if r == nil {
		return errors.New("storage driver registry is nil")
	}
	name := strings.TrimSpace(definition.Name)
	if !validDriverName(name) || definition.Build == nil || definition.Validate == nil || definition.ResolveBucket == nil {
		return errors.New("storage driver definition is incomplete")
	}
	sensitiveNames := make(map[string]struct{}, len(definition.SensitiveFields))
	for index := range definition.SensitiveFields {
		field := &definition.SensitiveFields[index]
		field.Name = strings.TrimSpace(field.Name)
		field.Label = strings.TrimSpace(field.Label)
		if field.Name == "" || field.Get == nil || field.Set == nil {
			return errors.New("storage driver sensitive field definition is incomplete: " + name)
		}
		if _, exists := sensitiveNames[field.Name]; exists {
			return errors.New("storage driver sensitive field already registered: " + name + "." + field.Name)
		}
		sensitiveNames[field.Name] = struct{}{}
		if field.Label == "" {
			field.Label = field.Name
		}
	}
	definition.Name = name
	definition.SensitiveFields = append([]SensitiveField(nil), definition.SensitiveFields...)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.definitions[name]; exists {
		return errors.New("storage driver already registered: " + name)
	}
	r.definitions[name] = definition
	return nil
}

func (r *Registry) Definition(name string) (DriverDefinition, bool) {
	if r == nil {
		return DriverDefinition{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	definition, ok := r.definitions[strings.TrimSpace(name)]
	definition.SensitiveFields = append([]SensitiveField(nil), definition.SensitiveFields...)
	return definition, ok
}

func validDriverName(name string) bool {
	if len(name) == 0 || len(name) > 20 {
		return false
	}
	for _, char := range name {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '_' {
			return false
		}
	}
	return true
}

func (r *Registry) Names() []string {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	names := make([]string, 0, len(r.definitions))
	for name := range r.definitions {
		names = append(names, name)
	}
	r.mu.RUnlock()
	sort.Strings(names)
	return names
}

func (r *Registry) Build(ctx context.Context, name string, config Config) (Driver, error) {
	definition, ok := r.Definition(name)
	if !ok {
		return nil, errors.New("unsupported storage driver: " + strings.TrimSpace(name))
	}
	if err := definition.Validate(config); err != nil {
		return nil, err
	}
	return definition.Build(ctx, config)
}

func (r *Registry) Validate(name string, config Config) error {
	definition, ok := r.Definition(name)
	if !ok {
		return errors.New("unsupported storage driver: " + strings.TrimSpace(name))
	}
	return definition.Validate(config)
}

func (r *Registry) ResolveBucket(name string, config Config, isPublic bool) (string, error) {
	definition, ok := r.Definition(name)
	if !ok {
		return "", errors.New("unsupported storage driver: " + strings.TrimSpace(name))
	}
	return definition.ResolveBucket(config, isPublic), nil
}

func (r *Registry) SensitiveFields(name string) ([]SensitiveField, error) {
	definition, ok := r.Definition(name)
	if !ok {
		return nil, errors.New("unsupported storage driver: " + strings.TrimSpace(name))
	}
	return append([]SensitiveField(nil), definition.SensitiveFields...), nil
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
