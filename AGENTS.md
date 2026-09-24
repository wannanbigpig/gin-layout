# AGENTS.md

本文件说明 `gin-layout` 的项目结构、代码风格、接口与数据约定及验证要求。

## 项目概览

- 这是一个基于 `Gin` 的后台管理系统脚手架，模块名为 `github.com/wannanbigpig/gin-layout`。
- 入口是 `main.go`，实际命令由 `cmd/` 下的 `Cobra` 命令注册。
- 后台运行模型分为三类进程：
  - `service`：HTTP API 服务；
  - `worker`：Asynq 异步任务消费进程；
  - `cron`：定时任务调度进程。
- 主要能力包括 JWT 认证、RBAC 权限、声明式路由、请求/登录日志、文件上传、系统配置/字典、任务中心、配置热更新和健康检查。

## 常用命令

以下为开发命令索引，各类变更的验证要求见“测试要求”。

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
- `internal/access/casbin/`：Casbin 初始化、策略加载与持久化。
- `internal/service/access/`：权限缓存、菜单/API/用户权限同步和事务协调。
- `internal/queue/`、`internal/jobs/`：Asynq 队列和任务处理器。
- `internal/cron/`、`cmd/cron/`：内置任务定义、cron handler 注册和调度。
- `pkg/`：可复用公共工具。
- `tests/`：路由和接口集成测试。

## HTTP 请求与数据返回链路

服务启动入口为 `main.go` → `cmd/` → `cmd/service/`；`cmd/bootstrapx/` 负责公共初始化，`config/` 加载配置，`data/` 初始化数据库等依赖，`internal/routers/` 构建 Gin 引擎。

普通管理端 JSON 接口的调用链：

```text
HTTP 请求
  → internal/routers/                  匹配方法、路径和声明的安全策略
  → internal/middleware/               执行全局及路由中间件
  → internal/controller/admin_v1/      接收请求，调用参数校验与 service
  → internal/validator/               使用 internal/validator/form/ 的结构绑定校验参数
  → internal/service/<业务模块>/        业务规则、数据权限、查询条件与事务
  → internal/model/ → GORM → MySQL     读取或写入数据，连接由 data/ 管理
  → internal/resources/               service 将模型转换为响应资源
  → controller.Api.Success / Err       控制器提交成功结果或错误
  → internal/pkg/response/             包装统一响应并写出 JSON
```

| 环节 | 实际入口与职责 |
|---|---|
| 路由与中间件 | `internal/routers/router.go` 注册全局中间件；`admin_router.go` 声明业务路由，`catalog.go` 解析继承的安全策略和处理器链。认证、权限、限流或依赖守卫失败时可提前响应，不进入 controller。 |
| 控制器与参数 | `internal/controller/admin_v1/` 嵌入 `controller.Api`；`internal/validator/form/` 定义请求结构，`internal/validator/binding.go` 的 `CheckQueryParams` / `CheckPostParams` 绑定校验。校验失败已写出统一错误，控制器直接 `return`，不要再次响应。 |
| 业务与读取数据 | `internal/service/` 组织业务条件、数据范围和事务；通过 model 的 CRUD、`ListPageE` 或既有 GORM 查询读取数据。`internal/model/base.go` 提供 `GetDB`，事务内复用同一 `tx`，不在 controller 查询数据库。 |
| 资源转换 | `internal/resources/` 定义对外字段及转换器；service 调用 `ToStruct` / `ToCollection` 等转换结果后返回控制器。`resources/base.go` 提供通用转换和分页结构。 |
| 统一写出 | `internal/controller/sys_base.go` 的 `Api.Success` / `Api.Err` 调用 `internal/pkg/response/response.go`；该层处理业务码、HTTP 状态、文案、耗时和请求 ID，最终调用 Gin 的 `AbortWithStatusJSON`。 |

实际示例：`GET /admin/v1/product/list` → `admin_router.go` 的 `productGroup` → `internal/controller/admin_v1/auth_product.go` 的 `ProductController.List` → `form.ProductList` 与 `CheckQueryParams` → `internal/service/product/product.go` 的 `ProductService.List` → `model.ListPageE` 查询 `model.Product` → `resources.NewProductTransformer().ToCollection` → `api.Success`。查询条件包含产品筛选和数据权限，返回前补充资源展示字段。

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

新增接口通常按以下职责处理，复用已有实现；修改接口只涉及受影响部分：

1. 在 `internal/validator/form/` 定义请求参数和校验规则。
2. 在 `internal/service/` 实现业务逻辑、事务和错误码。
3. 在 `internal/resources/` 定义或调整响应结构。
4. 在 `internal/controller/` 绑定参数、调用 service、返回统一响应。
5. 管理端路由在 `internal/routers/admin_router.go` 的 `AdminRouteTree()` 及对应子组中声明。
6. 通过 `RouteGroupDef.Security` 选择凭证与权限策略；`AuthMode` / `api.is_auth` 仅为兼容投影，不作为独立配置入口。
7. 路径、方法、认证策略或 API 展示元数据变化时，在已授权的目标环境执行 `go run main.go -c ./config.yaml command api-route -y`。该命令会写 API 表并同步权限；仅内部实现变化无需同步，只读分析或目标环境未明确时报告待同步项。
8. 新增接口在 `tests/` 补业务场景，相关包测试按需补充。

重要约定：

- 路由声明树经 `CompileRouteTree` 生成统一 `Catalog`，供 Gin 注册、API 同步、OpenAPI 和测试复用。不要只手写 Gin 路由或只改数据库权限表。
- `SecurityAccessTokenPermission`（投影为 `AuthModeAuth`）路由必须有默认菜单/API 绑定，或在测试中显式说明豁免原因。
- `SecurityRefreshCookie`、`SecurityWebSocketTicket` 虽投影为 `AuthModeNone`，仍有各自凭证校验，不能据此视为公开接口。
- 普通用户访问权限路由由 Casbin 判定，未获授权时拒绝；API 元数据缓存未命中允许回源数据库，不等同于权限拒绝。
- 修改接口认证模式后，必须考虑数据库 `api` 表和 Redis 权限缓存是否仍保留旧值。

## 统一响应与错误码

普通 JSON 业务接口复用 `internal/pkg/response/response.go` 的 `Result`，不在各控制器另定义响应外壳。文件下载、重定向和 WebSocket 等按各自协议返回。

分页成功响应示例（空列表）：

```json
{
  "code": 0,
  "msg": "OK",
  "data": {
    "total": 0,
    "per_page": 10,
    "current_page": 1,
    "last_page": 1,
    "data": []
  },
  "cost": "2.31ms",
  "request_id": "7d9d6d0e-51a3-4b80-a3b2-6ec759f5cd30"
}
```

- `code` 为整数业务码，`msg` 为文案，`data` 为业务数据；`cost` 是耗时字符串，`request_id` 来自请求上下文。`RequestCostHandler` 记录开始时间并生成请求 ID。
- `api.Success(c, resource)` 将结构体或 map 放入 `data`；`api.Success(c, nil)` 返回 `data: {}`。直接传入非 nil 的数组、切片或标量时，响应层包装为 `data: {"result": ...}`；nil 数据（含 typed nil）返回 `{}`。
- 分页使用 `resources.Collection`：分页字段位于 `data.total/per_page/current_page/last_page`，记录数组位于 `data.data`。通用 `ToCollection` 的空列表返回 `[]`，总数为零时 `last_page=1`。
- service 返回业务资源和 `error`，由 controller 分别调用 `api.Success` 或 `api.Err`；无返回数据的写操作使用 `api.Success(c, nil)`。
- `api.Err` 按业务错误携带的文案、文案 key 或错误码生成响应；依赖未就绪归为 `ServiceDependencyNotReady`，其他未分类技术错误归为 `ServerErr`，原始异常记日志，不直接暴露给客户端。
- `msg` 默认按请求语言解析，无请求语言时使用配置语言；业务显式文案优先。失败响应同样使用 `Result`，未附带业务数据时 `data: {}`。
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
- `init-system` 会清空目标数据库对象并重新执行迁移，不属于常规验证，仅用于已授权的数据库重建。`-y` 只跳过普通确认；非交互执行还需真实满足备份确认并传入匹配的数据库名（`--backup-confirmed --confirm-database <database>`），详见 `tests/README.md`。
- 修改权限、菜单、API、默认系统数据时，要同时考虑初始化数据、运行时同步、缓存刷新和测试覆盖。

## 权限、菜单与 i18n

- 当前权限模型以数据库关系为真相，Casbin 负责最终接口判定。
- `api` 权限元数据来自 `AppRouteTree()` 编译出的 `Catalog`，其中管理端路由由 `AdminRouteTree()` 声明；同步条件见“新增或修改接口”。
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
- 同一周期任务避免存在多个独立调度入口；worker 的历史 cron 队列兼容消费按 `queue.consume_cron_fallback` 配置处理，注册消费 handler 不等于新增周期调度。
- 新增异步任务时，确认队列名已在 `queue.queues` 配置中，否则 worker 不会消费。
- 新增内置 cron 任务在 `internal/cron/` 声明定义与 handler，并在目标环境执行 `go run main.go -c ./config.yaml command task sync-definitions` 同步定义；`cmd/cron/tasks.go` 根据 `task_definitions` 动态协调调度，通常无需修改调度器。新增任务无需执行 `init-system`。
- 高风险 cron 必须有显式配置开关和日志告警。

## 测试要求

- 文档或纯注释改动执行 `git diff --check`；涉及生成指令或构建约束时补相应验证。
- Go 代码修改先执行 `gofmt -w <changed-go-files>` 并运行相关测试；共享行为、权限/认证、跨模块或发布变更运行 `go test ./...`，必要时加 `go vet ./...`。
- 依赖变更或完整性问题运行 `go mod verify`；迁移变更运行 `go run main.go -c ./config.yaml command migrate check`。数据库重建不属于测试加跑项。
- 新增接口必须在 `tests/` 增加业务场景，自动鉴权/handler 覆盖不能替代业务场景；管理端接口默认放到 `tests/admin_test/` 并按资源域归类。
- `tests/` 接口测试只使用根目录 `config.test.yaml`（或 `GOTEST_CONFIG`），**不读业务 `config.yaml`**，会写入测试数据；执行前确认其指向专用测试库。MySQL 不可用时失败，不改为 Skip；其他包按各自已有测试约定执行。
- 不删除或弱化已有测试，不通过跳过校验、吞异常、硬编码返回来让测试通过。

## 文档维护

- 修改 `README.md` 中的命令、配置、接口契约、风险提示时，同步检查 `README.en.md`。
- 共享文档只写仓库事实、相对路径和可复用约定，不写个人机器的绝对路径。
- API 文档字段说明使用中文业务语义，枚举写清楚“值=语义”，示例值使用真实风格值。
- Apifox 导入应使用 `apiOverwriteMode=methodAndPath`，避免批量生成重复接口。
