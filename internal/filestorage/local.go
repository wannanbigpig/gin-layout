package filestorage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type LocalDriver struct {
	publicBasePath  string
	privateBasePath string
}

func NewLocalDriver(config LocalConfig, fallbackPublicPath, fallbackPrivatePath string) *LocalDriver {
	publicPath := firstNonEmpty(config.PublicBasePath, config.BasePath, fallbackPublicPath)
	privatePath := firstNonEmpty(config.PrivateBasePath, config.BasePath, fallbackPrivatePath)
	return &LocalDriver{publicBasePath: publicPath, privateBasePath: privatePath}
}

func (d *LocalDriver) Name() string { return "local" }

func (d *LocalDriver) Put(_ context.Context, input PutInput) (PutResult, error) {
	target, err := d.resolve(input.Bucket, input.ObjectKey)
	if err != nil {
		return PutResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return PutResult{}, err
	}
	file, err := os.Create(target)
	if err != nil {
		return PutResult{}, err
	}
	defer file.Close()
	if _, err := io.Copy(file, input.Reader); err != nil {
		return PutResult{}, err
	}
	return PutResult{Bucket: input.Bucket, ObjectKey: input.ObjectKey}, nil
}

func (d *LocalDriver) Open(_ context.Context, bucket, objectKey string) (io.ReadCloser, error) {
	path, err := d.resolve(bucket, objectKey)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

func (d *LocalDriver) Exists(_ context.Context, bucket, objectKey string) (bool, error) {
	path, err := d.resolve(bucket, objectKey)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (d *LocalDriver) Delete(_ context.Context, bucket, objectKey string) error {
	path, err := d.resolve(bucket, objectKey)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (d *LocalDriver) URL(bucket, objectKey string, _ bool) string {
	if objectKey == "" {
		return ""
	}
	return "/" + strings.TrimLeft(url.PathEscape(objectKey), "/")
}

func (d *LocalDriver) SignedURL(_ context.Context, bucket, objectKey string, _ time.Duration) (string, error) {
	return d.URL(bucket, objectKey, false), nil
}

func (d *LocalDriver) resolve(bucket, objectKey string) (string, error) {
	if objectKey == "" {
		return "", fmt.Errorf("object_key is required")
	}
	base := d.privateBasePath
	if bucket == "public" {
		base = d.publicBasePath
	}
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	target := filepath.Join(absBase, filepath.Clean(strings.ReplaceAll(objectKey, "\\", "/")))
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	if absTarget != absBase && !strings.HasPrefix(absTarget, absBase+string(filepath.Separator)) {
		return "", fmt.Errorf("object_key escapes storage root")
	}
	return absTarget, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
