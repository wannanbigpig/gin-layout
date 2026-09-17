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
| `jwt.ttl` / `jwt.refresh_token_ttl` / `jwt.refresh_cookie_*` | Access Token 生命周期、Refresh Token Cookie 生命周期与浏览器策略；`jwt.refresh_ttl` 已废弃 |
| `mysql` | 数据库开关与连接信息 |
| `redis` | 缓存、黑名单和分布式锁配置 |
| `queue.use_default_redis` | `true` 复用 `redis.*`；`false` 时改用 `queue.redis.*` 独立连接 |
| `queue` | Asynq 异步任务开关、队列命名空间、并发度、优先级和审计日志重试策略 |
| `storage.active_driver` / `storage.config` | 运行时文件存储驱动与敏感配置，保存在 `sys_config`，由 `/admin/v1/system/storage/*` 管理 |
| `logger` | 日志输出、切割和保留策略 |

如果通过 Nginx、Ingress 或负载均衡转发请求，需要同步配置 `app.trusted_proxies`，否则客户端 IP 可能记录不准确。

### Worker 与 Cron

- `service`、`worker`、`cron` 是三类进程角色；扩容 API 时只增加 `service` 实例，不要把它们理解为三个 API 实例。
- `service` 负责提供 HTTP API。
- `worker` 负责消费 Asynq 异步任务，当前内置了请求审计日志落库、请求日志导出、文件资源导出 3 类任务。多个 `worker` 可以连接同一 Redis、使用相同 `queue.namespace` 和队列配置并发消费；正常情况下同一任务只交给一个实例。Asynq 仍是至少一次语义：请求审计以 `request_id` 唯一键 upsert；已完成导出的顺序重投会复用既有记录；导出通知以任务执行 ID 和状态组成去重键。导出任务使用稳定 Task ID 抑制正常重复入队，但“检查已完成记录”和上传对象不是一个原子操作，不能把异常情况下同一 `task_run_id` 的并发执行视为 exactly-once。对象存储与数据库也不是同一事务，进程若在对象上传后、导出记录提交前退出，仍可能遗留不可见的孤立对象，应由文件清理任务处理；WebSocket 实时提醒不作为交付凭据，最终状态以消息列表和导出记录为准。
- 使用导出能力时，`queue.queues` 必须包含可消费的 `export` 队列，并运行 `worker`；仅注册任务或启动 `service` 不会消费导出任务。
- `queue.enable=false` 时，不需要启动 `worker`；请求审计日志由 service 进程内的有界缓冲批量异步落库。缓冲满且存储健康时，新条目使用固定超时的同步写入兜底；存储已故障时不会让每个业务请求持续阻塞在 DB 上，而是计数并限频告警溢出丢失。导出类异步能力会随之不可用。
- `cron` 负责定时任务调度；代码内置任务会同步到 `task_definitions`，后台运营配置中的 `status/cron_spec/allow_manual/allow_retry` 会作为运行时真相，并通过进程缓存和 `cron_task_states` 维护最近同步/执行状态。
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

审计日志统一落在 `request_logs` 的 `change_diff` 字段；有字段级差异的写操作可通过 `has_change_diff=1` 在列表和导出中筛选。已认证用户的后台业务请求不参与随机采样；`queue.audit_sample_get_keep_every` 仅用于无法关联登录用户的低风险成功只读流量。`/ping`、`/health/readiness`和未匹配路由的 404 噪声既不写入请求日志，也不进入面向业务的每日请求统计；已命中业务路由的 404 仍保留。基础设施总流量应由 ingress/Prometheus 等监控链路采集，不用 MySQL 审计表代替。每日请求统计写入有短超时和独立故障冷却，不会熔断主审计；数据库故障时统计允许少量缺失，不能作为计费或合规口径。内存缓冲不主动驱逐已接收记录，短暂数据库不可用时保留当前批次并等待重试；字段在持久化边界按表宽归一化，明确的永久行错误会被隔离，避免单条坏记录阻塞整个批次。该链路仍为 best-effort：队列和数据库同时不可用且缓冲溢出、停机 drain 超时或进程崩溃时不承诺零丢失；溢出丢失会计数并限频记录，不会反复阻塞业务请求。非数据库触发器也不会覆盖人工 SQL。

工作台和日志采用“个人可见、全局授权”的边界：所有登录用户可访问个人工作台与个人请求日志，但服务端固定按当前 Token 的 UID 查询，不能传入其他用户 ID；全局指标、服务器监控和全部请求日志仍分别要求 `dashboard:*`、`requestLog:*` 权限。个人日志默认使用 `scope=regular` 隐藏“个人请求日志列表/详情”自身产生的读取记录，避免每次查询和翻页持续制造噪声；用户可选择 `scope=self_reads`，或明确填写操作名称，重新查询这些审计记录。个人列表和详情不返回 JWT、Header、查询参数、请求体、响应体或原始 JSON 差异，只对用户、角色、部门、菜单、字典、系统配置、个人资料、Product 和文件/目录等已登记操作按 route、action、field 三级白名单生成自然语言摘要，敏感值继续脱敏或隐藏；未知结构只显示“已省略”，不会回退展示原文。管理员详情保留结构化 JSON，便于排障和审计。服务器监控响应的 `app.audit` 同时提供进程内接收、采样、入队、降级、持久化、失败、永久丢弃、溢出丢弃、缓冲待处理和存储不可用指标；多实例部署需按实例采集聚合。

个人待办、系统通知和任务中心是三个不同概念：待办表示“某个用户还需要完成的事情”，登录用户始终只能读取自己的 `user_todos`；系统通知表示“需要告知用户的信息”，可以已读但不代表业务已处理；任务中心表示“后端异步或定时执行过程”，用于查看运行状态、重试和导出结果。手工待办可由本人创建、编辑、推进状态和删除；业务待办由业务模块通过 `assignee_id + source_type + source_key` 幂等写入，用户只能跳转到站内业务页面处理，完成或取消必须由业务模块调用 `CloseBusinessTodo`，避免待办状态与订单、审批等业务真相分叉。业务模块的 `action_url` 只允许站内相对路径。列表按“逾期、紧急程度、截止时间”排序。参考 Product 模块已接入首个真实生命周期：禁用产品为创建者生成待启用事项，禁用态编辑刷新同一事项，启用后完成，删除时取消；产品再次由启用变为禁用会显式开启新一轮待办，普通消息重试不会重开历史终态。

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
