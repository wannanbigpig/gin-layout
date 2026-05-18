package filestorage

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Driver struct {
	client       *s3.Client
	presign      *s3.PresignClient
	bucket       string
	publicDomain string
}

func NewS3Driver(ctx context.Context, config S3Config) (*S3Driver, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(config.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(config.AccessKeyID, config.SecretAccessKey, "")),
	)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if config.Endpoint != "" {
			o.BaseEndpoint = aws.String(config.Endpoint)
		}
		o.UsePathStyle = config.ForcePathStyle
	})
	return &S3Driver{
		client:       client,
		presign:      s3.NewPresignClient(client),
		bucket:       config.Bucket,
		publicDomain: strings.TrimRight(config.PublicDomain, "/"),
	}, nil
}

func (d *S3Driver) Name() string { return "s3" }

func (d *S3Driver) Put(ctx context.Context, input PutInput) (PutResult, error) {
	bucket := firstNonEmpty(input.Bucket, d.bucket)
	out, err := d.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(input.ObjectKey),
		Body:          input.Reader,
		ContentLength: aws.Int64(input.Size),
		ContentType:   aws.String(input.ContentType),
	})
	if err != nil {
		return PutResult{}, err
	}
	return PutResult{Bucket: bucket, ObjectKey: input.ObjectKey, ETag: strings.Trim(aws.ToString(out.ETag), "\"")}, nil
}

func (d *S3Driver) Open(ctx context.Context, bucket, objectKey string) (io.ReadCloser, error) {
	out, err := d.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(firstNonEmpty(bucket, d.bucket)), Key: aws.String(objectKey)})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

func (d *S3Driver) Exists(ctx context.Context, bucket, objectKey string) (bool, error) {
	_, err := d.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(firstNonEmpty(bucket, d.bucket)), Key: aws.String(objectKey)})
	if err != nil {
		return false, err
	}
	return true, nil
}

func (d *S3Driver) Delete(ctx context.Context, bucket, objectKey string) error {
	_, err := d.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(firstNonEmpty(bucket, d.bucket)), Key: aws.String(objectKey)})
	return err
}

func (d *S3Driver) URL(bucket, objectKey string, isPublic bool) string {
	if !isPublic || d.publicDomain == "" || objectKey == "" {
		return ""
	}
	return d.publicDomain + "/" + strings.TrimLeft(objectKey, "/")
}

func (d *S3Driver) SignedURL(ctx context.Context, bucket, objectKey string, ttl time.Duration) (string, error) {
	out, err := d.presign.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(firstNonEmpty(bucket, d.bucket)), Key: aws.String(objectKey)}, func(options *s3.PresignOptions) {
		options.Expires = ttl
	})
	if err != nil {
		return "", err
	}
	return out.URL, nil
}
