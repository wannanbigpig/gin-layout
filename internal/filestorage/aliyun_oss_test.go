package filestorage

import (
	"context"
	"net/url"
	"testing"
	"time"
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
