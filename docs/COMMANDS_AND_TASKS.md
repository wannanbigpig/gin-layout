# 命令、定时任务与队列使用说明

本文档集中说明以下内容：

- 项目支持的命令及用途
- 如何启动 `service`、`worker`、`cron`
- 如何新增定时任务
- 如何创建、发布、消费异步队列任务
- 如何把任务放入指定队列，以及如何配置多队列

如果你刚接手项目，建议先看“运行模型”和“命令说明”，再看“定时任务”和“队列”两节。

## 运行模型

当前项目把后台运行能力拆成了 3 类进程：

- `service`
  - 提供 HTTP API
  - 处理登录、权限、菜单、上传、日志等业务
  - 某些场景下会往队列里发布异步任务

- `worker`
  - 消费异步任务
  - 当前已经接入请求审计日志异步落库

- `cron`
  - 负责周期性调度
  - 当前使用 `robfig/cron`
  - 适合“每天凌晨 2 点执行一次”这类固定时间任务

可以把它理解成：

- `service` 负责对外服务
- `worker` 负责后台消费
- `cron` 负责按时间触发

## 命令说明

项目根命令入口在 [cmd/root.go](/Users/liuml/data/go/src/go-layout/cmd/root.go)。

### 0. 迁移命令

常用入口：

```bash
go run main.go -c ./config.yaml command migrate check
go run main.go -c ./config.yaml command migrate up
```

详细说明见：[docs/MIGRATE_COMMANDS.md](/Users/liuml/data/go/src/go-layout/docs/MIGRATE_COMMANDS.md)。

### 1. 查看帮助

```bash
go run main.go -h
go run main.go command -h
```

### 2. 启动 API 服务

```bash
go run main.go service
```

用途：

- 启动 Gin HTTP 服务
- 加载配置、日志、数据库等基础资源
- 提供业务 API

常见场景：

- 本地开发
- 单机部署 API 服务
- 容器内主应用进程

### 3. 启动异步任务消费进程

```bash
go run main.go worker
```

入口文件：

- [cmd/worker/worker.go](/Users/liuml/data/go/src/go-layout/cmd/worker/worker.go)

用途：

- 连接 Redis
- 注册所有异步任务处理器
- 启动 Asynq worker，消费队列中的任务

当前已接入任务：

- `audit:request_log.write`

### 4. 启动定时任务调度器

```bash
go run main.go cron
```

入口文件：

- [cmd/cron/cron.go](/Users/liuml/data/go/src/go-layout/cmd/cron/cron.go)

用途：

- 启动周期调度器
- 注册当前定义的周期任务
- 收到退出信号后优雅关闭

### 5. 运行一次性命令

```bash
go run main.go command api-route
go run main.go command rebuild-user-permissions
go run main.go command init-system
go run main.go -c ./config.yaml command migrate up
```

入口文件：

- [cmd/command/command.go](/Users/liuml/data/go/src/go-layout/cmd/command/command.go)

支持的子命令：

- `api-route`
  - 扫描声明式路由树并重建 `api` 路由表
- `rebuild-user-permissions`
  - 按数据库关系重建用户最终 API 权限
- `init-system`
  - 回滚迁移、重新执行迁移、初始化 API 路由、重建用户权限
- `demo`
  - 示例命令
- `migrate`
  - 迁移管理子命令，支持 `create/check/up/down/goto/force/version`
  - 详细说明见 [docs/MIGRATE_COMMANDS.md](/Users/liuml/data/go/src/go-layout/docs/MIGRATE_COMMANDS.md)

### 6. 查看版本

```bash
go run main.go version
```

## 常见启动组合

### 1. 只启动 API

适合：

- 本地只调接口
- 不依赖异步任务的场景

```bash
go run main.go service
```

### 2. 启动 API + Worker

适合：

- 需要消费异步任务
- 已启用 Redis

```bash
go run main.go service
go run main.go worker
```

### 3. 启动 API + Worker + Cron

适合：

- 同时需要接口服务、异步消费和定时调度

```bash
go run main.go service
go run main.go worker
go run main.go cron
```

## 定时任务使用方式

当前定时任务代码分成 3 个文件：

- [cmd/cron/cron.go](/Users/liuml/data/go/src/go-layout/cmd/cron/cron.go)
  - 只负责启动与关闭
- [cmd/cron/schedule.go](/Users/liuml/data/go/src/go-layout/cmd/cron/schedule.go)
  - 提供任务声明 DSL
- [cmd/cron/tasks.go](/Users/liuml/data/go/src/go-layout/cmd/cron/tasks.go)
  - 集中定义当前有哪些周期任务

### 当前写法

当前项目推荐把所有任务写在 `defineSchedule` 里：

```go
func defineSchedule(schedule *Scheduler) {
	schedule.Call("demo", runTask).
		EveryFiveSeconds().
		WithoutOverlapping()

	cfg := config.GetConfig()
	if cfg != nil && cfg.EnableResetSystemCron {
		schedule.CallE("reset-system-data", system.ReinitializeSystemData).
			DailyAt("02:00:00").
			WithoutOverlapping()
	}
}
```

这样做的好处是：

- 新任务统一在一个地方声明
- 名称、时间规则、是否允许重入一眼能看出来
- 高风险任务可以显式配置启用，默认不注册
- 不需要每次手写 `AddJob`、`Chain`、`Recover`

### 可用方法

目前已经支持：

- `Call(name, func())`
  - 注册无返回值任务
- `CallE(name, func() error)`
  - 注册返回 `error` 的任务
- `Cron(spec)`
  - 直接使用 cron 表达式
- `EveryFiveSeconds()`
  - 每 5 秒执行一次
- `DailyAt("02:00:00")`
  - 每天固定时间执行
- `WithoutOverlapping()`
  - 任务运行期间不允许重入
- `AllowOverlap()`
  - 允许重入

### 示例 1：每 10 分钟执行一次

```go
schedule.Call("cleanup-cache", cleanupCache).
	Cron("0 */10 * * * *").
	WithoutOverlapping()
```

### 示例 2：每天凌晨 3 点执行

```go
schedule.CallE("sync-report", reportService.SyncDailyReport).
	DailyAt("03:00:00").
	WithoutOverlapping()
```

### 示例 3：允许重入

```go
schedule.Call("heartbeat", heartbeat).
	Cron("0/30 * * * * *").
	AllowOverlap()
```

### 新增定时任务步骤

1. 在业务层准备好任务函数
2. 如果函数返回 `error`，直接使用 `CallE`
3. 在 [tasks.go](/Users/liuml/data/go/src/go-layout/cmd/cron/tasks.go) 的 `defineSchedule` 中新增一行
4. 重启 `cron` 进程

## 队列使用说明

当前队列基于 `Asynq`，但业务层不直接依赖 `asynq.Client`，而是使用项目自己的统一接口。

核心代码：

- [internal/queue/queue.go](/Users/liuml/data/go/src/go-layout/internal/queue/queue.go)
- [internal/queue/asynqx/asynq.go](/Users/liuml/data/go/src/go-layout/internal/queue/asynqx/asynq.go)
- [internal/jobs/registry.go](/Users/liuml/data/go/src/go-layout/internal/jobs/registry.go)

### 队列完整链路

一条异步任务的执行链路如下：

1. 业务代码构造 payload
2. 调用 `queue.PublishJSON(...)` 发布任务
3. 任务进入 Redis 对应队列
4. `worker` 启动后注册任务处理器
5. Asynq 从 Redis 拉取任务
6. 调用对应 handler 执行业务逻辑

### 当前推荐 API

发布任务：

```go
_, err := queue.PublishJSON(ctx, taskType, queueName, payload, opts...)
```

注册消费：

```go
queue.RegisterJSON(registry, taskType, func(ctx context.Context, payload PayloadType) error {
	return handle(payload)
})
```

相比旧写法，这样有几个好处：

- 不需要单独实现一个 `Job` 结构体
- 不需要手动做 `json.Marshal` / `json.Unmarshal`
- payload 结构更直观
- 新任务更容易复制模板

## 如何创建一个新队列任务

下面用“发送邮件”举例。

### 1. 定义 payload

```go
type EmailPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

func (p EmailPayload) Validate() error {
	if p.To == "" {
		return errors.New("to is required")
	}
	return nil
}
```

如果 payload 实现了 `Validate() error`，消费前会自动校验。校验失败会按“不重试”处理。

### 2. 发布任务

```go
func EnqueueEmail(ctx context.Context, payload EmailPayload) error {
	_, err := queue.PublishJSON(
		ctx,
		"notify:email.send",
		"default",
		payload,
		queue.WithMaxRetry(5),
		queue.WithTimeout(30*time.Second),
	)
	return err
}
```

这里的参数分别表示：

- `"notify:email.send"`
  - 任务类型
- `"default"`
  - 队列名
- `payload`
  - 任务数据
- `WithMaxRetry`
  - 最大重试次数
- `WithTimeout`
  - 执行超时时间

### 3. 注册消费处理器

```go
func registerEmail(registry queue.Registry) {
	queue.RegisterJSON(registry, "notify:email.send", func(ctx context.Context, payload EmailPayload) error {
		return sendEmail(payload)
	})
}
```

### 4. 注册到统一入口

当前推荐把所有任务注册放在一个地方，例如：

```go
func RegisterAll(registry queue.Registry) {
	queue.RegisterJSON(registry, AuditLogTaskType, handleAuditLog)
	registerEmail(registry)
}
```

### 5. Worker 自动消费

`worker` 启动时会调用：

```go
registry := jobs.NewRegistry()
server, mux, err := asynqx.NewServer(cfg, registry)
```

所以只要你的任务已经注册进 `RegisterAll`，worker 就会自动消费。

## 如何把任务放到指定队列

看 `PublishJSON` 的第三个参数：

```go
queue.PublishJSON(ctx, "notify:email.send", "critical", payload)
```

这里的 `"critical"` 就是队列名。

常见用法：

- 审计日志放 `audit`
- 普通任务放 `default`
- 高优先级任务放 `critical`
- 低优先级清理任务放 `low`

## 如何配置多队列

配置文件示例：

```yaml
queue:
  enable: true
  namespace: go_layout
  concurrency: 8
  strict_priority: false
  queues:
    critical: 4
    default: 2
    audit: 2
    low: 1
  audit_max_retry: 3
  audit_timeout_seconds: 10
```

含义：

- `queues`
  - 声明 worker 需要监听哪些队列
- 数字
  - 表示各队列的权重
- `concurrency`
  - worker 并发度
- `strict_priority`
  - 是否严格优先消费高优先级队列

### 多队列示例

```yaml
queue:
  queues:
    critical: 5
    default: 3
    audit: 2
    low: 1
```

你可以这样分配：

- `critical`
  - 权限修复、账号状态同步
- `default`
  - 普通异步业务
- `audit`
  - 请求审计日志
- `low`
  - 清理、统计、补偿任务

## 当前项目中的真实示例

当前已经接入的任务是审计日志任务：

- 任务类型：`audit:request_log.write`
- 队列：`audit`

对应文件：

- [internal/jobs/audit_log.go](/Users/liuml/data/go/src/go-layout/internal/jobs/audit_log.go)

发布端：

```go
func EnqueueAuditLog(ctx context.Context, kind string, snapshot *auditsvc.AuditLogSnapshot) error {
	payload, err := NewAuditLogPayload(kind, snapshot)
	if err != nil {
		return err
	}
	_, err = queue.PublishJSON(ctx, AuditLogTaskType, AuditQueueName, payload, auditLogOptions()...)
	return err
}
```

消费端：

```go
func RegisterAll(registry queue.Registry) {
	queue.RegisterJSON(registry, AuditLogTaskType, handleAuditLog)
}
```

worker 启动：

```go
registry := jobs.NewRegistry()
server, mux, err := asynqx.NewServer(cfg, registry)
```

## 何时使用 Cron，何时使用 Queue

建议按职责区分：

- `cron`
  - 负责“什么时候触发”
  - 适合固定时间周期任务
- `queue`
  - 负责“任务如何异步执行”
  - 适合削峰、重试、异步消费

### 典型场景

用 `cron`：

- 每天凌晨重置系统数据
- 每小时清理缓存
- 每 5 分钟同步统计

用 `queue`：

- 审计日志落库
- 邮件发送
- Webhook 投递
- 权限增量同步

组合使用：

- `cron` 到点触发
- `cron` 内部把任务投递到 `queue`
- `worker` 真正执行任务

## 常见问题

### 1. 为什么发布了任务，但没有被消费？

先检查：

- Redis 是否可用
- `queue.enable` 是否开启
- `worker` 是否已启动
- 任务是否已经注册到 `RegisterAll`

### 2. 为什么任务校验失败后不重试？

因为 `RegisterJSON` 会把 payload 反序列化失败或 `Validate()` 失败视为“无效任务”，这类问题通常重试没有意义。

### 3. 为什么任务进了带前缀的物理队列？

因为当前实现支持 `namespace`，例如：

- 配置队列名：`audit`
- 实际 Redis 队列名：`go_layout:audit`

业务代码里仍然使用逻辑队列名即可。

### 4. 什么时候需要自己实现 `queue.Job`？

大多数业务场景都不需要。

只有当你需要：

- 自定义更复杂的 payload 生成过程
- 不想走 JSON
- 需要对任务对象做特殊封装

才建议自己实现 `queue.Job`。

默认情况下，优先使用：

- `queue.PublishJSON`
- `queue.RegisterJSON`

## 推荐实践

- 定时任务统一写在 [tasks.go](/Users/liuml/data/go/src/go-layout/cmd/cron/tasks.go)
- 异步任务统一写在 `internal/jobs`
- 一个任务类型只对应一个清晰的 payload
- payload 尽量保持小而稳定
- 高优先级任务和低优先级任务分队列
- 不要在业务代码里直接使用 Asynq 的底层 API
- 优先使用项目封装好的 `PublishJSON` / `RegisterJSON`
