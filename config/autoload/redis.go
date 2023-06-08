package autoload

type RedisConfig struct {
	Enable   bool   `mapstructure:"enable"`
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Password string `mapstructure:"password"`
	Database int    `mapstructure:"database"`
}

var Redis = RedisConfig{
	Enable:   false,
	Host:     "127.0.0.1",
	Password: "root1234",
	Port:     "6379",
	Database: 0,
}
