package data

import (
	"os"
	"strings"
	"testing"
)

func TestAdminLoginLogCleanupIndexesMatchDeletionConditions(t *testing.T) {
	content, err := os.ReadFile("migrations/20260425000001_init_table.up.sql")
	if err != nil {
		t.Fatalf("read init migration failed: %v", err)
	}
	sql := string(content)

	for _, indexDDL := range []string{
		"KEY `aall_created_at` (`created_at`)",
		"KEY `aall_deleted_at_is_revoked_revoked_at` (`deleted_at`, `is_revoked`, `revoked_at`)",
	} {
		if !strings.Contains(sql, indexDDL) {
			t.Fatalf("expected admin_login_logs cleanup index %q", indexDDL)
		}
	}
}
