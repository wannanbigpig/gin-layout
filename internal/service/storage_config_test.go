package service

import (
	"errors"
	"strings"
	"testing"

	pkgerr "github.com/wannanbigpig/gin-layout/internal/pkg/errors"
)

func TestNewStorageTestBusinessErrorForPermissionDenied(t *testing.T) {
	err := newStorageTestBusinessError("delete", errors.New("operation error DeleteObject: AccessDenied: no right to access this object"))
	if err == nil {
		t.Fatal("expected business error")
	}

	var businessErr *pkgerr.BusinessError
	if !errors.As(err, &businessErr) {
		t.Fatalf("expected BusinessError, got %T", err)
	}
	if businessErr.GetCode() != pkgerr.StorageTestFailed {
		t.Fatalf("expected code %d, got %d", pkgerr.StorageTestFailed, businessErr.GetCode())
	}
	if !strings.Contains(businessErr.GetMessage(), "没有删除对象权限") {
		t.Fatalf("expected friendly permission message, got %q", businessErr.GetMessage())
	}
	if len(businessErr.GetContextErr()) != 1 {
		t.Fatalf("expected one context error, got %d", len(businessErr.GetContextErr()))
	}
}

func TestNewStorageTestBusinessErrorForGeneralFailure(t *testing.T) {
	err := newStorageTestBusinessError("signed_url", errors.New("dial tcp timeout"))
	if err == nil {
		t.Fatal("expected business error")
	}

	var businessErr *pkgerr.BusinessError
	if !errors.As(err, &businessErr) {
		t.Fatalf("expected BusinessError, got %T", err)
	}
	if businessErr.GetCode() != pkgerr.StorageTestFailed {
		t.Fatalf("expected code %d, got %d", pkgerr.StorageTestFailed, businessErr.GetCode())
	}
	if !strings.Contains(businessErr.GetMessage(), "请检查签名配置、Bucket 读取权限和 Endpoint") {
		t.Fatalf("expected general guidance message, got %q", businessErr.GetMessage())
	}
}
