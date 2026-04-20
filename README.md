# <div align="center">gin-layout</div>

<div align="center">
  <strong>中文</strong> | <a href="./README.en.md">English</a>
</div>

<br />

<div align="center">
  <strong>基于 Gin 的后台管理系统脚手架</strong>
</div>

<div align="center">
  内置 JWT 认证、RBAC 权限、请求/登录日志、文件上传、readiness 探针、参数校验、声明式路由和 CLI 初始化命令。
</div>

<br />

<div align="center">
  <img src="https://github.com/wannanbigpig/gin-layout/actions/workflows/go.yml/badge.svg" alt="go" />
  <img src="https://github.com/wannanbigpig/gin-layout/actions/workflows/codeql.yml/badge.svg" alt="codeql" />
  <img src="https://goreportcard.com/badge/github.com/wannanbigpig/gin-layout" alt="go report card" />
  <img src="https://img.shields.io/github/license/wannanbigpig/gin-layout" alt="license" />
  <img src="https://img.shields.io/badge/Go-%3E%3D1.23-blue.svg" alt="go version" />
</div>

<br />

## 项目定位

很多后台项目一开始都只是想“先把登录、权限、菜单、上传和日志跑起来”，但真正进入开发后，通常会很快碰到这些重复问题：

- 认证、权限、日志和文件能力分散，初始化成本高
- 路由、菜单、API 权限关系容易逐步失控
- 不同项目里同一套后台基础设施被反复重写
- 配置、命令、迁移和部署流程缺少统一约定

`gin-layout` 的目标很明确：把后台管理场景里高频、重复、工程化要求高的基础能力沉淀成一套可以直接落地的后端骨架。

## 核心特性

| 能力 | 说明 |
| --- | --- |
| Auth | 内置 JWT 登录、Token 校验、自动刷新、黑名单 |
| RBAC | 管理员、角色、部门、菜单、API 权限管理 |
| Route Metadata | 声明式路由树统一生成 Gin 路由和 API 元数据 |
| Logs | 内置登录日志、请求日志、统一响应结构 |
| File Access | 文件上传与公开 / 私有文件访问 |
| Health Probes | 提供 `/ping` 与 `/health/readiness`，便于存活检测与依赖就绪检查 |
| Tooling | 提供 CLI 初始化、路由同步、权限重建、迁移配套能力 |
| Hot Reload | 支持部分配置热更新，失败时保留旧实例继续运行 |

## 相关资源

- 前端项目：[go-admin-ui](https://github.com/wannanbigpig/go-admin-ui)
- 在线文档：[Apifox](https://wannanbigpig.apifox.cn/)
- 演示地址：[在线演示](https://x-l-admin.wannanbigpig.com/)
- 命令与任务文档：[docs/COMMANDS_AND_TASKS.md](./docs/COMMANDS_AND_TASKS.md)

## 快速开始

### 1. 环境要求

- `Go >= 1.23`
- `MySQL >= 5.7`
- `Redis >= 5.0`（可选）
- [`migrate`](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate)

### 2. 安装项目

```bash
git clone https://github.com/wannanbigpig/gin-layout.git
cd gin-layout
go mod download
```

### 3. 执行迁移

```bash
migrate -database 'mysql://root:root@tcp(127.0.0.1:3306)/go_layout?charset=utf8mb4&parseTime=True&loc=Local' \
  -path data/migrations up
```

迁移执行完成后会写入一套默认基础数据，其中包含超级管理员账号 `super_admin / 123456`。仅建议用于本地初始化，首次登录后请立即修改密码。

### 4. 配置项目

源码运行时建议带上 `GO_ENV=development`。未显式传入 `-c` 时：

- 开发模式会把当前工作目录下的 `config/config.yaml.example` 自动复制为项目根目录 `config.yaml`
- 构建后的二进制会在可执行文件同级查找 `config.yaml`，若不存在则尝试从同目录 `config.yaml.example` 复制

也可以手动复制配置文件后再修改。

最小配置示例：

```yaml
app:
  app_env: local
  debug: true
  trusted_proxies:
    - 127.0.0.1
  watch_config: true
  # allow_degraded_startup: false

jwt:
  ttl: 7200
  refresh_ttl: 3600
  secret_key: change-me-to-a-random-secret

mysql:
  enable: true
  host: 127.0.0.1
  port: 3306
  database: go_layout
  username: root
  password: your_password

redis:
  enable: true
  host: 127.0.0.1
  port: 6379
  password: ""
  database: 0

queue:
  enable: true
  use_default_redis: true
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

注意：

- `jwt.secret_key` 为必填项，不能为空
- 如果只启动 API、不启用异步任务，可以把 `queue.enable` 设为 `false`
- 如果 `queue.enable=true` 但不想复用 `redis.*`，请把 `queue.use_default_redis` 设为 `false`，并补齐 `queue.redis.*`

### 5. 启动服务

```bash
GO_ENV=development go run main.go service
```

需要显式指定监听地址或端口时：

```bash
GO_ENV=development go run main.go service -H 127.0.0.1 -P 9001
```

如果启用了 `queue.enable=true`，还需要单独启动 worker：

```bash
GO_ENV=development go run main.go worker
```

### 6. 验证服务

```bash
curl http://127.0.0.1:9001/ping
curl http://127.0.0.1:9001/health/readiness
```

- `/ping` 返回 `pong`，说明 HTTP 进程已正常启动
- `/health/readiness` 返回 `ready=true`，说明当前配置下需要的依赖已经就绪

## 设计思路

### 声明式路由优先

后台路由维护在一棵声明式路由树中，目前入口位于 `internal/routers/admin_router.go` 的 `AdminRouteTree()`。Gin 路由注册和 API 元数据初始化都从这棵树生成，避免“代码路由”和“权限路由”长期分叉。

### 数据库关系是权限真相

当前权限模型采用“数据库关系为真相，Casbin 负责最终接口判定”的方式。角色、部门、菜单和 API 的业务关系以数据库为准，`rebuild-user-permissions` 命令会按这些关系重建用户最终 API 权限。

### 配置热更新是分级的

项目支持配置热更新，但不是所有配置都会在运行中立即生效。支持热更新的资源会尝试重建；如果重建失败，会继续保留旧实例运行，避免把服务直接打挂。

## 常用命令

查看帮助：

```bash
go run main.go -h
go run main.go command -h
go run main.go service --help
```

常用命令：

| 命令 | 说明 |
| --- | --- |
| `go run main.go version` | 输出当前版本号 |
| `go run main.go service` | 启动 API 服务 |
| `go run main.go service -H 0.0.0.0 -P 9001` | 显式指定监听地址与端口 |
| `go run main.go worker` | 启动 Asynq 异步任务消费进程 |
| `go run main.go cron` | 启动定时任务 |
| `go run main.go command demo` | 运行示例命令 |
| `go run main.go command api-route -y` | 扫描声明式路由树并重建 `api` 路由表 |
| `go run main.go command rebuild-user-permissions -y` | 按数据库关系重建用户最终 API 权限 |
| `go run main.go command init-system -y` | 回滚并重新执行迁移、重建 API 路由、重建用户权限 |

如果配置文件不在默认位置，可以显式指定：

```bash
go run main.go -c /path/to/config.yaml service
go run main.go -c /path/to/config.yaml command init-system
```

补充说明：

- `api-route`、`rebuild-user-permissions`、`init-system` 默认会二次确认；自动化场景建议显式加 `-y`
- `init-system` 会清空并重建系统数据，只适合本地初始化或明确允许重置的环境

## 配置说明

### 配置查找顺序

配置文件查找顺序：

1. 显式传入 `-c` / `--config`
2. `GO_ENV=development` 时，使用当前工作目录的 `config.yaml`
3. 若第 2 步缺失，则尝试从当前工作目录的 `config/config.yaml.example` 复制生成
4. 非开发模式下，使用可执行文件所在目录的 `config.yaml`
5. 若第 4 步缺失，则尝试从可执行文件同级的 `config.yaml.example` 复制生成

### 主要配置项

| 配置项 | 说明 |
| --- | --- |
| `app.base_path` | 日志、上传文件等本地路径的基础目录，默认取可执行文件所在目录 |
| `app.allow_degraded_startup` | 仅 `service` 命令生效；依赖初始化失败时允许 HTTP 服务先启动，并通过 readiness / 路由守卫暴露未就绪状态 |
| `app.base_url` | 文件访问 URL 前缀，用于生成公开文件地址 |
| `app.trusted_proxies` | 受信任代理地址或网段，影响 `ClientIP()` 与日志 IP |
| `jwt.secret_key` | 必填；生产环境不能使用弱占位值 |
| `jwt.ttl` / `jwt.refresh_ttl` | Token 过期时间与自动刷新阈值 |
| `mysql` | 数据库开关与连接信息 |
| `redis` | 缓存、黑名单和分布式锁配置 |
| `queue.use_default_redis` | `true` 复用 `redis.*`；`false` 时改用 `queue.redis.*` 独立连接 |
| `queue` | Asynq 异步任务开关、队列命名空间、并发度、优先级和审计日志重试策略 |
| `logger` | 日志输出、切割和保留策略 |

如果通过 Nginx、Ingress 或负载均衡转发请求，需要同步配置 `app.trusted_proxies`，否则客户端 IP 可能记录不准确。

### Worker 与 Cron

- `service` 负责提供 HTTP API。
- `worker` 负责消费 Asynq 异步任务。当前首版只接入请求审计日志异步落库。
- `queue.enable=false` 时，不需要启动 `worker`，请求审计日志会退回同步写库。
- `cron` 当前默认注册了 `demo`（每 5 秒打印一次日志）和 `reset-system-data`（每天 `02:00:00` 执行一次系统重建）。
- 不要把同一个周期任务同时注册到 `cron` 和 `worker` 体系里，否则会重复执行。

注意：`reset-system-data` 当前调用的是 `system.ReinitializeSystemData()`，会回滚迁移并重建系统数据。直接启用 `cron` 前，请务必先检查 [cmd/cron/tasks.go](/Users/liuml/data/go/src/go-layout/cmd/cron/tasks.go) 是否符合你的环境预期。

### 热更新

启用方式：

```yaml
app:
  watch_config: true
```

支持热更新：

- `logger.*`
- `mysql.*`
- `redis.*`
- `app.base_url`
- `app.cors_*`
- `jwt.ttl`
- `jwt.refresh_ttl`

仅检测并提示“需要重启”：

- `app.trusted_proxies`
- `app.language`
- `app.allow_degraded_startup`
- `jwt.secret_key`
- 服务监听地址与端口
- 路由结构

说明：

- `watch_config=true` 只表示启用监听，不代表所有配置都能无损切换
- MySQL、Redis、Casbin 会按新配置重建实例，失败时保留旧实例
- JWT 密钥当前不支持热更新，修改后会记录告警并继续使用旧密钥，直到进程重启

## 开发指南

### 新增接口流程

1. 在 `internal/controller/` 编写控制器
2. 在 `internal/service/` 编写业务逻辑
3. 在 `internal/validator/form/` 定义请求参数
4. 在 `AdminRouteTree()` 中声明路由
5. 需要更新 API 路由表时执行 `go run main.go command api-route`

### 测试

```bash
go test ./...
go test ./tests/admin_test
```

测试会优先使用项目根目录 `config.yaml`。如果当前环境的 MySQL 或 Redis 不可用，会自动回退到示例配置运行可脱离外部依赖的测试。

## 部署说明

### 构建

```bash
go build -o go-layout main.go
./go-layout service
```

如果没有显式传 `-c`，请把 `config.yaml` 放在二进制同级目录；若只有 `config.yaml.example`，首次启动会自动复制生成 `config.yaml`。

### Supervisor

```ini
[program:go-layout]
command=/path/to/go-layout -c /path/to/config.yaml service
directory=/path/to/go-layout
autostart=true
autorestart=true
startsecs=5
user=www-data
redirect_stderr=true
stdout_logfile=/path/to/go-layout/supervisord.log
```

### Nginx

```nginx
server {
    listen 80;
    server_name api.example.com;

    location / {
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_pass http://127.0.0.1:9001;
    }
}
```

如果前面有反向代理，请把代理地址或网段加入 `app.trusted_proxies`。

## 目录结构

```text
gin-layout/
├── cmd/                    # 命令行入口
├── config/                 # 配置结构与示例配置
├── data/                   # MySQL / Redis 与迁移
├── docs/                   # 补充文档与资源
├── internal/
│   ├── access/             # 权限基础设施
│   ├── controller/         # 控制器
│   ├── middleware/         # 中间件
│   ├── model/              # 数据模型
│   ├── resources/          # 资源转换
│   ├── routers/            # 声明式路由
│   ├── service/            # 业务逻辑
│   └── validator/          # 参数验证
├── pkg/                    # 通用工具
├── storage/                # 文件存储
├── tests/                  # 路由与集成测试
└── README.md
```

## 💝 赞助项目

感谢你使用 `gin-layout`。

如果这个项目对你有帮助，欢迎支持项目的持续开发与维护。

<a href="./docs/DONATE.md">
  <img src="https://img.shields.io/badge/BUY_ME_A_COFFEE-%E6%94%AF%E6%8C%81%E4%BD%9C%E8%80%85-f08a24?style=for-the-badge&logo=buymeacoffee&logoColor=ffdd00&labelColor=4a4a4a" alt="支持作者" />
</a>

## 许可证

本项目采用 MIT 许可证，详见 [LICENSE](LICENSE)。

## 贡献

欢迎提交 Issue 和 Pull Request。
