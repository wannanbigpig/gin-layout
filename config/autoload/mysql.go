package autoload

import "time"

// MysqlConfig 定义 MySQL 连接与连接池配置。
type MysqlConfig struct {
	// Enable 是否启用 MySQL 连接
	Enable bool `mapstructure:"enable"`
	// Host 数据库服务器地址
	Host string `mapstructure:"host"`
	// Username 数据库用户名
	Username string `mapstructure:"username"`
	// Password 数据库密码
	Password string `mapstructure:"password"`
	// Port 数据库端口
	Port uint16 `mapstructure:"port"`
	// Database 数据库名称
	Database string `mapstructure:"database"`
	// Charset 字符集，推荐 utf8mb4
	Charset string `mapstructure:"charset"`
	// TablePrefix 表名前缀，用于区分不同应用的表
	TablePrefix string `mapstructure:"table_prefix"`
	// MaxIdleConns 最大空闲连接数
	MaxIdleConns int `mapstructure:"max_idle_conns"`
	// MaxOpenConns 最大打开连接数（并发连接数上限）
	MaxOpenConns int `mapstructure:"max_open_conns"`
	// MaxLifetime 连接最大存活时间，超时会被复用前重新创建
	MaxLifetime time.Duration `mapstructure:"max_lifetime"`
	// LogLevel GORM 日志级别：1=silent, 2=error, 3=warn, 4=info
	LogLevel int `mapstructure:"log_level"`
	// PrintSql 是否打印 SQL 到控制台，调试时使用
	PrintSql bool `mapstructure:"print_sql"`
}

// Mysql 数据库默认配置。
var Mysql = MysqlConfig{
	Enable:       false, // 默认关闭，需要时开启
	Host:         "127.0.0.1",
	Username:     "root",
	Password:     "root1234",
	Port:         3306,
	Database:     "test",
	Charset:      "utf8mb4",
	TablePrefix:  "",
	MaxIdleConns: 10,
	MaxOpenConns: 100,
	MaxLifetime:  time.Hour, // 连接存活 1 小时
	LogLevel:     4,         // info 级别
	PrintSql:     false,     // 默认不打印 SQL
}
