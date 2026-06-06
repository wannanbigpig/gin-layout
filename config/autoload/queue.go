package autoload

// QueueRedisConfig 队列使用的 Redis 连接配置。
type QueueRedisConfig struct {
	// Host Redis 服务器地址
	Host string `mapstructure:"host" yaml:"host"`
	// Port Redis 服务器端口
	Port string `mapstructure:"port" yaml:"port"`
	// Password Redis 密码，空字符串表示无密码
	Password string `mapstructure:"password" yaml:"password"`
	// Database 数据库编号
	Database int `mapstructure:"database" yaml:"database"`
}

// QueueConfig 异步任务队列配置。
type QueueConfig struct {
	// Enable 是否启用异步队列。false 时同步执行任务（如审计日志直接写库）
	Enable bool `mapstructure:"enable" yaml:"enable"`
	// UseDefaultRedis 是否复用全局 redis 配置。
	// true: 使用 redis.* 作为队列连接（默认）
	// false: 使用 queue.redis.* 作为队列独立连接
	UseDefaultRedis bool `mapstructure:"use_default_redis" yaml:"use_default_redis"`
	// Redis 队列独立 Redis 配置，仅当 UseDefaultRedis=false 时生效
	Redis QueueRedisConfig `mapstructure:"redis" yaml:"redis"`
	// Namespace 队列命名空间前缀，用于隔离不同应用的队列
	Namespace string `mapstructure:"namespace" yaml:"namespace"`
	// Concurrency Worker Server 的最大并发协程数（全局上限）
	// 建议值：开发环境 2-4，小流量生产 8-16，中等流量 16-32
	// 注意：并发过高会增加数据库压力，审计日志类任务建议 8-16
	Concurrency int `mapstructure:"concurrency" yaml:"concurrency"`
	// StrictPriority 是否严格优先级模式。
	// true: 必须处理完高优先级队列的所有任务后，才处理低优先级队列
	// false: 按权重比例调度，高优先级队列的任务被调度的概率更大（推荐）
	StrictPriority bool `mapstructure:"strict_priority" yaml:"strict_priority"`
	// ConsumeCronFallback 是否允许 worker 消费历史误入 Asynq 的非高风险 cron 任务。
	// 仅用于清理旧队列残留，不影响 cron 调度开关。
	ConsumeCronFallback bool `mapstructure:"consume_cron_fallback" yaml:"consume_cron_fallback"`
	// Queues 各队列的权重配置，key 为队列名，value 为权重值
	// 新增队列必须在此配置，否则 Worker 不会消费该队列的任务！
	// 权重决定任务被调度的概率，不是分配的协程数量！
	// 所有队列共享 Concurrency 个协程，权重越高越容易被优先调度
	// 调度概率 = 该队列权重 / 所有队列权重之和
	// 示例（总权重=4+2+2+1=9）：
	//   critical: 权重 4 → 4/9≈44% 概率被调度（支付回调、短信发送）
	//   default:  权重 2 → 2/9≈22% 概率被调度（普通异步任务）
	//   audit:    权重 2 → 2/11≈18% 概率被调度（请求日志落库）
	//   export:   权重 2 → 2/11≈18% 概率被调度（异步导出）
	//   low:      权重 1 → 1/11≈9% 概率被调度（批量通知）
	Queues map[string]int `mapstructure:"queues" yaml:"queues"`
	// AuditMaxRetry 审计日志队列的最大重试次数
	AuditMaxRetry int `mapstructure:"audit_max_retry" yaml:"audit_max_retry"`
	// AuditTimeoutSeconds 审计日志任务的超时时间（秒）
	AuditTimeoutSeconds int `mapstructure:"audit_timeout_seconds" yaml:"audit_timeout_seconds"`
	// AuditFallbackBufferSize 队列降级路径的内存缓冲容量。
	// <=0 时使用默认值 1024。满时优先丢弃低风险的最旧条目，再不能丢弃则同步落库兜底。
	AuditFallbackBufferSize int `mapstructure:"audit_fallback_buffer_size" yaml:"audit_fallback_buffer_size"`
	// AuditFallbackBatchSize 批量 INSERT 的最大条数。<=0 时使用默认值 200。
	AuditFallbackBatchSize int `mapstructure:"audit_fallback_batch_size" yaml:"audit_fallback_batch_size"`
	// AuditFallbackFlushIntervalMs 批量 INSERT 的最长等待间隔（毫秒）。<=0 时使用默认值 1000。
	AuditFallbackFlushIntervalMs int `mapstructure:"audit_fallback_flush_interval_ms" yaml:"audit_fallback_flush_interval_ms"`
	// AuditSampleGetKeepEvery 对低风险只读请求（GET 且 2xx 且 is_high_risk=0）保留 1/N 条。
	// <=1 时表示全量保留（不采样）。默认 10 即保留 1/10。
	AuditSampleGetKeepEvery int `mapstructure:"audit_sample_get_keep_every" yaml:"audit_sample_get_keep_every"`
	// AuditSampleSlowKeepThresholdMs 低风险只读请求即便命中采样，若执行耗时 >= 该阈值（毫秒）也强制保留，
	// 避免慢请求被采样丢弃而漏掉性能问题线索。<=0 时使用默认值 1000（即 >=1s 的慢请求始终保留）。
	AuditSampleSlowKeepThresholdMs int `mapstructure:"audit_sample_slow_keep_threshold_ms" yaml:"audit_sample_slow_keep_threshold_ms"`
	// AuditLogRetentionDays 审计日志保留天数，超过此天数的 request_logs 和 admin_login_logs 将被清理。
	// 最小值 7 天，<=0 时使用默认值 30。
	AuditLogRetentionDays int `mapstructure:"audit_log_retention_days" yaml:"audit_log_retention_days"`
}

// Queue 队列默认配置。
var Queue = QueueConfig{
	Enable:          false, // 默认关闭队列，同步执行
	UseDefaultRedis: true,  // 默认复用全局 redis 配置
	Redis: QueueRedisConfig{
		Host:     "127.0.0.1",
		Port:     "6379",
		Password: "",
		Database: 0,
	},
	Namespace:      "go_layout",
	Concurrency:    8,     // 整个 worker 最多同时处理 8 个任务（全局上限）
	StrictPriority: false, // 按权重比例调度，非严格优先级
	// 默认不让 worker 消费 cron 类型任务；需要清理历史残留时再显式开启。
	ConsumeCronFallback: false,
	Queues: map[string]int{
		// 权重值表示调度概率，不是协程数量！
		// 所有队列共享 8 个协程，权重越高越容易被优先调度
		// 总权重 = 4+2+2+2+1 = 11
		// critical 被选中的概率 ≈ 4/11 ≈ 36%
		"critical": 4, // 权重 4，约 44% 概率被调度（如支付回调）
		"default":  2, // 权重 2，约 22% 概率被调度（普通任务）
		"audit":    2, // 权重 2，约 18% 概率被调度（审计日志）
		"export":   2, // 权重 2，约 18% 概率被调度（导出任务）
		"low":      1, // 权重 1，约 9% 概率被调度（批量通知）
	},
	AuditMaxRetry:                  3,    // 审计日志失败最多重试 3 次
	AuditTimeoutSeconds:            10,   // 审计日志任务超时 10 秒
	AuditFallbackBufferSize:        1024, // 降级路径内存缓冲容量
	AuditFallbackBatchSize:         200,  // 批量 INSERT 单批最大条数
	AuditFallbackFlushIntervalMs:   1000, // 批量 INSERT 最长等待间隔（毫秒）
	AuditSampleGetKeepEvery:        10,   // 低风险只读请求保留 1/10
	AuditSampleSlowKeepThresholdMs: 1000, // 慢请求（>=1s）即便命中采样也强制保留
	AuditLogRetentionDays:          30,    // 审计日志默认保留 30 天
}
