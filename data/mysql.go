package data

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

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
	mysqlValue     atomic.Value
	mysqlMu        sync.Mutex
)

type mysqlSlot struct {
	db *gorm.DB
}

// Writer 定义 GORM 自定义日志写入接口。
type Writer interface {
	Printf(string, ...interface{})
}

// WriterLog 将 GORM SQL 日志转发到项目日志组件。
type WriterLog struct{}

// Printf 实现 GORM logger.Writer 接口。
func (w WriterLog) Printf(format string, args ...interface{}) {
	if c.GetConfig().Mysql.PrintSql {
		log.Logger.Sugar().Infof(format, args...)
	}
}

// GenerateDSN 生成带固定连接参数的 MySQL DSN。
func GenerateDSN(cfg *c.Conf) string {
	// 防御性编码
	if cfg == nil || cfg.Mysql.Host == "" || cfg.Mysql.Database == "" {
		return ""
	}

	// 特殊字符处理
	username := strings.Replace(url.QueryEscape(cfg.Mysql.Username), "%", "%25", -1)
	password := strings.Replace(url.QueryEscape(cfg.Mysql.Password), "%", "%25", -1)

	// IPv6处理
	host := cfg.Mysql.Host
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}

	// 强制关键参数
	charset := "utf8mb4"
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		username,
		password,
		host,
		cfg.Mysql.Port,
		cfg.Mysql.Database,
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

// initMysql 初始化当前配置下的 MySQL 连接。
func initMysql() error {
	return reloadMysql(c.GetConfig())
}

func reloadMysql(cfg *c.Conf) error {
	mysqlMu.Lock()
	defer mysqlMu.Unlock()

	next, err := openMysql(cfg)
	if err != nil {
		return err
	}

	old := currentMysql()
	oldSQLDB := getSQLDB(old)
	mysqlDB = next
	mysqlValue.Store(mysqlSlot{db: next})
	mysqlInitError = nil
	if oldSQLDB != nil {
		_ = oldSQLDB.Close()
	}
	return nil
}

func openMysql(cfg *c.Conf) (*gorm.DB, error) {
	if cfg == nil || !cfg.Mysql.Enable {
		return nil, nil
	}
	// Validate configuration parameters
	if cfg.Mysql.MaxIdleConns < 0 || cfg.Mysql.MaxOpenConns < 0 || cfg.Mysql.MaxLifetime < 0 {
		return nil, fmt.Errorf("invalid MySQL configuration: MaxIdleConns, MaxOpenConns, and MaxLifetime must be non-negative")
	}

	// Initialize logger
	logConfig := logger.New(
		WriterLog{},
		logger.Config{
			SlowThreshold:             0,
			LogLevel:                  logger.LogLevel(cfg.Mysql.LogLevel),
			IgnoreRecordNotFoundError: false,
			Colorful:                  false,
		},
	)

	// Configure GORM settings
	configs := &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix: cfg.Mysql.TablePrefix,
		},
		Logger:                 logConfig,
		SkipDefaultTransaction: true,
	}

	// Open database connection
	dsn := GenerateDSN(cfg)
	db, err := gorm.Open(mysql.Open(dsn), configs)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MySQL: %s", err.Error())
	}

	// Get underlying sql.DB and configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %s", err.Error())
	}

	sqlDB.SetMaxIdleConns(cfg.Mysql.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.Mysql.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(cfg.Mysql.MaxLifetime)
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("failed to ping MySQL: %s", err.Error())
	}

	// Register callbacks if needed
	registerCallbacks(db)
	return db, nil
}

// registerCallbacks 预留 GORM 回调注册入口。
func registerCallbacks(db *gorm.DB) {
	// Uncomment and implement these functions if needed
	// db.Callback().Create().After("gorm:create").Register("log_create_operation", logCreateOperation)
	// db.Callback().Update().After("gorm:update").Register("log_update_operation", logUpdateOperation)
	// db.Callback().Delete().After("gorm:delete").Register("log_delete_operation", logDeleteOperation)
}

// MysqlDB 返回当前生效的 MySQL 连接实例。
func MysqlDB() *gorm.DB {
	if db := currentMysql(); db != nil {
		return db
	}
	if mysqlDB == nil {
		mysqlOnce.Do(func() {
			mysqlInitError = initMysql()
		})
	}
	return currentMysql()
}

// MysqlInitError 返回 MySQL 初始化阶段记录的错误。
func MysqlInitError() error {
	return mysqlInitError
}

// ReloadMysql 重新加载 MySQL 连接。
func ReloadMysql(cfg *c.Conf) error {
	return reloadMysql(cfg)
}

// CloseMysql 关闭当前 MySQL 连接池。
func CloseMysql() error {
	mysqlMu.Lock()
	defer mysqlMu.Unlock()

	current := currentMysql()
	mysqlDB = nil
	mysqlValue.Store(mysqlSlot{})
	mysqlInitError = nil
	if current == nil {
		return nil
	}

	sqlDB := getSQLDB(current)
	if sqlDB == nil {
		return nil
	}
	return sqlDB.Close()
}

func currentMysql() *gorm.DB {
	if slot, ok := mysqlValue.Load().(mysqlSlot); ok {
		return slot.db
	}
	return mysqlDB
}

func getSQLDB(db *gorm.DB) *sql.DB {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil
	}
	return sqlDB
}
