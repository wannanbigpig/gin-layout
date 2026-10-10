# 版本、API 兼容和发布

## 后端与管理端独立版本

| 项目 | 唯一手工版本来源 | 发布产物 |
| --- | --- | --- |
| 后端 gin-layout | `internal/global/common.go` 的 `Version` | service/worker/cron 共用二进制、迁移、完整 API 文档、OpenAPI、错误码 |
| 管理端 x-l-admin-vue3 | `package.json` 的 `version` | 静态资源和 `build-info.json` |

后端与管理端分别打 `vMAJOR.MINOR.PATCH`（预发布可用 `-rc.1`），不强制版本数字一致。官网随文档更新部署，不维护独立版本号、Release 或历史文档 revision。兼容性看 `docs/release.json` 声明的对端范围 `[min, maxExclusive)`，以及对应版本的升级说明；部署前同时满足两端范围。当前候选组合是后端和管理端 `>=0.10.0 <0.11.0`；官网 API 页面显示后端接口版本和经过核对的管理端具体 Tag，而不只显示范围。

修复提升 patch，兼容功能提升 minor。破坏性变更在 `0.x` 提升 minor，`>=1.0.0` 提升 major；预发布与正式发布可以共享 core version。源码版本不带时间戳或 build metadata。构建提交、时间、dirty 状态单独记录；开发构建不能作为正式版本凭证。

Go 模块的 major 与 HTTP API major 是两个维度。如果模块将来正式发布 `v2`，需按 Go 规则同步 module/import `/v2` 并独立验证；HTTP `/admin/v2` 不能代替 Go 模块迁移。

## 每次发版必须更新

1. 当前仓库唯一版本来源；管理端同时维护自己的 lockfile，后端重新生成 OpenAPI 和错误码，更新 `docs/api/catalog.mjs`、`docs/api/examples.mjs` 的业务字段及示例，再执行 `node scripts/generate-api-documentation.mjs`。文档工具使用 Node.js 24，Go 服务运行不依赖 Node.js。
2. `docs/release.json` 的 `previousTag`、`changeType`、对端兼容范围及 `upgradeGuide`。首次候选 `previousTag=null`；后续填写最近已发布的 Tag，CI 必须获取完整历史。不能用更早基线绕过已发布变更。
3. `CHANGELOG.md` 对应版本条目和 `docs/upgrades/<version>.md`。说明从哪个版本升级、字段/错误/权限/配置差异、迁移顺序、回滚限制和验收。不适用的项也明确写出理由。
4. 运行发布检查、相关回归和构建，提交完整源码及文档后打 Tag。正式 Tag 检查要求干净 checkout 且 HEAD 正好对应版本 Tag。不要修改已发布 Tag 或重新上传不同内容的同名资源。GitHub Release 流水线禁止覆盖已有资产，预发布 Tag 标识为 prerelease。

后端离线检查：`go run . command release-check`；正式检查追加 `--tag v<version>`，不读数据库或业务配置。管理端：`pnpm run check:release`、`pnpm run test:release`。官网更新 API 文档时执行 `npm run sync:openapi`、`npm run check:api`、`npm test` 和 `npm run build`。加 `--tag` 的严格检查属于正式打 Tag 后的发布阶段，不为本地未提交候选绕过检查。

## 不兼容 API 升级

保持现有 `/admin/v1`，优先添加可选字段、新端点和兼容行为。删除/重命名字段、改变类型、必填性、分页结构、权限/认证、错误码语义、上传/队列数据契约，均需审查真实调用方；即使自动 diff 没报告，也必须在升级说明列明。

确需支持新旧客户端并存时，先实现并验证 `/admin/v2` 与保留的 `/admin/v1`，为旧接口标记 deprecated，发布替代路径、字段映射、客户端最低版本及明确的移除版本/期限。在观察到受支持客户端已迁移、队列已排空并验证回滚边界后，才能在 breaking release 删除旧接口。新旧 API 都通过路由 Catalog、鉴权、默认权限、资源转换和业务测试，不仅改文档地址。当前未创建虚假的 v2 接口或自动移除机制。

后端 `apiChanges` 对比 `previousTag` 的 OpenAPI 和错误码，逐项记录 `operation`、`kind=compatible|breaking`、`reason`。路由删除或安全策略变化、已发布错误码删除/改号只能判 breaking；参数/响应/components 等变化要求人工分类；新端点不能走 patch。示例：

```json
{
  "operation": "POST /admin/v1/example",
  "kind": "breaking",
  "reason": "删除旧字段，客户端改用 replacement；升级及回滚见本版本指南"
}
```

共享 schema 用 `operation=components`，server 变化用 `servers`，错误码用 `error-code <Name>`。`apiChanges` 不是业务字段完全覆盖的证明：当前生成器完整描述路由和安全策略，未完整描述业务 schema、HTTP 状态映射和错误语义。业务字段及新增错误码使用场景仍需按真实前端请求人工审查，不直接生成完整 Client。

## 迁移、部署与回滚

正式发布或进入共享环境后，历史 migration 的 up/down 均不可修改或删除；后续新增独立成对 migration 并处理历史数据。后端发布检查比较上一 Tag 的文件内容。`init-system` 是破坏性重建，不是常规升级或回归命令。

升级遵循：备份并记录部署组合 → 预发布验证新增迁移/数据转换 → 迁移 → 按指南同步 API/权限和任务定义 → 协调 service/worker/cron 和队列格式 → 部署兼容管理端 → 验收权限、核心业务与健康检查。若采用 expand/contract，可分阶段部署；删除字段、队列格式或密钥变化必须写出混用限制。

回滚二进制不自动回滚数据库。数据转换或删除通常不能安全 down，应选择向前修复或经过演练的备份恢复；升级说明必须说明旧服务能否读取升级后的数据。构建记录后端版本与 commit/time/dirty（`version --json`）、管理端 build-info、兼容范围、数据库 migration version 和文档快照来源，才能判断真实部署组合。

## 官网 API 版本与管理端 Tag

官网不维护独立版本号、Release 或历史文档 revision。`/api/` 的版本选择器只选择后端正式 Release 的 Tag（例如 `v0.10.0`，与 HTTP 路径 `/admin/v1` 是不同维度），并显示配套管理端具体 Tag。OpenAPI、错误码、后端升级说明和版本对应信息直接在线阅读；OpenAPI 另保留文件下载。教程随官网文档维护。

### API 文档随后端发布

业务目录源文件位于后端 `docs/api/catalog.mjs` 和 `docs/api/examples.mjs`，按真实 controller/form/resource 和管理端请求维护。后端 OpenAPI、错误码和升级指南更新后执行：

```bash
node scripts/generate-api-documentation.mjs
node --test scripts/api-documentation.test.mjs
node scripts/generate-api-documentation.mjs --check
```

提交生成的 `docs/api-documentation.json`，随后打后端 Tag 并发布 GitHub Release。该文件包含此版本完整业务目录、参数、响应示例、凭证说明、OpenAPI、错误码、升级指南和管理端兼容范围；CI、Release 及 `build.sh --release` 检查生成内容无漂移，发布资产与归档包含该文件。历史版本读取其 Tag 对应提交中的文件，不借用最新目录；旧 Tag 缺少文档时显示错误。

### 发布后自动发现与切换

官网访问时查询后端和管理端的公开 GitHub Release 列表；只接受 `vMAJOR.MINOR.PATCH` 的正式发布，排除草稿和预发布，按 SemVer 选择最新后端。解析真实 Tag 提交后加载完整文档，再按双方 `[min, maxExclusive)` 范围选择最新兼容的管理端正式 Tag；双方版本号不必相同。缺少兼容管理端时明确提示，后端文档仍可阅读。Tag 与源码版本、文档版本或发布兼容声明不一致时拒绝显示。

页面打开且可见时每 5 分钟检查，回到页面时也检查；“检查新版本”可立即刷新。版本列表缓存 5 分钟。手动选择或含 `apiVersion` 的链接固定所选版本，后续检测不自动跳到最新；例如 `/api/?apiVersion=v0.10.0&adminVersion=v0.10.2&view=upgrade`，其中 Tag 必须真实发布且相互兼容。`view` 可选 `openapi`、`errors`、`upgrade`、`versions`；阅读页链接支持刷新、分享和新标签页。

首次部署本功能后，新增符合此契约的后端/管理端正式 Release 无需重建官网即可被发现。仅推送 Tag、源码版本号变化或部署到服务器都不等同于正式 Release。GitHub 查询是公开只读请求，不在浏览器放 token；需要可访问 `api.github.com` 和 `raw.githubusercontent.com`。网络失败或限流会明确提示；无指定 Tag 时保留已标示来源的本地文档，指定版本无法验证时不显示其他版本冒充。这里检测已发布版本，不推断线上实际部署版本；部署身份仍通过后端 `version --json` 和管理端 `build-info.json` 核对。

### 本地文档与离线快照

官网构建保留一组有明确来源的内置文档。运行 `npm run sync:openapi`，从后端的版本文档和管理端源码保存 `src/data/api-source.json`、`src/data/api-documentation.json`、业务目录快照、`docs/openapi.snapshot.yaml` 和 `public/api-current/`。`BACKEND_ROOT`、`FRONTEND_ROOT` 可指定源码根目录；可选 `OPENAPI_PATH` 必须与声明的后端来源一致。不存在的来源或目录、版本、兼容范围不一致时失败。

```bash
# Tag 必须存在于对应仓库；版本文档需已提交到后端 Tag。
BACKEND_ROOT=../gin-layout FRONTEND_ROOT=../go-admin-ui \
BACKEND_TAG=v0.10.0 FRONTEND_TAG=v0.10.2 npm run sync:openapi
npm run check:api
```

显式 Tag 同步读取真实 Tag 内容。开发工作区显示“待发布”，保存 commit/dirty 来源。`npm run check:api` 核对方法、路径、标题、安全策略、业务示例覆盖、文档 SHA-256 与双方兼容范围；独立检出用 `--snapshot-only` 只验证仓库快照。官网 CI 对已标示 Tag 的内置来源检出对应 Tag 再校验。当前自动 OpenAPI 校验覆盖路由和安全元数据，业务字段仍需人工核对；示例齐全不等于所有字段均已自动验证。
