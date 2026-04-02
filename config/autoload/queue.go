package autoload

// QueueConfig 异步任务队列配置。
type QueueConfig struct {
	Enable              bool           `mapstructure:"enable" yaml:"enable"`
	Namespace           string         `mapstructure:"namespace" yaml:"namespace"`
	Concurrency         int            `mapstructure:"concurrency" yaml:"concurrency"`
	StrictPriority      bool           `mapstructure:"strict_priority" yaml:"strict_priority"`
	Queues              map[string]int `mapstructure:"queues" yaml:"queues"`
	AuditMaxRetry       int            `mapstructure:"audit_max_retry" yaml:"audit_max_retry"`
	AuditTimeoutSeconds int            `mapstructure:"audit_timeout_seconds" yaml:"audit_timeout_seconds"`
}

var Queue = QueueConfig{
	Enable:         false,
	Namespace:      "go_layout",
	Concurrency:    8,
	StrictPriority: false,
	Queues: map[string]int{
		"critical": 4,
		"default":  2,
		"audit":    2,
		"low":      1,
	},
	AuditMaxRetry:       3,
	AuditTimeoutSeconds: 10,
}
