package filestorage

import (
	"context"
	"net/url"
	"testing"
	"time"
)

func TestAliyunOSSDriverSignedImageThumbnailURLIncludesProcess(t *testing.T) {
	driver := NewAliyunOSSDriver(AliyunOSSConfig{
		Endpoint:        "https://oss-cn-hangzhou.aliyuncs.com",
		Region:          "cn-hangzhou",
		Bucket:          "example-bucket",
		AccessKeyID:     "ak",
		AccessKeySecret: "sk",
	})

	signedURL, err := driver.SignedImageThumbnailURL(context.Background(), "example-bucket", "images/demo.png", time.Minute)
	if err != nil {
		t.Fatalf("signed thumbnail url failed: %v", err)
	}

	parsedURL, err := url.Parse(signedURL)
	if err != nil {
		t.Fatalf("parse signed url failed: %v", err)
	}
	if got := parsedURL.Query().Get("x-oss-process"); got != aliyunOSSThumbnailProcess {
		t.Fatalf("expected x-oss-process %q, got %q", aliyunOSSThumbnailProcess, got)
	}
}
