package autoload

// ServerConfig 定义项目配置
type ServerConfig struct {
	Host           string `ini:"host"`
	Port           uint   `ini:"port"`
}

var Server = &ServerConfig{
	Host:           "127.0.0.1",
	Port:           9999,
}