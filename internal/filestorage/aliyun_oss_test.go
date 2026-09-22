package filestorage

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
)

func TestBrowserSignaturesUsePublicEndpoint(t *testing.T) {
	d := NewAliyunOSSDriver(AliyunOSSConfig{Region: "cn-hangzhou", Endpoint: "https://oss-cn-hangzhou.aliyuncs.com", InternalEndpoint: "https://oss-cn-hangzhou-internal.aliyuncs.com", Bucket: "test-bucket", AccessKeyID: "test-key", AccessKeySecret: "test-secret"})
	ctx := context.Background()
	calls := map[string]func() (string, error){
		"download":  func() (string, error) { return d.SignedURL(ctx, "", "test.txt", time.Minute) },
		"thumbnail": func() (string, error) { return d.SignedImageThumbnailURL(ctx, "", "test.png", time.Minute) },
		"upload":    func() (string, error) { return d.PresignPutObject(ctx, "", "test.txt", "text/plain", time.Minute) },
		"part": func() (string, error) {
			v, e := d.PresignMultipartPart(ctx, MultipartPartInput{ObjectKey: "test.txt", UploadID: "upload-id", PartNumber: 1})
			return v.UploadURL, e
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			signed, err := call()
			if err != nil {
				t.Fatal(err)
			}
			u, err := url.Parse(signed)
			if err != nil {
				t.Fatal(err)
			}
			if u.Host != "test-bucket.oss-cn-hangzhou.aliyuncs.com" {
				t.Fatalf("wrong browser endpoint: %s", u.Host)
			}
		})
	}
}

func TestPresignPutObjectRequiresForbidOverwriteHeader(t *testing.T) {
	d := NewAliyunOSSDriver(AliyunOSSConfig{Region: "cn-hangzhou", Endpoint: "https://oss-cn-hangzhou.aliyuncs.com", Bucket: "test-bucket", AccessKeyID: "test-key", AccessKeySecret: "test-secret"})
	result, err := d.signingClient.Presign(context.Background(), &oss.PutObjectRequest{
		Bucket: oss.Ptr("test-bucket"), Key: oss.Ptr("test.txt"), ContentType: oss.Ptr("text/plain"), ForbidOverwrite: oss.Ptr("true"),
	}, func(options *oss.PresignOptions) {
		options.Expires = time.Minute
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.SignedHeaders["X-Oss-Forbid-Overwrite"] != "true" {
		t.Fatalf("expected signed PUT to require x-oss-forbid-overwrite header: %#v", result.SignedHeaders)
	}
}

func TestPresignUploadReturnsCompleteBrowserRequest(t *testing.T) {
	d := NewAliyunOSSDriver(AliyunOSSConfig{Region: "cn-hangzhou", Endpoint: "https://oss-cn-hangzhou.aliyuncs.com", Bucket: "test-bucket", AccessKeyID: "test-key", AccessKeySecret: "test-secret"})
	result, err := d.PresignUpload(context.Background(), "", "test.txt", "text/plain", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if result.Method != "PUT" || result.URL == "" {
		t.Fatalf("unexpected direct upload request: %#v", result)
	}
	if result.Headers["X-Oss-Forbid-Overwrite"] != "true" || result.Headers["Content-Type"] != "text/plain" {
		t.Fatalf("missing signed upload headers: %#v", result.Headers)
	}
}

func TestForcePathStyleChangesSignedURLAddressing(t *testing.T) {
	d := NewAliyunOSSDriver(AliyunOSSConfig{Region: "cn-hangzhou", Endpoint: "https://oss-cn-hangzhou.aliyuncs.com", Bucket: "test-bucket", AccessKeyID: "test-key", AccessKeySecret: "test-secret", ForcePathStyle: true})
	signed, err := d.SignedURL(context.Background(), "", "test.txt", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(signed)
	if err != nil {
		t.Fatal(err)
	}
	if u.Host != "oss-cn-hangzhou.aliyuncs.com" || u.Path != "/test-bucket/test.txt" {
		t.Fatalf("path-style configuration was not applied: %s", signed)
	}
}
