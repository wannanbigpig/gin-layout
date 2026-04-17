package autoload

import "time"

// RedisConfig 定义 Redis 连接配置。
type RedisConfig struct {
	// Enable 是否启用 Redis 连接
	Enable bool `mapstructure:"enable"`
	// Host Redis 服务器地址
	Host string `mapstructure:"host"`
	// Port Redis 服务器端口
	Port string `mapstructure:"port"`
	// Password Redis 密码，空字符串表示无密码
	Password string `mapstructure:"password"`
	// Database 数据库编号，默认 0
	Database int `mapstructure:"database"`
	// PoolSize 连接池大小（最大连接数）
	PoolSize int `mapstructure:"pool_size"`
	// MinIdleConns 最小空闲连接数
	MinIdleConns int `mapstructure:"min_idle_conns"`
	// ConnMaxIdleTime 连接最大空闲时间，超时会被回收
	ConnMaxIdle time.Duration `mapstructure:"conn_max_idle_time"`
	// ConnMaxLifetime 连接最大存活时间，超时会被重新创建
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	// ReadTimeout 读取超时时间
	ReadTimeout time.Duration `mapstructure:"read_timeout"`
	// WriteTimeout 写入超时时间
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// Redis 默认配置。
var Redis = RedisConfig{
	Enable:          false, // 默认关闭，需要时开启
	Host:            "127.0.0.1",
	Password:        "",
	Port:            "6379",
	Database:        0,
	PoolSize:        10,
	MinIdleConns:    5,
	ConnMaxIdle:     5 * time.Minute,  // 空闲 5 分钟回收
	ConnMaxLifetime: 30 * time.Minute, // 连接存活 30 分钟
	ReadTimeout:     3 * time.Second,  // 读取超时 3 秒
	WriteTimeout:    3 * time.Second,  // 写入超时 3 秒
}
