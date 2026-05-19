package filestorage

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
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

func (d *S3Driver) PresignPutObject(ctx context.Context, bucket, objectKey string, contentType string, ttl time.Duration) (string, error) {
	input := &s3.PutObjectInput{
		Bucket: aws.String(firstNonEmpty(bucket, d.bucket)),
		Key:    aws.String(objectKey),
	}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}
	req, err := d.presign.PresignPutObject(ctx, input, func(opts *s3.PresignOptions) {
		opts.Expires = ttl
	})
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

func (d *S3Driver) InitMultipartUpload(ctx context.Context, input MultipartUploadInput) (MultipartUploadResult, error) {
	bucket := firstNonEmpty(input.Bucket, d.bucket)
	out, err := d.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(input.ObjectKey),
		ContentType: aws.String(input.ContentType),
	})
	if err != nil {
		return MultipartUploadResult{}, err
	}
	return MultipartUploadResult{
		UploadID:  aws.ToString(out.UploadId),
		Bucket:    bucket,
		ObjectKey: input.ObjectKey,
	}, nil
}

func (d *S3Driver) PresignMultipartPart(ctx context.Context, input MultipartPartInput) (MultipartPartResult, error) {
	bucket := firstNonEmpty(input.Bucket, d.bucket)
	req, err := d.presign.PresignUploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(input.ObjectKey),
		UploadId:   aws.String(input.UploadID),
		PartNumber: aws.Int32(input.PartNumber),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = 15 * time.Minute
	})
	if err != nil {
		return MultipartPartResult{}, err
	}
	return MultipartPartResult{
		PartNumber: input.PartNumber,
		UploadURL:  req.URL,
		Headers:    nil,
	}, nil
}

func (d *S3Driver) CompleteMultipartUpload(ctx context.Context, bucket, objectKey, uploadID string, parts []CompletePartInfo) error {
	if len(parts) == 0 {
		return fmt.Errorf("parts is required")
	}
	bucket = firstNonEmpty(bucket, d.bucket)
	sortedParts := make([]types.CompletedPart, len(parts))
	for i, p := range parts {
		sortedParts[i] = types.CompletedPart{
			ETag:       aws.String(p.ETag),
			PartNumber: aws.Int32(p.PartNumber),
		}
	}
	sort.Slice(sortedParts, func(i, j int) bool {
		return aws.ToInt32(sortedParts[i].PartNumber) < aws.ToInt32(sortedParts[j].PartNumber)
	})
	_, err := d.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(objectKey),
		UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: sortedParts,
		},
	})
	return err
}

func (d *S3Driver) AbortMultipartUpload(ctx context.Context, bucket, objectKey, uploadID string) error {
	bucket = firstNonEmpty(bucket, d.bucket)
	_, err := d.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(objectKey),
		UploadId: aws.String(uploadID),
	})
	return err
}
