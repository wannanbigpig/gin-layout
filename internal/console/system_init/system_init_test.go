package system_init

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/wannanbigpig/gin-layout/config"
	"github.com/wannanbigpig/gin-layout/config/autoload"
)

func TestPrintDangerousResetWarningShowsTargetWithoutPassword(t *testing.T) {
	cfg := &config.Conf{
		AppConfig: autoload.AppConfig{
			AppEnv:  "prod",
			Debug:   false,
			BaseURL: "https://admin.example.com",
		},
		Mysql: autoload.MysqlConfig{
			Enable:      true,
			Host:        "db.example.com",
			Port:        3306,
			Database:    "prod_admin",
			Username:    "admin_user",
			Password:    "super-secret",
			TablePrefix: "x_",
		},
	}

	var out bytes.Buffer
	printDangerousResetWarning(&out, cfg)
	text := out.String()

	for _, want := range []string{
		ansiBoldRed,
		"危险操作",
		"app_env: prod",
		"base_url: https://admin.example.com",
		"host: db.example.com",
		"port: 3306",
		"database: prod_admin",
		"username: admin_user",
		"password: (hidden)",
		"table_prefix: x_",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected warning to contain %q, got:\n%s", want, text)
		}
	}
	if strings.Contains(text, "super-secret") {
		t.Fatalf("warning must not print mysql password, got:\n%s", text)
	}
}

func TestConfirmDangerousResetRequiresBackupPhrase(t *testing.T) {
	restore := setInitSystemConfirmationFlags(t, "", false)
	defer restore()

	cfg := initSystemTestConfig("demo_db")
	var out bytes.Buffer
	err := confirmDangerousReset(bufio.NewReader(strings.NewReader("no\n")), &out, cfg)
	if err == nil || !strings.Contains(err.Error(), errInitSystemCancelled.Error()) {
		t.Fatalf("expected cancellation when backup phrase is wrong, got %v", err)
	}
}

func TestConfirmDangerousResetRequiresMatchingDatabase(t *testing.T) {
	restore := setInitSystemConfirmationFlags(t, "other_db", true)
	defer restore()

	cfg := initSystemTestConfig("demo_db")
	var out bytes.Buffer
	err := confirmDangerousReset(bufio.NewReader(strings.NewReader("")), &out, cfg)
	if err == nil || !strings.Contains(err.Error(), "mysql.database=\"demo_db\"") {
		t.Fatalf("expected database mismatch error, got %v", err)
	}
}

func TestConfirmDangerousResetAcceptsBackupAndDatabaseName(t *testing.T) {
	restore := setInitSystemConfirmationFlags(t, "", false)
	defer restore()

	cfg := initSystemTestConfig("demo_db")
	var out bytes.Buffer
	err := confirmDangerousReset(bufio.NewReader(strings.NewReader(backupConfirmationText+"\ndemo_db\n")), &out, cfg)
	if err != nil {
		t.Fatalf("expected confirmation to pass, got %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "确认已经备份目标库") || !strings.Contains(text, "demo_db") {
		t.Fatalf("expected prompts to mention backup and database, got:\n%s", text)
	}
}

func initSystemTestConfig(database string) *config.Conf {
	return &config.Conf{
		AppConfig: autoload.AppConfig{AppEnv: "local"},
		Mysql: autoload.MysqlConfig{
			Enable:   true,
			Host:     "127.0.0.1",
			Port:     3306,
			Database: database,
			Username: "root",
		},
	}
}

func setInitSystemConfirmationFlags(t *testing.T, database string, backupOK bool) func() {
	t.Helper()
	oldDatabase := initSystemDatabase
	oldBackupOK := initSystemBackupOK
	initSystemDatabase = database
	initSystemBackupOK = backupOK
	return func() {
		initSystemDatabase = oldDatabase
		initSystemBackupOK = oldBackupOK
	}
}
