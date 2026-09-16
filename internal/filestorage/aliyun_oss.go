package filestorage

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

const (
	aliyunOSSThumbnailProcess = "image/resize,m_lfit,w_320,h_320"
)

type AliyunOSSDriver struct {
	client        *oss.Client
	signingClient *oss.Client
	bucket        string
	publicDomain  string
}

func NewAliyunOSSDriver(config AliyunOSSConfig) *AliyunOSSDriver {
	endpoint := firstNonEmpty(config.InternalEndpoint, config.Endpoint)
	cfg := oss.LoadDefaultConfig().
		WithRegion(config.Region).
		WithEndpoint(endpoint).
		WithCredentialsProvider(credentials.NewStaticCredentialsProvider(config.AccessKeyID, config.AccessKeySecret))
	signingCfg := oss.LoadDefaultConfig().
		WithRegion(config.Region).
		WithEndpoint(config.Endpoint).
		WithCredentialsProvider(credentials.NewStaticCredentialsProvider(config.AccessKeyID, config.AccessKeySecret))
	return &AliyunOSSDriver{
		client:        oss.NewClient(cfg),
		signingClient: oss.NewClient(signingCfg),
		bucket:        config.Bucket,
		publicDomain:  strings.TrimRight(config.PublicDomain, "/"),
	}
}

func (d *AliyunOSSDriver) Name() string { return "aliyun_oss" }

func (d *AliyunOSSDriver) Put(ctx context.Context, input PutInput) (PutResult, error) {
	bucket := firstNonEmpty(input.Bucket, d.bucket)
	out, err := d.client.PutObject(ctx, &oss.PutObjectRequest{
		Bucket:        oss.Ptr(bucket),
		Key:           oss.Ptr(input.ObjectKey),
		Body:          input.Reader,
		ContentLength: oss.Ptr(input.Size),
		ContentType:   oss.Ptr(input.ContentType),
	})
	if err != nil {
		return PutResult{}, err
	}
	return PutResult{Bucket: bucket, ObjectKey: input.ObjectKey, ETag: strings.Trim(oss.ToString(out.ETag), "\"")}, nil
}

func (d *AliyunOSSDriver) Open(ctx context.Context, bucket, objectKey string) (io.ReadCloser, error) {
	out, err := d.client.GetObject(ctx, &oss.GetObjectRequest{Bucket: oss.Ptr(firstNonEmpty(bucket, d.bucket)), Key: oss.Ptr(objectKey)})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

func (d *AliyunOSSDriver) Exists(ctx context.Context, bucket, objectKey string) (bool, error) {
	return d.client.IsObjectExist(ctx, firstNonEmpty(bucket, d.bucket), objectKey)
}

func (d *AliyunOSSDriver) Delete(ctx context.Context, bucket, objectKey string) error {
	_, err := d.client.DeleteObject(ctx, &oss.DeleteObjectRequest{Bucket: oss.Ptr(firstNonEmpty(bucket, d.bucket)), Key: oss.Ptr(objectKey)})
	return err
}

func (d *AliyunOSSDriver) Stat(ctx context.Context, bucket, objectKey string) (ObjectStat, error) {
	out, err := d.client.HeadObject(ctx, &oss.HeadObjectRequest{
		Bucket: oss.Ptr(firstNonEmpty(bucket, d.bucket)),
		Key:    oss.Ptr(objectKey),
	})
	if err != nil {
		return ObjectStat{}, err
	}
	stat := ObjectStat{Size: out.ContentLength}
	if out.ContentType != nil {
		stat.ContentType = *out.ContentType
	}
	return stat, nil
}

func (d *AliyunOSSDriver) URL(bucket, objectKey string, isPublic bool) string {
	if !isPublic || d.publicDomain == "" || objectKey == "" {
		return ""
	}
	return d.publicDomain + "/" + strings.TrimLeft(objectKey, "/")
}

func (d *AliyunOSSDriver) SignedURL(ctx context.Context, bucket, objectKey string, ttl time.Duration) (string, error) {
	out, err := d.signingClient.Presign(ctx, &oss.GetObjectRequest{Bucket: oss.Ptr(firstNonEmpty(bucket, d.bucket)), Key: oss.Ptr(objectKey)}, func(options *oss.PresignOptions) {
		options.Expires = ttl
	})
	if err != nil {
		return "", err
	}
	return out.URL, nil
}

func (d *AliyunOSSDriver) SignedImageThumbnailURL(ctx context.Context, bucket, objectKey string, ttl time.Duration) (string, error) {
	request := &oss.GetObjectRequest{
		Bucket:  oss.Ptr(firstNonEmpty(bucket, d.bucket)),
		Key:     oss.Ptr(objectKey),
		Process: oss.Ptr(aliyunOSSThumbnailProcess),
	}
	out, err := d.signingClient.Presign(ctx, request, func(options *oss.PresignOptions) {
		options.Expires = ttl
	})
	if err != nil {
		return "", err
	}
	return out.URL, nil
}

func (d *AliyunOSSDriver) PresignPutObject(ctx context.Context, bucket, objectKey string, contentType string, ttl time.Duration) (string, error) {
	request := &oss.PutObjectRequest{
		Bucket: oss.Ptr(firstNonEmpty(bucket, d.bucket)),
		Key:    oss.Ptr(objectKey),
	}
	if contentType != "" {
		request.ContentType = oss.Ptr(contentType)
	}
	result, err := d.signingClient.Presign(ctx, request, func(opts *oss.PresignOptions) {
		opts.Expires = ttl
	})
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

func (d *AliyunOSSDriver) InitMultipartUpload(ctx context.Context, input MultipartUploadInput) (MultipartUploadResult, error) {
	bucket := firstNonEmpty(input.Bucket, d.bucket)
	out, err := d.client.InitiateMultipartUpload(ctx, &oss.InitiateMultipartUploadRequest{
		Bucket:      oss.Ptr(bucket),
		Key:         oss.Ptr(input.ObjectKey),
		ContentType: oss.Ptr(input.ContentType),
	})
	if err != nil {
		return MultipartUploadResult{}, err
	}
	return MultipartUploadResult{
		UploadID:  oss.ToString(out.UploadId),
		Bucket:    bucket,
		ObjectKey: input.ObjectKey,
	}, nil
}

func (d *AliyunOSSDriver) PresignMultipartPart(ctx context.Context, input MultipartPartInput) (MultipartPartResult, error) {
	bucket := firstNonEmpty(input.Bucket, d.bucket)
	ttl := input.TTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	result, err := d.signingClient.Presign(ctx, &oss.UploadPartRequest{
		Bucket:     oss.Ptr(bucket),
		Key:        oss.Ptr(input.ObjectKey),
		UploadId:   oss.Ptr(input.UploadID),
		PartNumber: input.PartNumber,
	}, func(opts *oss.PresignOptions) {
		opts.Expires = ttl
	})
	if err != nil {
		return MultipartPartResult{}, err
	}
	return MultipartPartResult{
		PartNumber: input.PartNumber,
		UploadURL:  result.URL,
		Headers:    nil,
	}, nil
}

func (d *AliyunOSSDriver) CompleteMultipartUpload(ctx context.Context, bucket, objectKey, uploadID string, parts []CompletePartInfo) error {
	if len(parts) == 0 {
		return fmt.Errorf("parts is required")
	}
	bucket = firstNonEmpty(bucket, d.bucket)
	sortedParts := make([]oss.UploadPart, len(parts))
	for i, p := range parts {
		sortedParts[i] = oss.UploadPart{
			PartNumber: p.PartNumber,
			ETag:       oss.Ptr(p.ETag),
		}
	}
	sort.Slice(sortedParts, func(i, j int) bool {
		return sortedParts[i].PartNumber < sortedParts[j].PartNumber
	})
	_, err := d.client.CompleteMultipartUpload(ctx, &oss.CompleteMultipartUploadRequest{
		Bucket:   oss.Ptr(bucket),
		Key:      oss.Ptr(objectKey),
		UploadId: oss.Ptr(uploadID),
		CompleteMultipartUpload: &oss.CompleteMultipartUpload{
			Parts: sortedParts,
		},
	})
	return err
}

func (d *AliyunOSSDriver) AbortMultipartUpload(ctx context.Context, bucket, objectKey, uploadID string) error {
	bucket = firstNonEmpty(bucket, d.bucket)
	_, err := d.client.AbortMultipartUpload(ctx, &oss.AbortMultipartUploadRequest{
		Bucket:   oss.Ptr(bucket),
		Key:      oss.Ptr(objectKey),
		UploadId: oss.Ptr(uploadID),
	})
	return err
}
