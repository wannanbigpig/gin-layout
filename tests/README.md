# tests 目录说明

`tests` 是项目 HTTP 接口集成测试入口，以 `internal/routers.AppRouteTree()` 为路由真相。

当前非 Release 模式共 `131` 个接口：

- 根路由 `2` 个：`GET /ping`、`GET /health/readiness`
- 管理端 `/admin/v1` 路由 `129` 个
- Release 模式不注册 `GET /admin/v1/demo`，因此为 `130` 个

所有声明路由都至少会实际发起一次请求：根路由和公开路由使用显式场景，受保护路由由 `route_coverage_test.go` 自动生成鉴权用例；全部受保护 GET 和写接口还会带认证进入参数或业务处理层。新增路由若没有进入对应覆盖规则，测试会直接失败。

`admin_test/route_scenarios_test.go` 进一步要求每个管理端路由显式归类：按当前实际列表为 `91` 个已有业务成功或状态流（包括 `GET /admin/v1/system/file/upload/options`）、`24` 个覆盖参数、NotFound、权限或幂等契约、`14` 个受 Redis、queue、multipart 文件系统或隔离对象存储约束。新增路由未分类、重复分类或删除路由后遗留分类都会导致测试失败。

## 真实 MySQL 约束

- 接口测试只读取项目根目录 `config.yaml`，不再回退 `config/config.yaml.example` 或临时配置。
- `mysql.enable` 必须为 `true`，连接或健康检查失败时测试初始化直接失败，不允许 `Skip`。
- 涉及系统配置和字典的流程会检查所需表；缺表时直接提示先执行迁移或初始化。
- 测试会写入并清理带 `test-auto-` 前缀的数据。强烈建议让 `config.yaml` 指向专用测试库，不要指向生产库。
- 测试不会自动执行 `init-system`。首次准备专用测试库时可显式执行：

```bash
go run main.go -c ./config.yaml command migrate up
# 仅允许重建的专用测试库才执行：
go run main.go -c ./config.yaml command init-system -y --backup-confirmed --confirm-database <database>
```

CI 使用 MySQL service 和专用 `go_layout_test` 数据库，先初始化系统数据，再执行 `make test-coverage`。

## 目录职责

- `test.go`：加载根 `config.yaml`、初始化真实 MySQL、Redis（按配置）、验证器和 Gin Router。
- `ping_test.go`、`readiness_test.go`：根路由测试和根路由覆盖守卫。
- `admin_test/admin_test.go`：管理端 `TestMain`、请求发送、认证头和统一响应解析。
- `admin_test/route_coverage_test.go`：全部管理端声明路由的自动覆盖守卫、匿名鉴权边界和认证后 handler smoke 测试。
- `admin_test/*_test.go`：按资源域组织成功、校验、鉴权和状态流测试。
- `admin_test/test_helpers_test.go`：真实库测试数据查询、创建和清理辅助函数。

## 前置流程原则

- CRUD 流程按“创建前置数据 → 调接口 → 回读数据库/详情接口 → 更新/状态转换 → 删除或清理”执行。
- session、notification、file folder 等状态接口先创建可控且唯一的前置记录，并用 `defer` 清理。
- Redis、queue、对象存储等非 MySQL 依赖按测试 Router 的实际初始化状态断言成功、`ServiceDependencyNotReady` 或参数校验结果，不以 `Skip` 冒充通过。
- 文件直传、分片和硬删除不使用配置中的真实 OSS 做破坏性流程；这些端点当前覆盖认证和请求校验，完整对象存储生命周期应在隔离存储环境执行。

## 常用命令

```bash
go test ./tests/... -count=1
go test ./tests/admin_test -count=1
go test ./...
make test-coverage
```

`make test-coverage` 使用全部测试统计全仓语句覆盖率，默认最低门槛为 `60%`；可通过 `COVERAGE_MIN` 和 `COVERAGE_FILE` 覆盖默认值。

## 维护规则

- 新增接口时必须补业务场景；自动路由守卫只保证最低鉴权/handler 覆盖，不能替代成功路径测试。
- 管理端接口默认放到 `tests/admin_test`，同一资源的读写和状态流放在同一个 `*_test.go` 文件。
- 测试数据必须唯一、可回读、可清理，不依赖用例执行顺序。
- 不允许恢复 MySQL fallback、数据库不可用 `Skip`、吞掉前置失败或只断言非 404。
