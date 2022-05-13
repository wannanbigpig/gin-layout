package autoload

type LoggerConfig struct {
	Filename   string `ini:"file_name"`
	MaxSize    int    `ini:"max_size"`
	MaxBackups int    `ini:"max_backups"`
	MaxAge     int    `ini:"max_age"`
	Compress   bool   `ini:"compress"`
}

var Logger = &LoggerConfig{
	Filename:   "sys.log",
	MaxSize:    2,
	MaxBackups: 2,
	MaxAge:     14,
	Compress:   false,
}
