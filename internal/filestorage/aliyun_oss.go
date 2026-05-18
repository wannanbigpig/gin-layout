package filestorage

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

type AliyunOSSDriver struct {
	client       *oss.Client
	bucket       string
	publicDomain string
}

func NewAliyunOSSDriver(config AliyunOSSConfig) *AliyunOSSDriver {
	endpoint := firstNonEmpty(config.InternalEndpoint, config.Endpoint)
	cfg := oss.LoadDefaultConfig().
		WithRegion(config.Region).
		WithEndpoint(endpoint).
		WithCredentialsProvider(credentials.NewStaticCredentialsProvider(config.AccessKeyID, config.AccessKeySecret)).
		WithUsePathStyle(config.ForcePathStyle)
	return &AliyunOSSDriver{
		client:       oss.NewClient(cfg),
		bucket:       config.Bucket,
		publicDomain: strings.TrimRight(config.PublicDomain, "/"),
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

func (d *AliyunOSSDriver) URL(bucket, objectKey string, isPublic bool) string {
	if !isPublic || d.publicDomain == "" || objectKey == "" {
		return ""
	}
	return d.publicDomain + "/" + strings.TrimLeft(objectKey, "/")
}

func (d *AliyunOSSDriver) SignedURL(ctx context.Context, bucket, objectKey string, ttl time.Duration) (string, error) {
	out, err := d.client.Presign(ctx, &oss.GetObjectRequest{Bucket: oss.Ptr(firstNonEmpty(bucket, d.bucket)), Key: oss.Ptr(objectKey)}, func(options *oss.PresignOptions) {
		options.Expires = ttl
	})
	if err != nil {
		return "", err
	}
	return out.URL, nil
}
