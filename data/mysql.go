package data

import (
	"fmt"
	"net/url"
	"strings"
	"sync"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	c "github.com/wannanbigpig/gin-layout/config"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
)

var (
	mysqlDB        *gorm.DB
	mysqlOnce      sync.Once
	mysqlInitError error
)

// Writer interface for custom logger
type Writer interface {
	Printf(string, ...interface{})
}

// WriterLog Custom logger implementation
type WriterLog struct{}

func (w WriterLog) Printf(format string, args ...interface{}) {
	if c.Config.Mysql.PrintSql {
		log.Logger.Sugar().Infof(format, args...)
	}
}

// GenerateDSN generates the MySQL DSN string with proper encoding
func GenerateDSN() string {
	// 防御性编码
	if c.Config.Mysql.Host == "" || c.Config.Mysql.Database == "" {
		return ""
	}

	// 特殊字符处理
	username := strings.Replace(url.QueryEscape(c.Config.Mysql.Username), "%", "%25", -1)
	password := strings.Replace(url.QueryEscape(c.Config.Mysql.Password), "%", "%25", -1)

	// IPv6处理
	host := c.Config.Mysql.Host
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}

	// 强制关键参数
	charset := "utf8mb4"
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		username,
		password,
		host,
		c.Config.Mysql.Port,
		c.Config.Mysql.Database,
	)

	// 参数显式排序
	params := url.Values{
		"charset":      []string{charset},
		"parseTime":    []string{"true"},
		"loc":          []string{"Local"},
		"timeout":      []string{"5s"},
		"readTimeout":  []string{"30s"},
		"writeTimeout": []string{"60s"},
	}

	return dsn + "?" + params.Encode()
}

// initMysql initializes the MySQL database connection
func initMysql() error {
	// Validate configuration parameters
	if c.Config.Mysql.MaxIdleConns < 0 || c.Config.Mysql.MaxOpenConns < 0 || c.Config.Mysql.MaxLifetime < 0 {
		return fmt.Errorf("invalid MySQL configuration: MaxIdleConns, MaxOpenConns, and MaxLifetime must be non-negative")
	}

	// Initialize logger
	logConfig := logger.New(
		WriterLog{},
		logger.Config{
			SlowThreshold:             0,                                        // Slow SQL threshold
			LogLevel:                  logger.LogLevel(c.Config.Mysql.LogLevel), // Log level
			IgnoreRecordNotFoundError: false,                                    // Ignore ErrRecordNotFound
			Colorful:                  false,                                    // Disable colorful logs
		},
	)

	// Configure GORM settings
	configs := &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix: c.Config.Mysql.TablePrefix, // Table prefix
		},
		Logger:                 logConfig,
		SkipDefaultTransaction: true,
	}

	// Open database connection
	dsn := GenerateDSN()
	var err error
	mysqlDB, err = gorm.Open(mysql.Open(dsn), configs)
	if err != nil {
		return fmt.Errorf("failed to connect to MySQL: %s", err.Error())
	}

	// Get underlying sql.DB and configure connection pool
	sqlDB, err := mysqlDB.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %s", err.Error())
	}

	sqlDB.SetMaxIdleConns(c.Config.Mysql.MaxIdleConns)
	sqlDB.SetMaxOpenConns(c.Config.Mysql.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(c.Config.Mysql.MaxLifetime)

	// Register callbacks if needed
	registerCallbacks(mysqlDB)
	return nil
}

// registerCallbacks registers GORM callbacks for logging operations
func registerCallbacks(db *gorm.DB) {
	// Uncomment and implement these functions if needed
	// db.Callback().Create().After("gorm:create").Register("log_create_operation", logCreateOperation)
	// db.Callback().Update().After("gorm:update").Register("log_update_operation", logUpdateOperation)
	// db.Callback().Delete().After("gorm:delete").Register("log_delete_operation", logDeleteOperation)
}

// MysqlDB returns the singleton instance of gorm.DB
func MysqlDB() *gorm.DB {
	if mysqlDB == nil {
		mysqlOnce.Do(func() {
			mysqlInitError = initMysql()
		})
	}
	return mysqlDB
}

func MysqlInitError() error {
	return mysqlInitError
}
