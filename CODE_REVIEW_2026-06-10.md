# 全量代码审查报告

> 审查日期：2026-06-10
> 审查范围：全量代码库（已提交 + 未提交改动）
> 审查方法：5 组并行 Agent 分别覆盖安全、并发、错误处理/资源泄漏、数据一致性/性能、工程结构/部署

---

## 第一部分：代码库整体理解

### 技术栈

- **语言**：Go 1.26
- **Web 框架**：Gin 1.12
- **ORM**：GORM（MySQL 8.0 生产 / SQLite 测试）
- **缓存/消息**：Redis 7（go-redis v9）
- **队列**：Asynq 0.26（基于 Redis）
- **权限**：Casbin v3（RBAC + keyMatch3）
- **认证**：JWT（golang-jwt v5，48h TTL）
- **配置**：Viper + fsnotify 热更新
- **文件存储**：本地 + 阿里云 OSS（接口抽象）
- **构建**：Cobra CLI + build.sh 交叉编译

### 启动入口

- `main.go` -> `cmd.Execute()` -> Cobra 根命令
- 四个子命令：`service`（HTTP API）、`cron`（定时任务）、`worker`（异步消费）、`command`（一次性脚本）
- 统一 bootstrap：`bootstrapx.WrapCommand` -> Config -> Timezone -> Logger -> Data -> Validator -> Queue

### 核心目录职责

| 目录 | 职责 |
|------|------|
| `cmd/` | 进程入口和启动编排 |
| `config/` | 配置加载、热更新、子结构体定义 |
| `data/` | MySQL/Redis 初始化、迁移、运行时健康检查 |
| `internal/routers/` | 声明式路由树 + ControllerDeps DI |
| `internal/controller/` | HTTP 薄控制器层 |
| `internal/middleware/` | 认证、权限、日志、审计、CORS |
| `internal/service/` | 核心业务逻辑（14 个子包） |
| `internal/model/` | GORM 模型 + 通用 CRUD/分页/树形 |
| `internal/resources/` | 响应资源转换（Transformer 模式） |
| `internal/access/casbin/` | Casbin 权限引擎 |
| `internal/cron/` | 内置定时任务定义 + handler 注册 |
| `internal/jobs/` | Asynq 异步任务 |
| `internal/queue/` | 队列抽象层 |
| `internal/filestorage/` | 文件存储抽象 |
| `internal/pkg/` | 内部公共包（errors/i18n/logger/response） |
| `pkg/` | 可导出公共包（crypto/captcha/upload） |

### 主要业务链路

```
HTTP 请求 -> CORS -> RequestCost -> Locale -> ParseToken(JWT) -> [DB Ready Guard]
  -> AdminAuthHandler(Casbin) -> Controller -> Service -> Model/DB
                                                             -> Resources(Transformer) -> Response
```

### 关键模块位置

| 关注点 | 位置 |
|--------|------|
| 鉴权/权限 | `internal/middleware/admin_auth.go`, `internal/access/casbin/` |
| JWT 生成/验证 | `internal/service/auth/login*.go`, `internal/pkg/utils/token/jwt.go` |
| 文件上传 | `internal/service/file_resource*.go`, `internal/filestorage/` |
| 任务中心 | `internal/service/taskcenter/`, `cmd/cron/tasks.go` |
| 配置热更新 | `config/config_load.go`, `internal/runtime/config_reload.go` |
| 实时通知 | `internal/service/notification/realtime.go` |
| 数据权限 | `internal/service/access/data_scope_filter.go` |
| 审计日志 | `internal/middleware/audit_queue.go`, `internal/service/audit/` |

---

## 第二部分：高风险问题

### HIGH-1：`is_super_admin` 字段缺操作者身份校验，可越权提权

- **严重级别**：HIGH
- **文件**：`internal/service/admin/admin_user_mutation.go:77-83`
- **问题说明**：新增的 `is_super_admin` 字段直接进入 Create/Update 流程，校验逻辑 `actorCanManageSuperAdmin()` 仅在 `params.IsSuperAdmin != nil && (params.Id == 0 || oldIsSuperAdmin != *params.IsSuperAdmin)` 时触发。但 `actorCanManageSuperAdmin` 通过 `loadAdminUserForSuperAdminCheck` 全局函数变量加载操作者身份，该变量可在测试中被替换但生产中无保护。
- **影响范围**：所有管理员创建/编辑接口
- **触发条件**：拥有「编辑管理员」权限的非超管调用 `POST /admin/admin-user/create` 或 `POST /admin/admin-user/update`，传入 `is_super_admin=1`。`actorCanManageSuperAdmin` 从 DB 加载操作者记录检查 `IsSuperAdmin`，但加载函数是可替换的全局变量——如果测试 hook 未正确还原，生产代码可能使用错误的 loader。
- **修复建议**：将 `loadAdminUserForSuperAdminCheck` 改为 service 实例方法而非全局变量；或在 `saveAdminUserMutation` 开头直接通过 `model.NewAdminUsers().GetById(actorID)` 加载，不依赖全局函数。
- **是否立即修复**：建议上线前修复

### HIGH-2：登录失败计数器错误被吞，可绕过账户锁定

- **严重级别**：HIGH
- **文件**：`internal/service/auth/login_security.go:42-44`
- **问题说明**：`incrementLoginFailState` 失败时仅 Warn 日志，不阻断登录流程。如果该函数因 DB 错误持续失败，暴力破解的失败计数永远不会增加，账户锁定机制完全失效。
- **影响范围**：登录安全机制
- **触发条件**：MySQL 连接异常或 `login_security_states` 表不可用时，连续暴力破解不会触发锁定。
- **修复建议**：`incrementLoginFailState` 失败时应阻断登录（返回错误或 fail-closed），至少在连续失败时拒绝登录。
- **是否立即修复**：建议立即修复

### HIGH-3：密码/状态变更后 Token 撤销失败，用户保留有效会话

- **严重级别**：HIGH
- **文件**：`internal/service/admin/admin_user_mutation.go:166-171`
- **问题说明**：`revokeUserTokensAfterCommit` 在事务提交后执行，如果 Redis 不可用，Token 撤销失败仅 Warn 日志，用户保留有效 JWT（最长 48 小时）。管理员修改了某用户密码，但该用户旧 Token 仍可使用。
- **影响范围**：账户安全、会话管理
- **触发条件**：Redis 不可用时管理员修改用户密码/禁用账户/变更超管权限。
- **修复建议**：Token 撤销失败时应将失败记录到 DB 撤销表（已有 `token_revocations` 表），确保 `parseToken` 在下次请求时通过 DB 校验拒绝已撤销的 Token。当前 `login_revoke.go` 已有 DB fallback 路径，但 `revokeUserTokensAfterCommit` 未使用它。
- **是否立即修复**：建议立即修复

### HIGH-4：菜单唯一性校验在事务外，存在竞态导致重复数据或 500 错误

- **严重级别**：HIGH
- **文件**：`internal/service/menu/menu_edit.go:136-146`, `internal/model/menu.go:124-141`
- **问题说明**：`validateUniqueFields` 使用普通 `SELECT 1 LIMIT 1`（无 `FOR UPDATE`），在事务外执行。两个并发创建相同 `code` 的请求都可通过校验，其中一个在 INSERT 时触发 MySQL 唯一键冲突，返回原始 MySQL 错误（500）而非业务错误码。
- **对比**：`admin_user_mutation.go` 正确使用了 `ExistsWithLockExcludeId`（`SELECT ... FOR UPDATE`）。
- **影响范围**：菜单管理接口
- **触发条件**：并发创建相同 code 的菜单。
- **修复建议**：将唯一性校验移入事务内，使用 `SELECT ... FOR UPDATE`，或在事务内 catch 唯一键冲突并转换为业务错误。
- **是否立即修复**：建议上线前修复

### HIGH-5：`MenuCollectionResources.SetCustomFields` 缺少 nil 检查，可能 panic

- **严重级别**：HIGH
- **文件**：`internal/resources/menu.go:161`
- **问题说明**：所有其他 `SetCustomFields` 实现都有 `if data == nil { return }` 守卫，唯独 `MenuCollectionResources` 没有。如果 `[]*model.Menu` 中包含 nil 元素，调用 `data.MenuTypeMap()` 会 panic。
- **影响范围**：菜单列表/树形接口
- **触发条件**：DB 查询返回的菜单切片包含 nil 元素（GORM 在某些 join 场景下可能产生）。
- **修复建议**：添加 `if data == nil { return }` 守卫。
- **是否立即修复**：建议立即修复

### HIGH-6：`config.yaml` 包含硬编码 JWT 密钥并提交到 Git

- **严重级别**：HIGH
- **文件**：`config.yaml:14`（注意：是 `config.yaml` 而非 `config.yaml.example`）
- **问题说明**：实际的 `config.yaml` 包含 JWT 签名密钥和 MySQL 密码，且已提交到 Git 仓库。虽然 `.gitignore` 有 `*.yaml` 规则，但如果文件在规则添加前已被跟踪，Git 会继续跟踪。还包含注释掉的生产服务器 IP 和密码。
- **影响范围**：任何可以访问仓库的人都能伪造任意用户的 JWT Token。
- **触发条件**：仓库公开或被泄露。
- **修复建议**：`git rm --cached config.yaml` 从 Git 中移除；用 `${JWT_SECRET}` 环境变量替换硬编码值；`git filter-branch` 或 BFG 清理历史中的敏感信息。
- **是否立即修复**：**必须立即修复**

---

### MEDIUM-1：超管状态仅存于 JWT，撤销后最长 48 小时仍有效

- **严重级别**：MEDIUM
- **文件**：`internal/middleware/admin_auth.go:85-87`, `internal/service/auth/principal.go:15`
- **问题说明**：`isSuperAdmin` 判断直接从 JWT claims 读取，不查 DB。撤销某用户的 `is_super_admin` 权限后，其现有 Token 在 48 小时内仍携带 `IsSuperAdmin=1`，绕过所有 Casbin 权限检查。
- **修复建议**：对 `is_super_admin` 状态做 DB 实时校验（可缓存），或缩短 Token TTL。
- **是否立即修复**：上线前修复

### MEDIUM-2：未注册路由隐式放行，可能被利用访问未配置权限的接口

- **严重级别**：MEDIUM
- **文件**：`internal/middleware/admin_auth.go:107-124`
- **问题说明**：当 Casbin `Enforce` 返回 `false`（无权限）时，代码检查 `CheckoutRouteIsAuth`。如果路由不在 API 路由缓存中，`CheckoutRouteIsAuth` 返回 `false`，请求被放行。即：任何未在 `api` 表注册的路由，对所有已登录用户（包括非超管）都是可访问的。
- **修复建议**：默认策略应改为"未注册路由拒绝访问"，仅显式标记为公开的路由才放行。
- **是否立即修复**：上线前修复

### MEDIUM-3：`sys_config` Redis Pubsub 订阅者断线后不重连

- **严重级别**：MEDIUM
- **文件**：`internal/service/sys_config/cache_sync.go:86-119`
- **问题说明**：`runSysConfigCacheSyncSubscriber` 在 `ReceiveMessage` 返回错误后退出，`started` 标志重置为 `false`。但没有重连循环——只有下次本地配置变更时才会重新订阅。对比 `cmd/cron/tasks.go` 和 `notification/realtime.go` 都有重连逻辑。
- **影响范围**：多实例部署时，Redis 抖动后该实例永久收不到其他实例的配置变更通知。
- **修复建议**：添加外层重连循环（指数退避），与 `subscribeTaskDefinitionChanges` 模式一致。
- **是否立即修复**：上线前修复

### MEDIUM-4：`auditFallbackBuffer.Stop()` 并发调用会 panic

- **严重级别**：MEDIUM
- **文件**：`internal/middleware/audit_fallback_buffer.go:254-278`
- **问题说明**：`Stop()` 在释放 mutex 后通过 `select` + `default` 检查 `stopCh` 是否已关闭，然后调用 `close(stopCh)`。两个并发 `Stop()` 调用都可能通过 `default` 分支，导致 double-close panic。
- **修复建议**：用 `sync.Once` 包裹 close 操作，或在 mutex 内执行 close。
- **是否立即修复**：上线前修复

### MEDIUM-5：`setActiveConfig` 写 `Config` 全局指针无同步

- **严重级别**：MEDIUM
- **文件**：`config/config_clone.go:11-14`
- **问题说明**：`setActiveConfig` 先写 `Config = cfg`（无锁），再 `configValue.Store(cfg)`（atomic）。`GetConfig()` 的 fallback 路径直接返回 `Config` 指针。fsnotify goroutine 写 `Config` 与其他 goroutine 读 `Config` 存在 data race。
- **修复建议**：删除 `Config = cfg`，仅使用 `configValue.Store/Load`。
- **是否立即修复**：上线前修复

### MEDIUM-6：凭证上传 MIME 类型仅依赖客户端声明，缺服务端校验

- **严重级别**：MEDIUM
- **文件**：`internal/service/file_resource_folder_upload.go:370-489`
- **问题说明**：`UploadCredential`（直传凭证）流程中，MIME 类型来自客户端参数，服务端未对实际文件内容做 magic-byte 校验。攻击者可声称 `image/jpeg` 上传 HTML 文件，导致存储型 XSS。
- **修复建议**：在 `CompleteDirectUpload` 回调中对已上传文件做服务端 MIME 探测（如读前 512 字节 `http.DetectContentType`）。
- **是否立即修复**：上线前修复

### MEDIUM-7：登录失败计数 TOCTOU 竞态

- **严重级别**：MEDIUM
- **文件**：`internal/service/auth/login_security.go:83-125`
- **问题说明**：`incrementLoginFailState` 先 `FindByUsername`（普通 SELECT），再 `Save`。并发暴力破解请求可同时读到相同的 `fail_count`，导致计数漂移。虽然锁定行为本身正确（`applyLoginFailState` 对比阈值），但 `fail_count` 值可能不准确。
- **修复建议**：使用 `UPDATE ... SET fail_count = fail_count + 1` 原子递增，或在事务内加 `SELECT ... FOR UPDATE`。
- **是否立即修复**：后续优化

### MEDIUM-8：`pkg/utils` 导入 `internal/` 包，破坏 `pkg/` 可复用性

- **严重级别**：MEDIUM
- **文件**：`pkg/utils/`（多文件）
- **问题说明**：`pkg/utils` 导入了 `internal/global` 和 `internal/model`，违反了 `pkg/` 不应依赖 `internal/` 的原则。这使得 `pkg/utils` 无法在外部项目中复用。
- **修复建议**：将依赖 `internal/` 的代码移入 `internal/pkg/`，`pkg/` 保持纯外部可复用。
- **是否立即修复**：后续优化

### MEDIUM-9：`handleMutationError` 丢失原始错误信息

- **严重级别**：MEDIUM
- **文件**：`internal/service/admin/admin_user_mutation.go:45-56`
- **问题说明**：非 `BusinessError` 的错误（网络超时、OOM、context canceled）被静默转换为通用业务错误码，原始错误信息完全丢失，生产环境难以定位根因。
- **修复建议**：用 `log.Logger.Error` 记录原始错误后再转换，或 `Wrap` 原始错误。
- **是否立即修复**：上线前修复

### MEDIUM-10：通知 Dispatcher 启动后无对应 Stop 调用

- **严重级别**：MEDIUM
- **文件**：`cmd/service/service.go:62-63` vs `87-116`
- **问题说明**：`notification.EnsureRealtimeDispatcherStarted()` 在启动时调用，但 `waitForShutdown` 中只有 `dashboard.StopMonitorPush`，没有 `notification.StopRealtimeDispatcher`。WebSocket 连接和相关 goroutine 不会被优雅关闭。
- **修复建议**：在 shutdown 流程中添加 `notification.StopRealtimeDispatcher`。
- **是否立即修复**：上线前修复

### MEDIUM-11：任务运行状态 Finish 无乐观锁，取消操作可被覆盖

- **严重级别**：MEDIUM
- **文件**：`internal/service/taskcenter/recorder.go:274-330`
- **问题说明**：`Finish` 方法更新 task run 状态时无 `WHERE status = 'running'` 守卫。用户取消任务（设为 canceled）的同时 worker 完成执行（设为 success），后者会覆盖取消状态。
- **修复建议**：`Finish` 的 UPDATE 添加 `WHERE status = ?` 条件，检查 affected rows。
- **是否立即修复**：后续优化

### MEDIUM-12：`FileResourceResources` 和 `FileReferenceResources` 的 `SetCustomFields` 缺 nil 守卫

- **严重级别**：MEDIUM
- **文件**：`internal/resources/file_resource.go:74`, `internal/resources/file_resource.go:256`
- **问题说明**：同 HIGH-5 模式，缺少 `if data == nil { return }` 守卫。
- **修复建议**：添加 nil 检查。
- **是否立即修复**：建议立即修复

---

### LOW-1：`redisDb`/`mysqlDB` 回退变量无锁读

- **严重级别**：LOW
- **文件**：`data/redis.go:16-17,178-183`, `data/mysql.go`（同样模式）
- **问题说明**：`currentRedis()` 在 atomic.Value 未初始化时 fallback 到 `redisDb`，该变量在 mutex 下写但无锁读。race detector 会告警。
- **修复建议**：删除 fallback 变量，仅使用 `atomic.Value`。

### LOW-2：Cron `taskCanRun` 释放 RLock 后读取字段（TOCTOU）

- **严重级别**：LOW
- **文件**：`cmd/cron/tasks.go:242-265`
- **问题说明**：`taskCanRun` 在 `r.mu.RUnlock()` 后读取 `current.definition` 等字段。由于 `runtimeCronTask` 是值类型（copy 语义），实际风险很小，但违反 Go race detector 规则。
- **修复建议**：在 RLock 内拷贝所有需要的字段到局部变量。

### LOW-3：Admin 密码最短长度仅 6 字符

- **严重级别**：LOW
- **文件**：`internal/validator/form/admin_user.go:6`
- **问题说明**：`min=6` 对管理后台密码偏弱，建议至少 8 字符。

### LOW-4：JWT Token TTL 48 小时偏长

- **严重级别**：LOW
- **文件**：`config/autoload/jwt.go`
- **问题说明**：`TTL: 172800`（48 小时）。Token 被盗后有 48 小时窗口。结合 HIGH-3（撤销失败），风险放大。
- **修复建议**：生产环境缩短至 2-4 小时，配合 refresh token。

### LOW-5：`cleanupStoredUpload` 在 Close 错误检查前执行

- **严重级别**：LOW
- **文件**：`internal/service/common.go:120-127`
- **问题说明**：`file.Close()` 后先删除本地临时文件，再检查 `Close` 错误。如果 `Close` 失败（数据未刷盘），本地副本已删除，远端存储数据可能不完整。
- **修复建议**：先检查 `closeErr`，再执行 cleanup。

### LOW-6：LIKE 通配符注入

- **严重级别**：LOW
- **文件**：`internal/pkg/query_builder/query_builder.go:26-29`
- **问题说明**：`AddLike` 不转义用户输入中的 `%` 和 `_`，可导致匹配范围超出预期。

### LOW-7：`children_num` 计数器非原子更新

- **严重级别**：LOW
- **文件**：`internal/model/base_tree.go:15-41`
- **问题说明**：`COUNT(*)` + `UPDATE` 模式在无事务时存在竞态。

### LOW-8：Login Log Worker 无优雅关闭

- **严重级别**：LOW
- **文件**：`internal/service/auth/login.go:34-51`
- **问题说明**：`loginLogQueue` channel 永不关闭，goroutine 永不退出，进程退出时缓冲区内的登录日志丢失。

### LOW-9：Cron `subscribeTaskDefinitionChanges` 不过滤自身消息

- **严重级别**：LOW
- **文件**：`cmd/cron/tasks.go:332-352`
- **问题说明**：发布变更通知的实例自身也会收到并触发 reconcile，产生冗余 DB 读取。

### LOW-10：`service/system/init.go` 导入 `routers`（层级违规）

- **严重级别**：LOW
- **文件**：`internal/service/system/init.go:14`
- **问题说明**：service 层向上依赖 router 层，仅用于 `init api-route` 命令。架构边界不清晰。

---

## 第三部分：建议优先级

### 必须立即修复

| # | 问题 | 文件 |
|---|------|------|
| HIGH-6 | `config.yaml` 含硬编码 JWT 密钥并提交 Git | `config.yaml:14` |
| HIGH-5 | `MenuCollectionResources.SetCustomFields` nil panic | `internal/resources/menu.go:161` |
| MEDIUM-12 | `FileResource`/`FileReference` SetCustomFields 缺 nil 守卫 | `internal/resources/file_resource.go:74,256` |

### 上线前建议修复

| # | 问题 | 文件 |
|---|------|------|
| HIGH-2 | 登录失败计数错误被吞，可绕过锁定 | `internal/service/auth/login_security.go:42` |
| HIGH-3 | 密码变更后 Token 撤销失败保留有效会话 | `internal/service/admin/admin_user_mutation.go:166` |
| HIGH-4 | 菜单唯一性校验在事务外竞态 | `internal/service/menu/menu_edit.go:136` |
| HIGH-1 | `is_super_admin` 操作者校验依赖可替换全局变量 | `internal/service/admin/admin_user_mutation.go:77` |
| MEDIUM-1 | 超管状态仅存 JWT，撤销后 48h 内仍有效 | `internal/middleware/admin_auth.go:85` |
| MEDIUM-2 | 未注册路由隐式放行 | `internal/middleware/admin_auth.go:119` |
| MEDIUM-3 | sys_config pubsub 断线不重连 | `internal/service/sys_config/cache_sync.go:86` |
| MEDIUM-4 | auditFallbackBuffer.Stop() 并发 panic | `internal/middleware/audit_fallback_buffer.go:254` |
| MEDIUM-5 | setActiveConfig 写全局指针无同步 | `config/config_clone.go:11` |
| MEDIUM-6 | 凭证上传 MIME 仅依赖客户端声明 | `internal/service/file_resource_folder_upload.go:370` |
| MEDIUM-9 | handleMutationError 丢失原始错误 | `internal/service/admin/admin_user_mutation.go:45` |
| MEDIUM-10 | 通知 Dispatcher 无 Stop 调用 | `cmd/service/service.go:62` |

### 后续优化

| # | 问题 |
|---|------|
| MEDIUM-7 | 登录失败计数 TOCTOU 竞态 |
| MEDIUM-8 | `pkg/utils` 导入 `internal/` |
| MEDIUM-11 | 任务 Finish 无乐观锁 |
| LOW-1 | `redisDb`/`mysqlDB` 回退变量无锁读 |
| LOW-2 | Cron taskCanRun TOCTOU |
| LOW-3 | 密码最短长度 6 字符 |
| LOW-4 | JWT TTL 48 小时 |
| LOW-5 | cleanupStoredUpload 时序 |
| LOW-6 | LIKE 通配符注入 |
| LOW-7 | children_num 非原子更新 |
| LOW-8 | Login Log Worker 无关闭 |
| LOW-9 | Cron 订阅不过滤自身消息 |
| LOW-10 | service/system 导入 routers |

### 测试覆盖缺口（建议补充）

| 包 | 严重性 | 说明 |
|---|--------|------|
| `internal/service/product/` | HIGH | 零测试，含数据权限过滤的完整 CRUD |
| `pkg/utils/crypto/` | MEDIUM | 安全敏感代码无测试 |
| `pkg/convert/` | LOW | 无测试 |
| `pkg/utils/captcha/` | LOW | 无测试 |

---

## 第四部分：不确定项

| # | 问题 | 需确认内容 |
|---|------|-----------|
| 1 | `config.yaml` 是否真的被 Git 跟踪 | `.gitignore` 有 `*.yaml` 规则，但文件可能在规则添加前已被 `git add`。需确认 `git ls-files config.yaml` 是否有输出。安全 Agent 报告称已确认被跟踪。 |
| 2 | `AdminCustomClaims` 中的 PII 是否会直接序列化到 JWT | `jwt_payload.go` 使用精简的 `adminAccessTokenClaims`，但需确认是否有其他代码路径直接序列化完整 `AdminCustomClaims`。 |
| 3 | `CheckoutRouteIsAuth` 的路由缓存是否覆盖所有已注册路由 | 需确认 `api` 表的初始化是否完整覆盖 `admin_router.go` 中声明的所有路由。如果有遗漏，MEDIUM-2 的风险更严重。 |
| 4 | 生产环境 `config.yaml` 是否使用环境变量替换敏感值 | `resolveEnvVars` 支持 `${ENV_VAR}` 语法，但需确认生产部署是否实际使用了该机制。 |
| 5 | `realtimePingPeriod`（30s）导致 keepalive goroutine 最长泄漏 30s | 实际影响取决于并发 WebSocket 连接数量。如果是长连接场景，累积的 idle goroutine 可能消耗内存。 |
| 6 | Casbin 策略跨实例同步机制 | `ReloadPolicyCache` 从 DB 重新加载，但多实例间无 pubsub 通知。需确认是否有其他机制确保策略变更在所有实例生效。 |

---

## 正面观察

审查过程中也发现了一些良好的工程实践，值得记录：

1. **Admin 用户唯一性校验使用 `SELECT ... FOR UPDATE`**（`admin_users.go:137-158`），正确防止并发创建重复用户。
2. **事务包装**（`RunInTransaction`, `RunAfterCommitWithCode`）在 role/dept/menu/admin_user 变更中一致使用。
3. **Sys config 缓存**使用 Redis pub/sub 做跨实例失效，带 source-ID 过滤避免回声。
4. **API 路由缓存**使用 temp-key + RENAME 原子交换，防止部分读。
5. **权限同步**正确包含在事务内（通过 `coordinator.SyncUsers`）。
6. **文件路径遍历防护**在 local storage driver 和 upload helper 中正确实现。
7. **SQL 注入防护**：所有 raw SQL 使用参数化查询，QueryBuilder 的 LIKE 也使用 `?` 占位。
8. **CORS 配置**默认严格，启动时校验不与 credentials 同时使用通配符。
9. **优雅关停**：service 命令的关停顺序正确（HTTP 停止 -> 审计 drain -> 监控停止 -> 数据源关闭）。
10. **`is_super_admin` 变更会触发 token 撤销**（`admin_user_mutation.go:145-151`），设计意图正确。
