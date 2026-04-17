package autoload

// DivisionTime 定义按时间切割日志时的参数。
type DivisionTime struct {
	// MaxAge 日志文件保留的最大天数，过期会被删除
	MaxAge int `mapstructure:"max_age"`
	// RotationTime 多长时间切割一次日志文件，单位小时（24 表示每天切割）
	RotationTime int `mapstructure:"rotation_time"`
}

// DivisionSize 定义按大小切割日志时的参数。
type DivisionSize struct {
	// MaxSize 日志文件的最大大小（以 MB 为单位），超过该值会触发切割
	MaxSize int `mapstructure:"max_size"`
	// MaxBackups 保留的旧日志文件最大个数，超过会被删除
	MaxBackups int `mapstructure:"max_backups"`
	// MaxAge 旧日志文件保留的最大天数，过期会被删除
	MaxAge int `mapstructure:"max_age"`
	// Compress 是否压缩/归档旧日志文件（gzip 格式）
	Compress bool `mapstructure:"compress"`
}

// LoggerConfig 定义日志输出与切割策略。
type LoggerConfig struct {
	// Output 日志输出方式：file（输出到文件）、stderr（输出到标准错误）
	Output string `mapstructure:"output"`
	// DefaultDivision 默认切割方式：time（按时间）、size（按大小）
	DefaultDivision string `mapstructure:"default_division"`
	// Filename 日志文件名
	Filename string `mapstructure:"file_name"`
	// DivisionTime 按时间切割的参数配置
	DivisionTime DivisionTime `mapstructure:"division_time"`
	// DivisionSize 按大小切割的参数配置
	DivisionSize DivisionSize `mapstructure:"division_size"`
}

var Logger = LoggerConfig{
	Output:          "file", // 默认输出到文件
	DefaultDivision: "time", // 默认按时间切割
	Filename:        "gin-layout.sys.log",
	DivisionTime: DivisionTime{
		MaxAge:       15,  // 日志保留 15 天
		RotationTime: 24,  // 每 24 小时切割一次
	},
	DivisionSize: DivisionSize{
		MaxSize:    20,    // 日志文件最大 20MB
		MaxBackups: 15,    // 最多保留 15 个备份
		MaxAge:     15,    // 日志保留 15 天
		Compress:   false, // 默认不压缩
	},
}
