<div align="center">
  <img src="./docs/images/gin-layout-mark.svg" width="88" height="88" alt="Gin Layout Logo" />
  <h1>gin-layout</h1>
</div>

<div align="center">
  <strong>中文</strong> | <a href="./README.en.md">English</a>
</div>

<br />

<div align="center">
  <strong>基于 Gin 的后台管理系统脚手架</strong>
</div>

<div align="center">
  内置 JWT 双 Token 认证、RBAC 权限、请求/登录日志、文件上传、任务中心、系统通知、readiness 探针、参数校验、请求语言国际化、声明式路由和 CLI 命令体系。
</div>

<br />

<div align="center">
  <img src="https://github.com/wannanbigpig/gin-layout/actions/workflows/go.yml/badge.svg" alt="go" />
  <img src="https://github.com/wannanbigpig/gin-layout/actions/workflows/codeql.yml/badge.svg" alt="codeql" />
  <img src="https://goreportcard.com/badge/github.com/wannanbigpig/gin-layout" alt="go report card" />
  <img src="https://img.shields.io/github/license/wannanbigpig/gin-layout" alt="license" />
  <img src="https://img.shields.io/badge/Go-%3E%3D1.27-blue.svg" alt="go version" />
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
| Auth | 内置 JWT 双 Token 登录、Access Token 校验、Refresh Token Cookie 刷新、黑名单 |
| RBAC | 管理员、角色、部门、菜单、API 权限管理 |
| Route Metadata | 声明式路由树统一生成 Gin 路由和 API 元数据 |
| Logs | 内置登录日志、请求日志、统一响应结构 |
| Personal Workspace | 个人工作台、最近操作与可扩展的个人 / 业务待办 |
| File Access | 文件上传与公开 / 私有文件访问 |
| Task Center | 内置异步任务、定时任务定义、运行记录、重试与运营配置 |
| Realtime | 系统通知、WebSocket 临时票据与实时消息连接 |
| I18n | 基于 `Accept-Language` 返回错误文案与菜单标题（支持 `zh-CN` / `en-US`） |
| Health Probes | 提供 `/ping` 与 `/health/readiness`，便于存活检测与依赖就绪检查 |
| Tooling | 提供 CLI 初始化、路由同步、权限重建、迁移配套能力 |
| Hot Reload | 支持部分配置热更新，失败时保留旧实例继续运行 |

## 相关资源

- 前端项目：[go-admin-ui](https://github.com/wannanbigpig/go-admin-ui)
- 在线文档：[Apifox](https://wannanbigpig.apifox.cn/)
- 演示地址：[在线演示](https://x-l-admin.wannanbigpig.com/)
- 命令与任务文档：[docs/COMMANDS_AND_TASKS.md](./docs/COMMANDS_AND_TASKS.md)
- 迁移命令详解：[docs/MIGRATE_COMMANDS.md](./docs/MIGRATE_COMMANDS.md)
- 前后端兼容与版本策略：[docs/COMPATIBILITY.md](./docs/COMPATIBILITY.md)

## 快速开始

### 1. 环境要求

- `Go >= 1.27`
- `MySQL >= 8.0`
- `Redis >= 6.2`（可选；启用 WebSocket ticket、缓存、队列或分布式锁时需要）

### 2. 安装项目

```bash
git clone https://github.com/wannanbigpig/gin-layout.git
cd gin-layout
go mod download
```

### 3. 执行迁移

推荐使用项目命令：

```bash
go run main.go -c ./config.yaml command migrate        # 默认等价于 migrate up
go run main.go -c ./config.yaml command migrate check
```

迁移执行完成后会写入一套默认基础数据，其中包含超级管理员账号 `super_admin / 123456`。仅建议用于本地初始化，首次登录后请立即修改密码；个人改密必须提供当前密码，成功后会撤销该用户的全部既有会话。

迁移文件创建、时间戳命名规范、`down/goto/force/version` 等详细说明见：[docs/MIGRATE_COMMANDS.md](./docs/MIGRATE_COMMANDS.md)。

### 4. 配置项目

源码运行时建议带上 `GO_ENV=development`。未显式传入 `-c` 时：

- 开发模式会把当前工作目录下的 `config/config.yaml.example` 自动复制为项目根目录 `config.yaml`
- 非开发模式会在可执行文件同级查找 `config.yaml`，若不存在则尝试从同目录 `config.yaml.example` 复制

也可以手动复制配置文件后再修改。

最小配置示例：

```yaml
app:
  app_env: local
  debug: true
  language: zh_CN
  trusted_proxies:
    - 127.0.0.1
  watch_config: false
  # allow_degraded_startup: false

jwt:
  ttl: 1800
  refresh_ttl: 0
  refresh_token_ttl: 604800
  refresh_cookie_name: refresh_token
  refresh_cookie_path: /admin/v1/auth
  refresh_cookie_same_site: Lax
  refresh_cookie_secure: false
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
    export: 2
    low: 1
  audit_max_retry: 3
  audit_timeout_seconds: 10
```

注意：

- `jwt.secret_key` 为必填项，不能为空
- `jwt.refresh_ttl` 已废弃，当前双 Token 方案实际使用 `jwt.refresh_token_ttl` 和 `jwt.refresh_cookie_*`
- 如果只启动 API、不启用异步任务，可以把 `queue.enable` 设为 `false`
- 如果 `queue.enable=true` 但不想复用 `redis.*`，请把 `queue.use_default_redis` 设为 `false`，并补齐 `queue.redis.*`
- 文件存储驱动和敏感存储参数运行时保存在 `sys_config`（`storage.active_driver` / `storage.config`），后台通过 `/admin/v1/system/storage/*` 管理；`config.yaml` 主要负责基础依赖与进程级配置

### 5. 启动服务

```bash
GO_ENV=development go run main.go -c ./config.yaml service
```

需要显式指定监听地址或端口时：

```bash
GO_ENV=development go run main.go -c ./config.yaml service -H 127.0.0.1 -P 9001
```

如果启用了 `queue.enable=true`，还需要单独启动 worker：

```bash
GO_ENV=development go run main.go -c ./config.yaml worker
```

本地开发如需热更新，项目已提供 Air 配置：

```bash
go install github.com/air-verse/air@latest
make dev-service
```

如果启用了 `queue.enable=true`，worker 也可以单独使用热更新：

```bash
make dev-worker
```

注意：

- `make dev-service` 使用根目录的 [`.air.toml`](./.air.toml) 启动 API 服务热更新。
- `make dev-service` / `make dev-worker` 都会显式传入 `-c ./config.yaml`，请先确保项目根目录已存在 `config.yaml`。
- `make dev-worker` 使用 [`.air.worker.toml`](./.air.worker.toml) 启动 worker 热更新，二进制和构建日志与 service 隔离，避免互相覆盖。
- 不要在运行 `make dev-worker` 的同时，再手动执行 `go run main.go worker` 或启动其他 worker 进程，否则会出现多个 worker 并发消费同一队列。
- worker 热更新会重启进程，执行中的任务可能被队列机制重试一次；这是开发环境的预期行为。

### 6. 验证服务

```bash
curl http://127.0.0.1:9001/ping
curl http://127.0.0.1:9001/health/readiness
```

- `/ping` 返回 `pong`，说明 HTTP 进程已正常启动
- `/health/readiness` 返回 `ready=true`，说明当前配置下需要的依赖已经就绪

## 设计思路

### 声明式路由优先

后台路由维护在一棵声明式路由树中，入口位于 `internal/routers/admin_router.go` 的 `AdminRouteTree()`。`internal/routers/catalog.go` 会一次性编译完整路径、分组、安全策略和 Handler 链，Gin 注册、API 元数据同步与 OpenAPI 都复用该 Catalog，避免多套递归逻辑长期分叉。

安全策略声明在路由组上，子路由默认继承。`api.is_auth` 仍保留为 `0/1/2` 兼容投影并由 `api-route` 同步，不允许在接口权限编辑页修改；新增路由时应选择 `SecurityPublic`、`SecurityRefreshCookie`、`SecurityWebSocketTicket`、`SecurityAccessToken` 或 `SecurityAccessTokenPermission`。

### 数据库关系是权限真相

当前权限模型采用“数据库关系为真相，Casbin 负责最终接口判定”的方式。角色、部门、菜单和 API 的业务关系以数据库为准，`rebuild-user-permissions` 命令会按这些关系重建用户最终 API 权限。

Casbin 策略刷新支持多个 `service` 实例。权限策略与全局版本在同一 MySQL 事务内提交；正常请求优先读取 Redis 版本，发现本机策略落后时同步回源 MySQL 后再判权。权限写事务会先设置 Redis dirty 标记，提交后更新版本并发布通知；其他实例收到通知会立即刷新。Redis 缺失、异常或 dirty 存在时，请求回源 MySQL，单个实例内的并发回源由 `singleflight` 合并；Redis 版本键保留 30 秒安全过期，定时回源仍作为补偿。版本读取或策略重载失败时 fail-closed。该机制提供正常路径近实时、异常路径有界最终一致，不是跨实例提交确认屏障。

升级到含权限版本校验的版本时，必须先执行 `command migrate up` 创建 `permission_policy_versions`，再启动或滚动替换 `service` 实例；缺少该表时权限判定会按 fail-closed 拒绝，并在日志中报告迁移缺失。

多实例必须使用同一 MySQL 数据库、同一 Redis 实例与逻辑库，并保持 `jwt.secret_key`、`app.app_env` 和 `queue.namespace` 一致。策略、系统参数缓存和其他内部 Pub/Sub 通道使用 `queue.namespace`、`app.app_env`、`redis.database` 组成隔离命名；不要让同一部署的进程使用不同值。系统参数缓存也在启动时回源、订阅通知，并每 30 秒回源兜底。

滚动升级若改变通道命名，旧版本和新版本不会互相收到通知；短时混用可能造成缓存或策略刷新延迟。此类版本应协调重启全部 `service`、`worker`、`cron` 进程后再恢复流量。

### 配置热更新是分级的

项目支持配置热更新，但不是所有配置都会在运行中立即生效。支持热更新的资源会尝试重建；如果重建失败，会继续保留旧实例运行，避免把服务直接打挂。

### 请求语言驱动文案

项目会在请求链路读取 `Accept-Language`（当前支持 `zh-CN` / `en-US`），并做归一化与降级处理：

- 错误码文案：按请求语言返回；无法识别时降级到默认语言（`zh-CN`）。
- 参数无效、配置缺失、资源状态冲突、权限不足等可预期业务错误返回明确且可国际化的安全文案；未分类异常、数据库/SQL 错误、凭据和供应商原始响应只记录到服务端日志，对客户端统一返回通用服务端错误，避免泄露内部信息。
- 菜单列表/用户菜单树：仅返回 `title`，由后端按请求语言解析后返回。
- 菜单详情：返回 `title_i18n`（用于前端编辑回填），不返回 `title`。
- 菜单新增/编辑：写接口使用 `title_i18n`，目前支持 `zh-CN` 与 `en-US`，两者至少一种非空。

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
| `go run main.go -c ./config.yaml service` | 启动 API 服务 |
| `go run main.go -c ./config.yaml service -H 0.0.0.0 -P 9001` | 显式指定监听地址与端口 |
| `go run main.go -c ./config.yaml worker` | 启动 Asynq 异步任务消费进程 |
| `go run main.go -c ./config.yaml cron` | 启动定时任务 |
| `go run main.go command demo` | 运行示例命令 |
| `go run main.go -c ./config.yaml command gen --from=gen/specs/product.yaml` | 从表结构或 YAML 生成 CRUD 代码 |
| `go run main.go -c ./config.yaml command gen-openapi` | 从声明式路由 Catalog 生成 `docs/openapi.yaml` |
| `go run main.go -c ./config.yaml command api-route -y` | 扫描声明式路由树并重建 `api` 路由表 |
| `go run main.go -c ./config.yaml command rebuild-user-permissions -y` | 按数据库关系重建用户最终 API 权限 |
| `go run main.go -c ./config.yaml command init-system` | 清空当前配置数据库对象并重建系统数据 |
| `go run main.go -c ./config.yaml command task sync-definitions` | 非破坏性同步内置任务定义到 `task_definitions` |
| `go run main.go -c ./config.yaml command task scan-async` | 扫描异步任务注册与任务定义镜像 |
| `go run main.go -c ./config.yaml command task scan-cron` | 扫描定时任务定义与任务定义镜像 |
| `go run main.go -c ./config.yaml command migrate check` | 校验迁移文件命名与 up/down 配对 |
| `go run main.go -c ./config.yaml command migrate up` | 执行全部未应用迁移 |
| `go run main.go -c ./config.yaml command migrate down 1` | 回滚 1 个迁移版本 |

如果配置文件不在默认位置，可以显式指定：

```bash
go run main.go -c /path/to/config.yaml service
go run main.go -c /path/to/config.yaml command init-system
```

迁移命令完整参数见：[docs/MIGRATE_COMMANDS.md](./docs/MIGRATE_COMMANDS.md)。

补充说明：

- `api-route`、`rebuild-user-permissions` 默认会二次确认；自动化场景可以显式加 `-y`
- `init-system` 是高危重置命令，会清空当前配置数据库对象并重建系统数据；执行前会用红色提示当前环境、MySQL host、port、database、username、table_prefix，并要求确认已备份和输入当前库名
- `init-system -y` 只跳过普通确认，不能单独跳过高危确认；自动化场景必须同时传入 `--backup-confirmed --confirm-database <mysql.database>`，例如：

```bash
go run main.go -c ./config.yaml command init-system -y --backup-confirmed --confirm-database go_layout_test
```

- 新增内置异步任务或定时任务上线后，不要执行 `init-system`；使用 `command task sync-definitions` 同步任务定义，再用 `scan-async` / `scan-cron` 检查任务是否已在表中。同步只更新代码定义字段，会保留线上 `status`、`cron_spec`、是否允许手动触发、是否允许重试和记录策略等运营配置。
- `20260916000001_add_refresh_session_lookup_index` 是独立的 refresh-token 查询索引迁移。部署时先执行 `go run main.go -c ./config.yaml command migrate up`，再在获授权的目标环境执行 `go run main.go -c ./config.yaml command api-route -y` 同步包含新路由的 API 表和权限。

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
| `app.base_path` | 日志、上传文件等本地路径的基础目录；未配置时默认按 `GO_ENV` 选择（development=当前工作目录，其他=可执行文件目录） |
| `app.max_request_body_mb` | 普通 API 请求体上限，默认 `16` MB |
| `app.max_upload_body_mb` | 本地 `multipart` 上传的整个请求体上限，默认 `256` MB；包含全部文件和表单字段，不替代 `storage.max_file_size_mb` 的单文件限制 |
| `app.allow_degraded_startup` | 仅 `service` 命令生效；依赖初始化失败时允许 HTTP 服务先启动，并通过 readiness / 路由守卫暴露未就绪状态 |
| `app.base_url` | 文件访问 URL 前缀，用于生成公开文件地址 |
| `app.trusted_proxies` | 受信任代理地址或网段，影响 `ClientIP()` 与日志 IP |
| `jwt.secret_key` | 必填；生产环境不能使用弱占位值 |
| `security.credential_encryption_key` | 保存短信、OSS 等业务凭据前必填的独立随机主密钥，建议通过 `${CREDENTIAL_ENCRYPTION_KEY}` 注入；各用途使用独立派生密钥，所有实例必须一致，变更需协调重启并迁移 `enc:v1` 密文 |
| `jwt.ttl` / `jwt.refresh_token_ttl` / `jwt.refresh_cookie_*` | Access Token 生命周期、Refresh Token Cookie 生命周期与浏览器策略；`jwt.refresh_ttl` 已废弃 |
| `mysql` | 数据库开关与连接信息 |
| `redis` | 缓存、黑名单和分布式锁配置 |
| `queue.use_default_redis` | `true` 复用 `redis.*`；`false` 时改用 `queue.redis.*` 独立连接 |
| `queue` | Asynq 异步任务开关、队列命名空间、并发度、优先级和审计日志重试策略 |
| `storage.active_driver` / `storage.config` | 运行时文件存储驱动与敏感配置，保存在 `sys_config`，由 `/admin/v1/system/storage/*` 管理；OSS AccessKey Secret 以 `enc:v1` AES-256-GCM 密文保存 |
| `todo.reminder_enabled` / `todo.reminder_sms_enabled` / `todo.reminder_lead_minutes` / `todo.reminder_catchup_minutes` | 待办提醒总开关、短信渠道总开关、提前提醒分钟数和 Cron 停机后的逾期补偿窗口，保存在 `sys_config`；短信默认关闭 |
| `logger` | 日志输出、切割和保留策略 |

如果通过 Nginx、Ingress 或负载均衡转发请求，需要同步配置 `app.trusted_proxies`，否则客户端 IP 可能记录不准确。

### Worker 与 Cron

- `service`、`worker`、`cron` 是三类进程角色；扩容 API 时只增加 `service` 实例，不要把它们理解为三个 API 实例。
- `service` 负责提供 HTTP API。
- `worker` 负责消费 Asynq 异步任务，当前内置了请求审计日志落库、请求日志导出、文件资源导出 3 类任务。多个 `worker` 可以连接同一 Redis、使用相同 `queue.namespace` 和队列配置并发消费；正常情况下同一任务只交给一个实例。Asynq 仍是至少一次语义：请求审计以 `request_id` 唯一键 upsert；导出记录通过数据库条件更新和稳定的 Asynq Task ID 领取执行权，同一 `task_run_id` 的异常并发只有一个初始执行者，只有同一 Task ID 的后续重试可接管自身释放或遗留的 `running` 状态；导出结果采用首个成功提交者获胜，未采用的临时文件会立即清理，清理失败则保留带过期时间的文件记录供定时任务重试。完成导出时会在记录中固化文件过期时间，导出列表展示导出总条数和实时已处理条数；到期后接口返回 `file_expired=true`、`download_ready=false`，前端显示“已过期”而不再提供下载。对象上传成功但后续数据库步骤失败时会补偿删除，删除失败的对象行标记为 `delete_failed`。对象存储与数据库仍不是同一事务，进程若恰好在对象上传后、补偿逻辑运行前被强制终止，仍需依赖对象存储生命周期或独立的对象对账能力；当前 `cron:clean-expired-files` 只处理已有 `upload_files.expires_at` 的记录，不能覆盖这个极窄窗口。WebSocket 实时提醒不作为交付凭据，最终状态以消息列表和导出记录为准。
- 使用导出能力时，`queue.queues` 必须包含可消费的 `export` 队列，并运行 `worker`；仅注册任务或启动 `service` 不会消费导出任务。
- 导出文件由顶部“我的导出”独立页面承接，不属于任务运维 Tab。`GET /admin/v1/system/export/list` 和 `detail?public_id=<UUID>` 只要求登录并在服务端限定当前用户；提交导出只返回随机 `public_id` 和状态，个人列表/详情、进度推送和结果通知不暴露任务执行 ID、队列或原始异常。任务运维中的执行记录继续供有权限的管理员排障。初版重置部署时，初始化迁移会为 `sys_export_records.public_id` 创建唯一索引；若目标库已执行旧版初始迁移，需先按部署方案重建数据库后再启用本版代码，并执行 `command api-route -y` 同步接口认证元数据。公开编号用于识别和反馈问题，不代替登录及文件所有者校验。
- 请求日志与文件资源导出提交前会按导出筛选条件检查是否有记录；空集返回业务码 `NoExportData`（`10004`，暂无需要导出的数据），不会创建任务或导出记录。该检查不冻结数据，若记录在提交后发生变化，最终导出仍以 worker 执行时为准。
- `queue.enable=false` 时，不需要启动 `worker`；请求审计日志由 service 进程内的有界缓冲批量异步落库。缓冲满且存储健康时，新条目使用固定超时的同步写入兜底；存储已故障时不会让每个业务请求持续阻塞在 DB 上，而是计数并限频告警溢出丢失。导出类异步能力会随之不可用。
- `cron` 负责定时任务调度；代码内置任务会同步到 `task_definitions`，后台运营配置中的 `status/cron_spec/allow_manual/allow_retry` 会作为运行时真相，并通过进程缓存和 `cron_task_states` 维护最近同步/执行状态。
- `cron:todo-reminder` 默认每 5 分钟扫描临期与最近逾期的待办。站内提醒先持久化到 `sys_messages`，再通过 WebSocket/Redis 通知在线客户端；可选短信先与业务事务一起写入 `sms_deliveries`，再由本次及后续扫描发送。`tdr:v1:{todo_id}:{due_at}:{type}` / `tds:v1:{todo_id}:{due_at}:{type}` 幂等键保证重复扫描不会重复创建同一提醒周期。明确失败最多尝试 3 次；供应商结果未知或进程在发送中断时转为 `unknown`，不自动重试以避免重复计费。`accepted` 只表示供应商 API 已接受，不是手机送达回执。修改截止时间会进入新的提醒周期，完成、取消、关闭短信偏好或手机号失效会停止未发送短信。
- 演示定时任务初始化默认关闭；`reset-system-data` 默认关闭，启用执行时会输出高风险告警。
- 不要把同一个周期任务同时注册到 `cron` 和 `worker` 体系里，否则会重复执行。
- 当前版本必须只运行一个 `cron` 实例。普通任务只有进程内 `WithoutOverlapping` 保护，多实例会对同一调度周期重复执行或重复入队。Redis 分布式锁仅覆盖标记为高风险的 cron 任务，只提供执行期间的跨实例互斥，不提供每个调度周期的唯一触发保证；锁固定 30 分钟 TTL、没有自动续期，超过 TTL 的长任务也可能再次并发。释放锁会校验持有者 token，避免旧实例删除新持有者的锁。需要多 `cron` 实例前，必须另行实现按调度周期去重和锁续期，不能仅依赖现有高风险任务锁。

注意：`reset-system-data` 当前调用的是 `system.ReinitializeSystemData()`，会清空数据库对象并按当前迁移重建系统数据。生产环境必须保持该任务在 `task_definitions.status=0`，只有在明确需要重建系统数据时才临时启用并及时关闭。

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
- `jwt.*`（`jwt.secret_key` 变更会被检测，但实际生效仍需重启）

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
4. 在 `AdminRouteTree()` 的合适安全策略分组中声明路由
5. 需要更新 API 路由表时执行 `go run main.go -c ./config.yaml command api-route`

### 参数校验约定

- 枚举字段（如 `status`、`is_auth`、`method`）建议使用 `oneof` 显式约束。
- ID 数组字段（如 `role_ids`、`menu_list`、`api_list`）建议统一使用 `dive,gt=0`，避免无效 ID（如 `0`）进入业务层。
- 新增/修改校验规则时，建议同时补充正反例单测，避免后续回归。

### 测试

```bash
go test ./...
go test ./tests/admin_test
```

`tests` 下的接口集成测试只使用项目根目录 `config.yaml`，并要求其中启用且可连接真实 MySQL；配置缺失、MySQL 不可用或迁移表缺失时直接失败，不会回退示例配置或跳过数据库流程。建议使用可重建的专用测试库，详见 `tests/README.md`。

文件上传约定：`GET /admin/v1/system/file/upload/options` 需要 `file:create` 权限，只返回当前驱动、大小和类型白名单、直传/分片能力等最小配置，不返回密钥、`bucket` 或 endpoint。上传初始化由服务端创建 session 并签发 `complete_token`；complete 只接受 token 对应的服务端元数据，multipart abort 只接受 `complete_token`。直传完成时，服务端把临时对象流复制到独立最终 key，并对实际内容计算 SHA-256 后校验；客户端 hash 只用于完整性声明，不再据此执行秒传复用。升级前已创建的旧版秒传复用会话会被拒绝，客户端需重新初始化上传。

存储驱动通过 `filestorage.DriverDefinition` 编译期注册。二开供应商实现基础 `Driver`，按需实现 `DirectUploadCapable`、`MultipartCapable`、`StatCapable`、`ImageThumbnailCapable`，再调用 `filestorage.RegisterDriver`；业务层不再按驱动名称分支。自定义配置使用 `Config.DecodeProviderConfig` / `SetProviderConfig` 保存在 `config.<driver>`，未知供应商配置会原位保留；顶层字符串密钥通过 `SensitiveFields` 与 `ProviderString` / `SetProviderString` 接入统一的加密、掩码、空值清除、查看权限和审计脱敏。配套前端新增一个供应商表单并调用 `registerStorageProviderForm`，无需修改 `StorageConfigPanel.vue`。驱动 ID 会持久化到文件记录，发布后必须保持稳定，历史对象迁移完成前不能移除旧驱动注册。

审计日志统一落在 `request_logs` 的 `change_diff` 字段；有字段级差异的写操作可通过 `has_change_diff=1` 在列表和导出中筛选。已认证用户的后台业务请求不参与随机采样；`queue.audit_sample_get_keep_every` 仅用于无法关联登录用户的低风险成功只读流量。`/ping`、`/health/readiness`和未匹配路由的 404 噪声既不写入请求日志，也不进入面向业务的每日请求统计；已命中业务路由的 404 仍保留。基础设施总流量应由 ingress/Prometheus 等监控链路采集，不用 MySQL 审计表代替。每日请求统计写入有短超时和独立故障冷却，不会熔断主审计；数据库故障时统计允许少量缺失，不能作为计费或合规口径。内存缓冲不主动驱逐已接收记录，短暂数据库不可用时保留当前批次并等待重试；字段在持久化边界按表宽归一化，明确的永久行错误会被隔离，避免单条坏记录阻塞整个批次。该链路仍为 best-effort：队列和数据库同时不可用且缓冲溢出、停机 drain 超时或进程崩溃时不承诺零丢失；溢出丢失会计数并限频记录，不会反复阻塞业务请求。非数据库触发器也不会覆盖人工 SQL。

工作台和日志采用“个人可见、全局授权”的边界：所有登录用户可访问个人工作台与个人请求日志，但服务端固定按当前 Token 的 UID 查询，不能传入其他用户 ID；全局指标、服务器监控和全部请求日志仍分别要求 `dashboard:*`、`requestLog:*` 权限。首页“今日操作”和“我的最近操作”只统计写操作、失败、高风险或有字段变更的记录，并排除 WebSocket 建连/临时票据及个人日志自查询等后台技术请求，避免连接重建和页面轮询淹没真实业务操作；完整审计记录仍保留在请求日志中。`GET /admin/v1/dashboard/statistics` 默认查询最近 7 个自然日，也接受包含首尾日期的 `start_date` / `end_date`（`YYYY-MM-DD`），最多 90 天，供工作台的 7 天、30 天和自定义范围共用同一真实统计口径。个人日志默认使用 `scope=regular` 隐藏“个人请求日志列表/详情”自身产生的读取记录，避免每次查询和翻页持续制造噪声；用户可选择 `scope=self_reads`，或明确填写操作名称，重新查询这些审计记录。个人列表和详情不返回 JWT、Header、查询参数、请求体、响应体或原始 JSON 差异，只对用户、角色、部门、菜单、字典、系统配置、个人资料、Product 和文件/目录等已登记操作按 route、action、field 三级白名单生成自然语言摘要，敏感值继续脱敏或隐藏；未知结构只显示“已省略”，不会回退展示原文。管理员详情保留结构化 JSON，便于排障和审计。服务器监控响应的 `app.audit` 同时提供进程内接收、采样、入队、降级、持久化、失败、永久丢弃、溢出丢弃、缓冲待处理和存储不可用指标；多实例部署需按实例采集聚合。

个人待办、系统通知和任务中心是三个不同概念：待办表示“某个用户还需要完成的事情”，登录用户始终只能读取自己的 `user_todos`；系统通知表示“需要告知用户的信息”，可以已读但不代表业务已处理；任务中心表示“后端异步或定时执行过程”，用于查看运行状态、重试和导出结果。任务中心内部继续保持定义、可抽样执行明细、按完成时间聚合的全量统计和 Cron 运行态四层边界；失败、超时、中断、取消明细始终保留，统计响应提供 `detail_time_basis/detail_start_time/detail_end_time`，前端据此按 `finished_at` 精确下钻。成功明细允许采样，因此成功总数不承诺与明细列表逐条对账。待办到期提醒只创建通知，不自动改变待办状态。手工待办可由本人创建、编辑、推进状态和删除；业务待办由业务模块通过 `assignee_id + source_type + source_key` 幂等写入，用户只能跳转到站内业务页面处理，完成或取消必须由业务模块调用 `CloseBusinessTodo`，避免待办状态与订单、审批等业务真相分叉。业务模块和通知的 `action_url` 只允许站内相对路径。列表按“逾期、紧急程度、截止时间”排序。参考 Product 模块已接入首个真实生命周期：禁用产品为创建者生成待启用事项，禁用态编辑刷新同一事项，启用后完成，删除时取消；产品再次由启用变为禁用会显式开启新一轮待办，普通消息重试不会重开历史终态。“我的待办”页面是登录可见的隐藏路由，不出现在侧栏，唯一产品入口为首页待办卡的“查看全部”。

### 短信渠道扩展边界

当前版本已实现可选的阿里云短信验证码登录和待办到期短信提醒，两者均默认关闭。拥有 `sms:config` 的管理员可在“系统配置 → 短信配置”维护供应商、模板、场景路由与验证码限制，`sms:update` 用于保存，`sms:test` 用于向指定号码发送一次受控测试。配置保存到受保护的 `sms.config`，AccessKey Secret 使用 `security.credential_encryption_key` 派生的短信专用密钥进行 AES-256-GCM 加密，读取接口只返回 `********`，请求日志和审计差异不会记录明文密钥、手机号或验证码。

启用条件如下：

- Redis 必须可用；验证码挑战、错误次数、手机号/IP 配额与一次性消费均以 Redis Lua 原子执行，Redis 故障时短信登录 fail-closed，不回退进程内存。
- 手机号日限额与全局日预算只在通过冷却检查、准备实际发送时原子占用；`sms:test` 另受操作者、目标号码和全局预算限制。
- 当前首版只接受中国大陆手机号（国家代码 `86`）；初始化表通过生成列唯一索引约束未删除账号的非空完整手机号。
- 初始化超级管理员不预置示例手机号；手机号必须在已有密码登录态下由本人（需当前密码）或具备用户管理权限的管理员显式配置，随后每次短信登录仍由 OTP 证明当前持有。首版没有开放匿名绑定或注册流程。
- 登录页先读取 `GET /admin/v1/login/options`，只有 `sms.config.enabled=true` 且配置校验通过才显示“验证码登录”标签；密码登录接口保持兼容。
- 发送接口同时校验图形验证码并对不存在、禁用或重复手机号返回一致结构，降低账号枚举风险；OTP 使用 HMAC 保存、限制错误次数并在成功时删除。
- 手机号已经成为登录凭据：个人修改手机号必须验证当前密码，管理员代改或个人改号都会撤销该用户已有会话。
- 待办短信还必须同时满足 `todo.reminder_enabled=true`、`todo.reminder_sms_enabled=true`、存在有效 `todo_due` 路由、用户已绑定手机号并在 `GET/POST /admin/v1/todo/reminder-preference` 中主动开启短信渠道；默认偏好为站内开启、短信关闭。
- 短信测试接口接收当前页面的完整草稿配置并仅在内存中校验、发送，不会先写入 `sms.config`；草稿中的 `********` 密钥会安全复用已保存密钥，空值表示明确清除，新密钥只用于本次调用。请求日志会递归脱敏供应商登记的敏感字段、手机号和验证码。阿里云 `endpoint` 是固定的完整官方 URL `https://dysmsapi.aliyuncs.com`，不是 `cn-hangzhou` 这类 Region ID。
- 阿里云登录验证码 `SendSms` 不自动重试；待办短信的明确拒绝最多重试 3 次，超时或中断属于发送状态未知并永久停止自动重试，避免盲目重发造成重复计费。生产启用前仍应在阿里云侧配置配额/告警，并限制 RAM 凭据权限。

短信扩展采用“场景 → 模板 → 供应商路由”，业务模块只提交 `scene`、接收人和模板变量：

```text
login_otp  ──> aliyun ──> SMS_OTP_TEMPLATE
todo_due   ──> aliyun ──> SMS_TODO_TEMPLATE
```

配置结构按供应商、模板和场景路由分离，不能把 `todo_due_template_code` 一类字段写死在阿里云配置中：

```json
{
  "providers": {
    "aliyun": { "enabled": true, "access_key_id": "...", "access_key_secret": "...", "endpoint": "https://dysmsapi.aliyuncs.com" }
  },
  "templates": {
    "aliyun": {
      "login_otp": { "sign_name": "示例签名", "template_code": "SMS_OTP_TEMPLATE", "variables": ["code"] },
      "todo_due": { "sign_name": "示例签名", "template_code": "SMS_TODO_TEMPLATE", "variables": ["title", "due_at"] }
    }
  },
  "routes": {
    "login_otp": { "provider": "aliyun", "template": "login_otp" },
    "todo_due": { "provider": "aliyun", "template": "todo_due" }
  }
}
```

- 统一 Provider 契约为 `Send(ctx, input)`，第一方只注册阿里云。新增供应商时实现 Provider 和 Factory，并在 `RegisterBuiltinProviders` 登记 `ProviderDefinition`；顶层密钥字段写入 `SensitiveFields` 后会统一获得掩码保留、空值清除、AES-256-GCM 入库、读取脱敏和审计清理，不再修改 service 分支。供应商代码不能判断 `login_otp`、`todo_due` 等业务场景。Go 使用编译期注册而非动态 `.so` 插件；配套前端在短信供应商表单注册表中增加一个显式 Vue 表单，同时必须原样保留尚未识别的 providers/templates/routes 数据。
- 场景由代码白名单声明，管理员只能为既有场景选择模板和供应商，不能通过配置创造任意业务场景；模板同时声明允许变量，登录模板必须且只能接收 `code`，待办模板必须且只能接收 `title`、`due_at`。初始化配置预留待办模板但不创建路由，必须配置真实模板后再显式启用。
- 待办短信使用持久化 `sms_deliveries` Outbox、业务幂等键、Redis 全局日预算和用户渠道偏好；不复用同步 OTP 发送流程。Outbox 不持久化明文手机号，发送前重新读取当前绑定手机号和偏好；已取消、已完成、改期、手机号失效或关闭偏好的待办会取消未发送记录。当前没有供应商送达回执和人工重放 `unknown` 的接口。
- `security.credential_encryption_key` 必须通过 `${CREDENTIAL_ENCRYPTION_KEY}` 等环境变量提供，并与 `jwt.secret_key` 分离；同一部署的 `service`、`worker`、`cron` 及所有副本必须使用相同值。主密钥按短信凭据、OTP/HMAC、配额 HMAC、OSS 凭据等用途独立派生，不能跨用途复用。它只在进程启动时读取，轮换需要先用旧密钥离线解密现有 `enc:v1` 凭据、用新密钥重加密，再协调重启全部实例；轮换期间混用新旧值会导致部分实例无法解密。后续可迁移到 RAM Role/KMS 与多 key version。
- 本版仍处于首次发布前，权限和 `sms.config` 已直接收敛到初始化迁移；首个正式 tag 后的短信表结构或默认权限演进必须新增独立成对迁移。

### 版本与升级策略

- `v0.10.0` 是本轮首个公开版本候选。当前仍处于首版初始化阶段，部署会重置系统数据，因此 `v0.10.0` 正式 tag 前允许直接收敛现有初始化迁移。
- 首个公开 `vX.Y.Z` tag 发布后，已发布迁移视为不可变；表结构、权限和默认数据演进必须新增成对的 `up/down` 迁移，不再修改历史迁移。
- `docs/openapi.yaml` 随源码纳入版本控制；路由、安全策略或 API 元数据变化后必须重新生成，CI 和 Release 会校验生成结果无漂移，发布归档同时携带该文件。
- 发布版本遵循 SemVer：`0.x` 阶段的破坏兼容调整升级 minor，兼容修复升级 patch；进入 `1.x` 后，破坏兼容调整升级 major，向后兼容功能升级 minor，兼容修复升级 patch。tag 触发 release workflow，并校验 tag、源码版本和 CHANGELOG 条目一致，执行单元测试、`go vet`、迁移检查和 MySQL 接口集成测试；产物包含二进制、示例配置、RBAC 模型、迁移、README、CHANGELOG、兼容说明、LICENSE 和校验和。
- 升级顺序为：备份数据库与配置 → `command migrate check` → `command migrate up` → 按变更说明执行 `api-route -y` / `task sync-definitions` → 协调重启 `service/worker/cron` → 验证 readiness 与关键权限流程。
- 回滚必须同时评估二进制与数据库：仅在新迁移明确支持安全 `down` 时回退数据库；涉及数据转换或破坏性结构变更时优先前向修复，不自动降级。每个正式版本应在 release notes 中列出兼容边界、升级命令、回滚限制和已知风险。

## 部署说明

### 构建

```bash
go build -o go-layout main.go
./go-layout service
```

如果没有显式传 `-c`，请在开发环境使用 `GO_ENV=development`，并确保当前工作目录存在 `config/config.yaml.example`（或已生成 `config.yaml`）；部署二进制时请保证可执行文件同级存在配置文件。

### 负载均衡与多实例

多个 API 实例可以通过负载均衡或 Kubernetes Service 暴露。负载均衡器需支持 WebSocket Upgrade，并将 `/health/readiness` 用作 readiness 检查、`/ping` 用作存活检查；依赖未就绪时 readiness 返回非 2xx，实例不应继续接收流量。

默认 `local` 存储只适合单实例。多实例部署请启用 OSS 或其他共享对象存储，并让 API、`worker`、`cron` 使用同一存储配置；所有进程还必须共享同一 MySQL/Redis 逻辑库、JWT 密钥和环境 namespace。

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
map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

server {
    listen 80;
    server_name api.example.com;

    location / {
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_pass http://127.0.0.1:9001;
    }
}
```

上例中的 `map` 放在 Nginx `http` 配置块中。如果由云负载均衡或 Ingress 转发，也要开启对应的 WebSocket Upgrade 和长连接超时。

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

## 免责声明

本项目按 **“现状”提供**，不附带任何明示或默示担保。项目可能存在缺陷、安全漏洞或与特定业务场景不匹配的实现；上线前请自行完成代码审查、安全加固、配置审查、权限验收和数据备份。因使用、依赖、部署、改造或运维本项目导致的问题，由使用者自行承担。
