package service

import (
	"errors"
	"strings"
	"testing"

	"github.com/wannanbigpig/gin-layout/internal/model"
)

func TestStorageSettingsAuditDiffRedactsChangedSecrets(t *testing.T) {
	before := StorageSettings{ActiveDriver: "local"}
	before.Config.AliyunOSS.AccessKeySecret = "old-private-key"
	after := before
	after.ActiveDriver = "aliyun_oss"
	after.Config.AliyunOSS.AccessKeySecret = "new-private-key"
	raw := storageSettingsAuditDiff(before, after)
	if strings.Contains(raw, "old-private-key") || strings.Contains(raw, "new-private-key") {
		t.Fatalf("secret leaked: %s", raw)
	}
	if !strings.Contains(raw, "access_key_secret") || !strings.Contains(raw, "active_driver") {
		t.Fatalf("missing changed fields: %s", raw)
	}
	if raw := storageSettingsAuditDiff(before, before); raw != "[]" {
		t.Fatalf("unchanged settings produce diff: %s", raw)
	}
}

func TestStorageSettingsDoNotFallBackOnDatabaseFailure(t *testing.T) {
	_, err := NewStorageConfigService().Get(false)
	if !errors.Is(err, model.ErrDBUninitialized) {
		t.Fatalf("expected database error, got %v", err)
	}
}
