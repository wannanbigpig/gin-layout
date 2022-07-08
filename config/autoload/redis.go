package autoload

type RedisConfig struct {
	Enable   bool   `ini:"enable" yaml:"enable"`
	Host     string `ini:"host" yaml:"host"`
	Port     string `ini:"port" yaml:"port"`
	Password string `ini:"password" yaml:"password"`
	Database int    `ini:"database" yaml:"database"`
}

var Redis = RedisConfig{
	Enable:   false,
	Host:     "127.0.0.1",
	Password: "root1234",
	Port:     "6379",
	Database: 0,
}
