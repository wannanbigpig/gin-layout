package data

import (
	"testing"
	"time"

	driver "github.com/go-sql-driver/mysql"
	"github.com/wannanbigpig/gin-layout/config"
)

func TestGenerateDSNPreservesCredentials(t *testing.T) {
	for _, host := range []string{"localhost", "::1", "[::1]"} {
		t.Run(host, func(t *testing.T) {
			cfg := &config.Conf{}
			cfg.Mysql.Host = host
			cfg.Mysql.Port = 3306
			cfg.Mysql.Database = "test-db"
			cfg.Mysql.Username = "test+user"
			cfg.Mysql.Password = "test-only@pass+% /:?"
			parsed, err := driver.ParseDSN(GenerateDSN(cfg))
			if err != nil {
				t.Fatal(err)
			}
			if parsed.User != cfg.Mysql.Username || parsed.Passwd != cfg.Mysql.Password {
				t.Fatal("DSN changed the configured credentials")
			}
			if parsed.DBName != cfg.Mysql.Database || !parsed.ParseTime || parsed.Loc != time.Local || parsed.Timeout != 5*time.Second {
				t.Fatal("DSN lost database or connection options")
			}
		})
	}
}
