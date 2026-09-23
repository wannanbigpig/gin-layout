# 三项目代码审查记录

> 日期：2026-09-22（第二轮：全量复核 + 新发现）  
> 方式：5 路子 agent 并行只读审查（后端核心安全 + 业务服务；前端、官网、跨项目契约各一路）+ 主线抽查  
> 范围：未改业务代码；file:line 为审查时读源码结论  
> 关联文档：`docs/REVIEW_AND_ROADMAP_2026-09.md`、`ROADMAP.md`、`docs/COMPATIBILITY.md`  
> 上一版结论已逐条复核：B4 降级、change_diff 前端 XSS 撤销、W2 数字修正、三仓版本部分不成立、C1 加重。

## 审查对象

| 项目 | 路径 | 说明 |
|---|---|---|
| 后端 | 本仓库 `gin-layout` | Gin 后台脚手架 v0.10.0 |
| 前端 | `x-l-admin-vue3`（remote: `go-admin-ui`） | 配套管理端 v0.10.0 |
| 官网 | `go-layout-doc`（package: `gin-layout-website`） | 文档站 `version=0.0.0` |

**总评**：分层、声明式路由、Casbin、JWT 双 Token 主链路整体扎实；问题集中在热更新生命周期、错误语义、上传默认与超时、三仓契约漂移、官网内容治理。无已证实的登录/刷新/菜单主链路 P0 断裂。

---

## 一、高优先级问题

### 后端 `gin-layout`

| # | 问题 | 位置 | 影响 | 建议 | 分类 |
|---|---|---|---|---|---|
| B1 | 配置热更 `Finalize` 立即关旧 MySQL/Redis 池，无 in-flight drain | `data/config_reload.go:87-95`、`internal/runtime/config_reload.go`、`config/config_load.go` | 热更窗口在途请求可能 `database is closed`；与优雅关闭 drain 策略不一致 | 延迟关闭 / 引用计数 / 与 HTTP drain 对齐 | 已确认（触发频率=风险） |
| B2 | 限流配置启动时闭包冻结，热更不生效 | `internal/routers/router.go:52-70`、`internal/middleware/rate_limit.go:31-48,81` | 运维改 `rate_limit_*` 无效 | 每请求读配置，或登记 restart-required | 已确认 |
| B3 | 生产默认 DB 弱密码 `root1234`、空 Redis 密码；仅 JWT 有 prod 弱口令门禁 | `config/autoload/mysql.go:40`、`config/autoload/redis.go` | 忘改直上生产可被扫库 | 对齐 JWT secret 策略，prod 启动失败 | 已确认 |
| B5 | AdminUser 删除把事务内任意错误一律吞成 `DeleteUserFailed` | `internal/service/admin/admin_user_mutation.go:654-655` | 403/404/500 无法区分；与同文件 `handleMutationError` 不一致 | 业务 `BusinessError` 上抛，仅技术错包装 | 已确认 |
| B6 | API 权限更新：DB 已提交但 `RefreshCache` 失败仍返回 error | `internal/service/api_permission/api.go:74-82` | 假失败 → 调用方重试/重复审计 | 缓存失败降级日志 + 后台重试，对外成功 | 已确认 |
| B7 | `sys_config` 审计 snapshot 失败 `return "", nil` 吞错；删除后 `RefreshCache` 失败假失败 | `internal/service/sys_config/audit_diff.go:78,104,155`；同类 `auth_storage_config.go:47`、`auth_sys_config.go:101` | 审计 change_diff 静默丢失；与 B6 同型假失败 | 返回 err 或降级日志，勿吞 snapshot 错 | 已确认（新） |
| B8 | cron queue fallback 注册不检查任务 `enabled` | `internal/cron/queue_fallback.go:13-34` | 运营已禁用的低风险周期任务仍可能被执行 | 注册/执行时读 DB 运行缓存 | 已确认 |
| B9 | 菜单删除先读 `ChildrenNum` 再删，事务内无行锁重检 | `internal/service/menu/menu_query.go:56-69` | 并发加子+删父 TOCTOU | 事务内 `FOR UPDATE` 重检 | 已确认 |
| B10 | fsnotify 回调裸写 `initErr` 无独立锁 | `config/config_load.go:187-189` | `-race` 可复现数据竞争 | 赋值纳入与读路径同一锁 | 已确认 |
| B11 | Casbin 用 `Request.URL.Path` 而非 `FullPath` | `internal/middleware/admin_auth.go:55` | 动态路由引入后策略路径错位 | 切 `c.FullPath()`（静态路由暂无问题） | 风险 |

### 前端 `x-l-admin-vue3`

| # | 问题 | 位置 | 影响 | 建议 | 分类 |
|---|---|---|---|---|---|
| F1 | 本地上传/头像继承全局 10s axios 超时（OSS 直传用裸 axios 不受影响） | `src/utils/request.ts:46`、`src/modules/shared/constants.ts:7` | 大文件/慢网 `ECONNABORTED` | 上传路径 `timeout: 0` 或按分片独立超时 | 已确认 |
| F2 | 会话过期 / WS 强踢只清本地，不调 `/auth/logout` | `src/stores/auth.ts:259-285`、`src/stores/notification.ts:628`、`src/router/guard.ts` | refresh cookie 可能残留，刷新静默再登 | 统一先 logout 再 reset；与后端确认 kick 是否清 cookie | 风险（依赖后端 revoke） |
| F3 | 上传默认 `is_public=1`（前后端双默认）；批量 complete 不带 `is_public` 依赖后端 `IsPublic:1` | `src/modules/system/service.ts:496,557,628,655`；`src/composables/useFileUpload.ts:435-468`；`form/file_resource.go:168,230` | 敏感文件默认公开 | 默认私有，UI 显式选公开 | 已确认 |

### 官网 / 跨项目契约

| # | 问题 | 位置 | 影响 | 建议 | 分类 |
|---|---|---|---|---|---|
| W1 | `marked.parse` 无 sanitize + `v-html` | `src/DocsApp.vue:60,144` | 恶意 PR 在 md 写脚本可上线执行 | DOMPurify 或拒 raw HTML；CI 扫 md 内联标签 | 已确认 |
| W2 | API 目录手写漂移：约 **133** 条 vs openapi **145** ops（交集 131，缺 14）；openapi 反缺 `/ping`、`/health/readiness` | `src/data/api-catalog.js` ↔ `docs/openapi.yaml` | 缺 `login/options`、`sms*`、`todo/*` 等 | openapi Catalog 生成/校验，CI diff | 已确认（数字修正） |
| W3 | `App.vue:357` 引用 `x-l-admin-super-admin-api-list.png` 但文件未入 git | `src/App.vue:357`、`public/screenshots/` | 干净克隆/CI 预览图 404 | `git add` 该 png；CI 校验引用资源存在 | 已确认（新，高） |
| C1 | 错误码 11011 语义三层错位 + 全局静默 | 后端 `code_file.go:15-16`（11011=LocalStorageMultipartUnsupported，11012=FileTooLarge）↔ 前端 `request.ts:35-37`、`system/service.ts:643-646`、`useFileUpload.ts:143-148` | 超限被当成 11011 静默/错误降级；注释写“未配置/未启用”“直传超限”均不符 | 修正注释与降级判定（超限用 11012）；前端接入共享 TS 或 CI 校验 | 已确认（加重） |
| C2 | `ApiResponse` 缺 `cost`/`request_id`；分页丢 `last_page` | 后端 `response.go:17-23`、`resources/base.go:168` ↔ 前端 `types/common.d.ts:4-8`、`request.ts` normalize | 类型与真实 Result 不一致 | 类型补字段；normalize 映射或文档声明丢弃 | 已确认 |
| C3 | `docs/error_codes.ts` 前端零引用；硬编码双轨 | `docs/error_codes.ts`、前端硬编码 `11011/20048/...` | 错误码漂移无门禁 | 接入 import 或 CI 比对 | 已确认 |
| C4 | 三仓版本未对齐：后端/前端 0.10.0，官网 `0.0.0`；COMPATIBILITY 未覆盖官网 | 各仓 `package.json` / 模块版本 | 兼容矩阵失真 | 版本 CI 交叉校验 | 已确认（旧“三仓均 0.10.0”部分不成立） |

---

## 二、中低优先级问题（应排期）

### 后端

- 通知列表 `context.Background()`，客户端断开查询仍跑满（`internal/service/notification/service.go:441`；controller 无法传 ctx）。
- 服务层 `ExportCenterService.List` 无属主强制且接受 `trigger_user_id`（`export_center.go:469-474`）；**HTTP 已走 `MyList`（`auth_export_center.go:27`），B4 降级为“方法残留风险，勿再挂路由”**。
- `change_diff` **入库无统一脱敏**；个人读侧白名单摘要，管理员 `Detail` 返回完整 JSON（依赖 `requestLog:*`）。调用方规则不齐（如 `adminUserDiffRules` 含明文 phone/email）。
- 认证每请求查 Redis 黑名单 +（MySQL 可用时）登录会话撤销（`login_token_ops.go`）；正确但高 QPS 成本 → Redis 正向缓存 + 撤销失效（可选改进）。
- 超管 `IsSuperAdmin` 来自 JWT claims 不回源；依赖业务层 revoke。
- 单实例限流/非高危 cron：多实例需网关限流或文档强制边界。
- 登录限流与全局限流各自独立 map，预算不叠加（`rate_limit.go`）。
- `ClientIP` 依赖 TrustedProxies，云上误配可导致限流/日志 IP 欺骗。
- CSRF 仅靠 `X-Requested-With`（`auth_cookie_csrf.go`）；CORS 热更路径无二次校验。
- `role.ToRawCollection` 模型直出可能带出未文档化字段（`role.go:67`，MEMORY 已记债）。
- `SysMessageManageList` 等 form/resource 声明无对应路由（死代码/未完成面）。
- `gofmt -l` 9 文件未格式化：`config/autoload/logger.go`、`internal/controller/sys_base.go`、`internal/model/i18n_test.go`、`internal/pkg/response/response.go`、`internal/service/file_access_aggregator{,_test}.go`、`internal/validator/form/common.go`、`pkg/utils/crypto/types.go`、`tests/admin_test/menu_test.go`。

### 前端

- `normalizeDetailData` 可能误剥业务字段 `data`（`modules/shared/response.ts:59-62`）。
- `checkingTokenPromise` / `ensureTokenValid` 共享 promise 未按 token 键绑定（`stores/auth.ts:187-209`）。
- 日志 `change_diff`/`request_body` 为 **`{{ }}` 文本插值，非 v-html，无前端 XSS**；风险改为完全依赖后端 mask（旧 XSS 定性**撤销**）。
- 登出不清字典 `localStorage`/内存缓存（`stores/auth.ts:239-254`、`useDictOptions`）。
- `v-permission` 里 `is_show=0` 仍隐藏（`directives/permission.ts:42`；API 侧 code 仍在，UI 隐藏语义需文档化）。
- `pnpm type-check` **已通过**；未跑 `test:run` / `lint` / e2e。

### 官网

- `element-plus` / `@element-plus/icons-vue` 零引用死依赖 + `vite.config.js` 死 chunk 配置。
- `DESIGN.md` 实为 Cal.com 模板误提交，与实现（`#1677ff` 蓝）完全不符。
- `dist` 中 `api-list.png` 与 `public/` MD5 不一致（JS/CSS/HTML 与现 dist 同 hash，仅该图过期）；源码 `© 2026` 硬编码（`App.vue:270`）。
- SEO/可访问性弱：仅 `og:title/description`，无 `og:image/url`、`canonical`、`robots.txt`/`sitemap`；tab 缺 `tabpanel`/`aria-controls`；docs 仅 `?guide=`。
- 脚手架残留：`HelloWorld.vue`、`logo.png`/`favicon.png` 约 1.44MB、未引用 `wide-users.png` 等。
- README 技术栈仍列 Element Plus；`package-lock` 全走 `registry.npmmirror.com`。
- WS 示例缺 `code/msg/cost`（`api-examples.js:135`）；正文约定已含 `cost/request_id`。
- 临时目录 `vite build` **exit 0**；C1/11011 **不属本仓**。

---

## 三、已验证的良好点（可依赖）

- 路由安全策略禁止 Public 组下藏鉴权路由；Refresh/Ticket 投影为 None 但有各自校验。
- 认证 fail-closed：blacklist Redis 失败回源 DB；Casbin 重载失败拒绝判权。
- Refresh 轮换 `FOR UPDATE`；JWT secret 热更保持旧值并标 restart；prod JWT 弱口令启动校验存在。
- 导出 **HTTP 已属主强制**（`MyList`/`MyDetail`）；上传 complete 以 session/`complete_token` 为准，未复现信任客户端元数据。
- 菜单 i18n（列表 `title`、详情/写 `title_i18n`）、软删级联、个人日志强制 `operator_id`、导出 CSV 流式等抽查正常。
- 迁移 6 对 up/down 完整；产品/文件删除事务与引用检查到位。
- 前端分页字段与后端 `Collection` 一致；token 不落 localStorage；refresh 单飞 + `sessionVersion` 有单测；**无 `v-html`**。
- 后端/前端版本 0.10.0；兼容矩阵文案一致；`Menu.Code` → `code` 一致。
- `CGO_ENABLED=0 go vet ./...` 通过；前端 `type-check` 通过；官网临时 build 通过。
- govulncheck 可达漏洞 0（模块级 openpgp 告警未导入，不表述为“零漏洞”）。

---

## 四、后续完善路线

### 第一阶段：止血（1–2 个迭代）

1. 后端：热更连接池 drain（B1）、限流热更/重启标记（B2）、prod 弱密码启动失败（B3）、B5–B10。
2. 前端：上传超时（F1）、登出/强踢链路（F2）、`is_public` 默认私有（F3）。
3. 官网：sanitize 或 CI 扫 md（W1）；提交 `api-list.png`（W3）；修正 11011/11012 注释与降级判定（C1）。

### 第二阶段：契约单一真相

1. openapi Catalog → 生成/校验官网 `api-catalog.js`（禁止手维第二份路径表）；openapi 补 `/ping`、`/health/readiness`。
2. `docs/error_codes.ts` 接入前端或 CI 比对硬编码码（C3）。
3. CI 交叉校验：三仓版本 + 兼容矩阵 + error_codes + openapi 路径（C4）。
4. `ApiResponse` 补 `cost?/request_id?`；`PageData` 增 `lastPage?` 或文档声明丢弃（C2）。

### 第三阶段：中长期架构演进（已列入规划）

1. **限流配置化双驱动**：`config.yaml` 增加 `rate_limit_driver: "memory"/"redis"`，支持单机内存与 Redis+Lua 分布式令牌桶平滑热更切换，带本地自动熔断降级。
2. **敏感字段通用透明脱敏门禁**：以现有「通用字段」（`audit.sensitive_fields`）为中枢，在 `auditdiff` 注入切面门禁，使 HTTP 请求日志与业务变更审计（Change Diff）统一透明脱敏。
3. **导出中心方法属主防御**：底层收敛私有化 `List`，对外仅暴露编译期强绑定当前用户的 `ListMyExports`，全局审计接口独立权限隔离。
4. **`sys_config` 复杂配置解耦与独立分表**：保留基础标量/规则白名单于 `sys_config`；将云厂商多驱动存储拆分为 `sys_storage_config`（带 AES 密文存储），短信渠道与场景拆分为 `sys_sms_channel` + `sys_sms_template`。
5. **端到端契约与联调质量**：OpenAPI 生成 TS Types；三仓集成与强踢/上传越权自动化 E2E。

### 暂不进默认核心（沿用 ROADMAP）

多租户、ABAC、OpenTelemetry、微服务等仅作扩展点评估。

---

## 五、验证边界

| 状态 | 内容 |
|---|---|
| 已验证 | 上表关键 file:line 读源码确认；`gofmt -l`、`CGO_ENABLED=0 go vet`、前端 type-check、官网临时 build、api-catalog↔openapi 集合 diff |
| 复核修正 | B4 HTTP 已修（降级）；change_diff 前端 XSS 撤销；W2 改为 ~133 vs 145；三仓版本部分不成立；C1 加重 |
| 推断 | 热更关池实际触发频率；强踢后 Cookie 是否已被后端清掉；export `List` 未来误接线 |
| 未执行 | `go test ./...`、`tests/admin_test`、pnpm test/lint/e2e、真实联调、压测、带 CGO 的完整 vet（本机 Xcode license 未同意） |

---

## 六、TypeSafe 实验与据此落地的修复（2026-09-23）

> 使用 TypeSafe `jev-latest` / `jev-1.13.0`，对“剩余未修项优先级、运行时候选、CI 形态、is_show 语义、路由风险”做结构化判断；仅采纳高置信且可落地项。

### 实验摘要

| 实验 | 关键答案 | 含义 |
|---|---|---|
| E1 剩余项优先级 | `CI_GATE` 0.68；`do_now_runtime` **0.93**；风险分 2.43（中高） | 先补契约门禁，且存在应立刻修的运行时问题 |
| E5 运行时候选 | `PROMISE_KEY` 选择 **0.91 / confidence 0.88**；`promise_is_bug` 0.61 | 明确优先修 `checkingTokenPromise` 按 token 隔离 |
| E6 CI 形态 | `gate_blocking` **0.81**；优先 error_codes 同步 / 版本 / check:api | 契约 CI 应阻断发布 |
| E3 is_show | `is_bug` 0.36；`keep_and_document` | **不改行为**，文档化即可 |
| E2 change_diff | 偏 `entity_rules_only` | 不引入全局写侧 PII 门禁（本轮不改） |
| E4 路由 | `release_blocker` 0.72、`secret_reveal` 优先，但 `storage_secret_ok` 仅 0.44 | 已有 `SecurityAccessTokenPermission` + 前端 `storage:secret`；**证据不足，本轮不改 Secret 接口** |

### 已落地

1. **前端** `checkingTokenPromise` 按 `token:expires` key 隔离；token 轮换后不再复用在途校验；`resetAuthStore` 清空 key；新增单测（`stores/auth.test.ts`）。
2. **后端 CI** `go.yml` 增加 `gen-error-codes` 漂移门禁 + `gofmt -l` 门禁（与已有 openapi 漂移门禁对齐）。
3. **官网 CI** 新增 `.github/workflows/ci.yml`：`npm run check:api` + `build`。
4. **is_show 文档** `v-permission` / `hasButtonPermission` 注释标明：`is_show=0` 是 UI 可见性，API 仍以权限码为权威。

### 验证

- 前端 `type-check` 通过；`auth.test.ts` + `permission.test.ts` 19 通过。
- 官网 `check:api` 147 端点零漂移；`vite build` 通过。
- `gofmt -l` 为空；`CGO_ENABLED=0 go vet ./config/... ./internal/routers/...` 通过。
- 未跑全量 `go test` / `tests/admin_test` / FE 全量 test:run。

---

## 七、子 Agent 分工备查

| Agent | 覆盖 | 状态 |
|---|---|---|
| 后端核心安全 | middleware / routers / casbin / auth / config / cmd / cron | partial（B1/B2/B8 本机+子 agent 双确认；部分项由主线/业务路补齐） |
| 后端业务服务 | controller / service / model / queue / migrations | success（B4 降级、B5/B6/B7、change_diff 写路径） |
| 前端 | x-l-admin-vue3 全量 src + 契约抽查 | success（type-check 过） |
| 官网 | go-layout-doc 全量 + 临时 build | success |
| 跨项目契约 | 版本矩阵 / 错误码 / OpenAPI / 分页 / gofmt/vet | success |
| TypeSafe 实验 | 优先级 / 运行时 / CI / is_show / 路由 | success（E1–E6；据此落地 PROMISE_KEY + CI + 文档） |
