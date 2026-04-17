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
	// Queues 各队列的权重配置，key 为队列名，value 为权重值
	// 新增队列必须在此配置，否则 Worker 不会消费该队列的任务！
	// 权重决定任务被调度的概率，不是分配的协程数量！
	// 所有队列共享 Concurrency 个协程，权重越高越容易被优先调度
	// 调度概率 = 该队列权重 / 所有队列权重之和
	// 示例（总权重=4+2+2+1=9）：
	//   critical: 权重 4 → 4/9≈44% 概率被调度（支付回调、短信发送）
	//   default:  权重 2 → 2/9≈22% 概率被调度（普通异步任务）
	//   audit:    权重 2 → 2/9≈22% 概率被调度（请求日志、登录日志）
	//   low:      权重 1 → 1/9≈11% 概率被调度（批量通知、数据导出）
	Queues map[string]int `mapstructure:"queues" yaml:"queues"`
	// AuditMaxRetry 审计日志队列的最大重试次数
	AuditMaxRetry int `mapstructure:"audit_max_retry" yaml:"audit_max_retry"`
	// AuditTimeoutSeconds 审计日志任务的超时时间（秒）
	AuditTimeoutSeconds int `mapstructure:"audit_timeout_seconds" yaml:"audit_timeout_seconds"`
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
	Queues: map[string]int{
		// 权重值表示调度概率，不是协程数量！
		// 所有队列共享 8 个协程，权重越高越容易被优先调度
		// 总权重 = 4+2+2+1 = 9
		// critical 被选中的概率 ≈ 4/9 ≈ 44%
		"critical": 4, // 权重 4，约 44% 概率被调度（如支付回调）
		"default":  2, // 权重 2，约 22% 概率被调度（普通任务）
		"audit":    2, // 权重 2，约 22% 概率被调度（审计日志）
		"low":      1, // 权重 1，约 11% 概率被调度（批量通知）
	},
	AuditMaxRetry:       3,  // 审计日志失败最多重试 3 次
	AuditTimeoutSeconds: 10, // 审计日志任务超时 10 秒
}
