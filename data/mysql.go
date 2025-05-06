package data

import (
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	c "github.com/wannanbigpig/gin-layout/config"
	log "github.com/wannanbigpig/gin-layout/internal/pkg/logger"
)

var MysqlDB *gorm.DB

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
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		c.Config.Mysql.Username,
		c.Config.Mysql.Password,
		c.Config.Mysql.Host,
		c.Config.Mysql.Port,
		c.Config.Mysql.Database,
		c.Config.Mysql.Charset,
	)
}

// initMysql initializes the MySQL database connection
func initMysql() {
	// Validate configuration parameters
	if c.Config.Mysql.MaxIdleConns < 0 || c.Config.Mysql.MaxOpenConns < 0 || c.Config.Mysql.MaxLifetime < 0 {
		panic("Invalid MySQL configuration: MaxIdleConns, MaxOpenConns, and MaxLifetime must be non-negative")
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
	MysqlDB, err = gorm.Open(mysql.Open(dsn), configs)
	if err != nil {
		panic(fmt.Sprintf("Mysql connection failed: %v", err))
	}

	// Get underlying sql.DB and configure connection pool
	sqlDB, err := MysqlDB.DB()
	if err != nil {
		panic(fmt.Sprintf("Failed to get sql.DB: %v", err))
	}

	sqlDB.SetMaxIdleConns(c.Config.Mysql.MaxIdleConns)
	sqlDB.SetMaxOpenConns(c.Config.Mysql.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(c.Config.Mysql.MaxLifetime)

	// Register callbacks if needed
	registerCallbacks(MysqlDB)
}

// registerCallbacks registers GORM callbacks for logging operations
func registerCallbacks(db *gorm.DB) {
	// Uncomment and implement these functions if needed
	// db.Callback().Create().After("gorm:create").Register("log_create_operation", logCreateOperation)
	// db.Callback().Update().After("gorm:update").Register("log_update_operation", logUpdateOperation)
	// db.Callback().Delete().After("gorm:delete").Register("log_delete_operation", logDeleteOperation)
}
