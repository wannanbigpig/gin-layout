# AGENTS.md

本文件是 `gin-layout` 的根目录 AI 协作规则。AI 或贡献者修改代码前，应先阅读本文件，并结合当前任务继续读取相关源码、测试和文档。

## 项目概览

- 这是一个基于 `Gin` 的后台管理系统脚手架，模块名为 `github.com/wannanbigpig/gin-layout`。
- 入口是 `main.go`，实际命令由 `cmd/` 下的 `Cobra` 命令注册。
- 后台运行模型分为三类进程：
  - `service`：HTTP API 服务；
  - `worker`：Asynq 异步任务消费进程；
  - `cron`：定时任务调度进程。
- 主要能力包括 JWT 认证、RBAC 权限、声明式路由、请求/登录日志、文件上传、系统配置/字典、任务中心、配置热更新和健康检查。

## 开始前必读

- 先读当前任务相关的最近规则与经验：
  - `.aitasks/lessons.md`：如存在，必须优先读取，避免重复踩坑。
  - `.aitasks/todo.md`：如存在未完成任务，确认是否与当前修改冲突。
- 再按任务范围读取项目文档：
  - `README.md`：项目定位、快速启动、目录结构、开发指南。
  - `docs/COMMANDS_AND_TASKS.md`：命令、worker、cron、任务中心约定。
  - `docs/MIGRATE_COMMANDS.md`：数据库迁移命令和命名规范。
  - `docs/SYSTEM_CONFIG_AND_DICT_GUIDELINES.md`：系统配置和字典边界。
  - `tests/README.md`：接口测试目录分工和维护规则。
- 查找隐藏目录下的规则、经验或任务文件时使用 `rg --hidden --files`，必要时加 `--no-ignore`；默认 `rg --files` 会漏掉点目录内容。

## 常用命令

```bash
go mod download
go test ./...
go vet ./...
go mod verify
go run main.go -h
go run main.go -c ./config.yaml command migrate check
go run main.go -c ./config.yaml command migrate up
GO_ENV=development go run main.go -c ./config.yaml service
GO_ENV=development go run main.go -c ./config.yaml worker
GO_ENV=development go run main.go -c ./config.yaml cron
```

- 本地依赖可用 `make dev-up` 启动 MySQL、Redis、MinIO，`make dev-down` 停止。
- 热更新服务使用 `make dev-service`，worker 使用 `make dev-worker`，两者都依赖根目录 `config.yaml`。
- 通过 `go run main.go` 执行命令时，建议显式传 `-c ./config.yaml`，避免临时可执行路径导致配置查找失败。

## 目录职责

- `cmd/`：命令入口，包括 `service`、`worker`、`cron` 和 `command` 子命令。
- `config/`：配置结构、默认值、加载、热更新与测试配置替换。
- `data/`：MySQL、Redis 初始化和 `data/migrations/` 数据库迁移。
- `internal/routers/`：声明式路由树、Gin 路由注册、路由校验和 API 元数据来源。
- `internal/controller/`：控制器层，只做参数绑定、调用 service、统一响应。
- `internal/validator/form/`：请求参数结构与 `binding` 校验规则。
- `internal/service/`：业务逻辑、事务、权限同步、审计差异等。
- `internal/model/`：GORM 模型、基础 CRUD、数据库表结构映射。
- `internal/resources/`：响应资源转换，避免直接暴露模型内部结构。
- `internal/middleware/`：认证、CORS、请求语言、日志、审计、依赖就绪守卫等。
- `internal/access/`：Casbin、权限缓存、菜单/API/用户权限同步。
- `internal/queue/`、`internal/jobs/`：Asynq 队列和任务处理器。
- `internal/cron/`、`cmd/cron/`：内置任务定义、cron handler 注册和调度。
- `pkg/`：可复用公共工具。
- `tests/`：路由和接口集成测试。

## 代码风格

- Go 代码必须通过 `gofmt`，import 顺序交给 `goimports` 或等价工具处理。
- 遵循现有包职责，不把 controller、service、model、resource 的逻辑混在一起。
- 控制器统一嵌入 `controller.Api`，成功响应用 `api.Success(c, data)`，错误响应优先用 `api.Err(c, err)`。
- 业务层错误优先返回 `internal/pkg/errors.NewBusinessError(...)`，不要在 service 中直接拼 HTTP 响应。
- 普通技术错误保留原始 `error` 并向上返回，由 controller 的 `Err` 统一归类。
- 日志使用 `internal/pkg/logger` 的 zap logger，字段化记录关键上下文，不用 `fmt.Println` 做业务日志。
- 表单校验写在 `internal/validator/form/`，枚举字段使用 `oneof`，ID 数组使用 `dive,gt=0`。
- 响应结构写在 `internal/resources/`，列表/详情字段要和前端契约一致，不直接返回 GORM 模型。
- 模型使用 `BaseModel` 或 `ContainsDeleteBaseModel`，新增模型要实现 `NewXxx()` 和 `TableName()`。
- 事务优先复用已有的 `access.RunInTransaction`、`PermissionSyncCoordinator` 或当前模块既有事务写法。
- 不新增不必要依赖；已有库能解决的问题，优先使用当前项目已经引入的库。
- 注释保持简洁，优先解释业务规则、风险和非显然决策，不写机械化注释。

## 新增或修改接口

新增接口通常按这个顺序处理：

1. 在 `internal/validator/form/` 定义请求参数和校验规则。
2. 在 `internal/service/` 实现业务逻辑、事务和错误码。
3. 在 `internal/resources/` 定义或调整响应结构。
4. 在 `internal/controller/` 绑定参数、调用 service、返回统一响应。
5. 在 `internal/routers/admin_router.go` 的 `AdminRouteTree()` 中声明路由。
6. 如接口需要权限控制，确认 `AuthModeNone`、`AuthModeLogin`、`AuthModeAuth` 是否正确。
7. 执行 `go run main.go -c ./config.yaml command api-route -y` 同步 API 路由表。
8. 在 `tests/` 或相关包内补充测试。

重要约定：

- 路由树是 API 权限元数据真相。不要只手写 Gin 路由或只改数据库权限表。
- `AuthModeAuth` 路由必须有默认菜单/API 绑定，或在测试中显式说明豁免原因。
- 权限缓存查不到路由时按 fail-closed 处理，表现为无接口操作权限，不是放行。
- 修改接口认证模式后，必须考虑数据库 `api` 表和 Redis 权限缓存是否仍保留旧值。

## 错误码与 HTTP 状态码

- 业务响应体固定包含 `code`、`msg`、`data`、`cost`、`request_id`。
- 成功 `code=0` 对应 HTTP `200`。
- 参数错误、资源不存在、业务冲突等客户端类失败通常无需登记 HTTP 状态，默认归为 `400`。
- 登录态、权限、未找到、依赖未就绪等协议级错误在 `internal/pkg/errors/http_status.go` 显式映射。
- 写库失败、令牌生成失败、依赖初始化失败等服务端技术故障必须登记到 HTTP `500`，避免监控误判为客户端问题。
- 新增错误码时同步检查：
  - `internal/pkg/errors/code_*.go`
  - `internal/pkg/errors/zh-cn*.go`
  - `internal/pkg/errors/en-us*.go`
  - `internal/pkg/errors/http_status.go`
  - `docs/error_codes.json`
  - `docs/error_codes.ts`

## 数据库与迁移

- 迁移文件位于 `data/migrations/`，命名格式为 `YYYYMMDDHHMMSS_desc.up.sql` 和 `YYYYMMDDHHMMSS_desc.down.sql`。
- 每个版本必须恰好有一对 `up/down`，版本号递增且唯一。
- 创建和校验迁移优先使用项目命令：

```bash
go run main.go -c ./config.yaml command migrate create add_example_table
go run main.go -c ./config.yaml command migrate check
```

- 未发布、未进入共享环境的开发期变更，可以直接收敛当前 migration；已发布或生产可达字段演进必须新增独立迁移并处理历史数据。
- `init-system` 会回滚并重建系统数据，只能用于本地初始化或明确允许重置的环境。
- 修改权限、菜单、API、默认系统数据时，要同时考虑初始化数据、运行时同步、缓存刷新和测试覆盖。

## 权限、菜单与 i18n

- 当前权限模型以数据库关系为真相，Casbin 负责最终接口判定。
- `api` 权限元数据来自 `AdminRouteTree()`，修改路由后要同步 API 路由表。
- 菜单列表/树返回当前语言 `title`；菜单详情返回 `title_i18n` 供编辑回填。
- 菜单写接口使用 `title_i18n`，`zh-CN` 和 `en-US` 至少一种非空。
- 菜单 i18n 是 Replace 语义；系统配置/字典 i18n 是 PATCH 语义，不能混用清洗策略。
- `is_auth=0` 菜单表示登录可见基础菜单，不等同于可分配角色权限。
- 角色权限保存真实授权节点，目录/父级展示由权限树推导，不要把不可授权分组节点强行入库。

## 文件、上传与审计

- 上传完成不能信任客户端回传的 `file_object_id`、`object_key`、`hash`、`size`、`mime`、`bucket`、`is_public`。
- 上传凭证/init 阶段必须服务端创建 session 并签发 `complete_token`，complete 阶段只使用 token 中的服务端元数据。
- 文件物理对象复用要同时考虑 `is_public` 和 `owner_uid`：公开对象可全局复用，私有对象仅同用户复用。
- 目录和文件都使用软删除时，级联删除、恢复和统计必须明确直属/递归语义。
- 审计日志队列在服务关闭流程中必须 drain；fallback 同步写库要支持可注入 DB，避免测试或停服窗口丢审计。

## 队列与定时任务

- `service` 负责对外 API，`worker` 消费 Asynq，`cron` 按时间触发任务。
- 不要把同一个周期任务同时注册到 `cron` 和 `worker`，否则会重复执行。
- 新增异步任务时，确认队列名已在 `queue.queues` 配置中，否则 worker 不会消费。
- 新增 cron 任务优先在 `internal/cron/registry.go` 声明内置任务定义和 handler，再由 `cmd/cron/tasks.go` 注册。
- 高风险 cron 必须有显式配置开关和日志告警。

## 测试要求

- 文档或注释改动至少执行 `git diff --check`。
- Go 代码改动至少执行：

```bash
gofmt -w <changed-go-files>
go test ./...
```

- 影响共享逻辑、权限、认证、迁移、队列、cron、配置或中间件时，视情况加跑：

```bash
go vet ./...
go mod verify
go run main.go -c ./config.yaml command migrate check
go run main.go -c ./config.yaml command init-system -y
```

- 新增接口必须在 `tests/` 增加对应接口测试，管理端接口默认放到 `tests/admin_test/` 并按资源域归类。
- 测试需要真实 MySQL 时使用已有 helper 跳过不可用环境，不要伪造通过。
- 不删除或弱化已有测试，不通过跳过校验、吞异常、硬编码返回来让测试通过。
- 无法完成验证时，最终说明必须明确写出未验证项、原因和建议验证命令。

## 文档维护

- 修改 `README.md` 中的命令、配置、接口契约、风险提示时，同步检查 `README.en.md`。
- 共享文档只写仓库事实、相对路径和可复用约定，不写个人机器的绝对路径。
- API 文档字段说明使用中文业务语义，枚举写清楚“值=语义”，示例值使用真实风格值。
- Apifox 导入应使用 `apiOverwriteMode=methodAndPath`，避免批量生成重复接口。

## Git 与协作

- 修改前先查看工作区状态，识别已有用户或其他 Agent 改动。
- 不回滚、不覆盖与当前任务无关的改动。
- 只修改完成任务所需文件，避免顺手重构。
- 发现实际代码、测试结果或风险范围与预期不一致时，先停止扩大修改，重新梳理方案。
- 最终回复应包含：做了什么、关键修改点、根因、验证方式、遗留风险或未验证项。
