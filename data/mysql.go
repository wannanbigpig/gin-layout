package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	c "github.com/wannanbigpig/gin-layout/config"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
	"go.uber.org/zap"
)

var (
	mysqlDB        *gorm.DB
	mysqlOnce      sync.Once
	mysqlInitError error
	mysqlValue     atomic.Value
	mysqlMu        sync.Mutex
	mysqlHealth    = newRuntimeHealthCache(defaultRuntimeHealthTTL)
)

type mysqlSlot struct {
	db *gorm.DB
}

const mysqlProbeTimeout = 2 * time.Second

var mysqlRuntimeProbe = func(db *gorm.DB) error {
	sqlDB := getSQLDB(db)
	if sqlDB == nil {
		return errors.New("mysql sql.DB is unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), mysqlProbeTimeout)
	defer cancel()
	return sqlDB.PingContext(ctx)
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
	if next != nil {
		mysqlHealth.SeedReady()
	} else {
		mysqlHealth.Reset()
	}
	if oldSQLDB != nil {
		if err := oldSQLDB.Close(); err != nil {
			log.Logger.Warn("关闭旧 MySQL 连接池失败", zap.Error(err))
		}
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

	return db, nil
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

// MysqlRuntimeStatus 返回带缓存的 MySQL 运行时健康探测结果。
func MysqlRuntimeStatus() RuntimeHealthStatus {
	db := MysqlDB()
	if db == nil {
		mysqlHealth.Reset()
		return RuntimeHealthStatus{
			Ready:     false,
			Error:     mysqlUnavailableError(),
			CheckedAt: time.Now(),
		}
	}
	status := mysqlHealth.Check(func() error {
		return mysqlRuntimeProbe(db)
	})
	if !status.Ready && status.Error == nil {
		status.Error = mysqlUnavailableError()
	}
	return status
}

// MysqlReady 判断 MySQL 当前是否可用。
func MysqlReady() bool {
	return MysqlRuntimeStatus().Ready
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
	mysqlHealth.Reset()
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

func mysqlUnavailableError() error {
	if mysqlInitError != nil {
		return mysqlInitError
	}
	return ErrDBUnavailable
}

// ErrDBUnavailable 表示 MySQL 连接当前不可用。
var ErrDBUnavailable = errors.New("mysql connection is unavailable")
