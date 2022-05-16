package data

import (
	"fmt"
	c "github.com/wannanbigpig/gin-layout/config"
	log "github.com/wannanbigpig/gin-layout/pkg/logger"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

var MysqlDB *gorm.DB

type Writer interface {
	Printf(string, ...interface{})
}

type WriterLog struct{}

func (w WriterLog) Printf(format string, args ...interface{}) {
	log.Logger.Sugar().Infof(format, args...)
}

func initMysql() {
	logConfig := logger.New(
		WriterLog{},
		logger.Config{
			SlowThreshold:             0,                                        // 慢 SQL 阈值
			LogLevel:                  logger.LogLevel(c.Config.Mysql.LogLevel), // 日志级别
			IgnoreRecordNotFoundError: false,                                    // 忽略ErrRecordNotFound（记录未找到）错误
			Colorful:                  false,                                    // 是否启用彩色打印
		},
	)

	configs := &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix: c.Config.Mysql.TablePrefix, // 表名前缀
			// SingularTable: true,  // 使用单数表名
		},
		Logger: logConfig,
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		c.Config.Mysql.Username,
		c.Config.Mysql.Password,
		c.Config.Mysql.Host,
		c.Config.Mysql.Port,
		c.Config.Mysql.Database,
		c.Config.Mysql.Charset,
	)
	var err error
	MysqlDB, err = gorm.Open(mysql.Open(dsn), configs)

	if err != nil {
		panic("连接数据库失败：" + err.Error())
	}

	sqlDB, _ := MysqlDB.DB()
	// SetMaxIdleConns 用于设置连接池中空闲连接的最大数量。
	sqlDB.SetMaxIdleConns(c.Config.Mysql.MaxIdleConns)
	// SetMaxOpenConns 设置打开数据库连接的最大数量。
	sqlDB.SetMaxOpenConns(c.Config.Mysql.MaxOpenConns)
	// SetConnMaxLifetime 设置了连接可复用的最大时间。
	sqlDB.SetConnMaxLifetime(c.Config.Mysql.MaxLifetime)
}
