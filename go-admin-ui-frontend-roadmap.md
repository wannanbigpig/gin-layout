# go-admin-ui 前端架构改进与开源版本治理规划

> 项目：[`wannanbigpig/go-admin-ui`](https://github.com/wannanbigpig/go-admin-ui)  
> 对应后端：[`wannanbigpig/gin-layout`](https://github.com/wannanbigpig/gin-layout)  
> 评审日期：2026-09-14  
> 文档性质：前端架构审计 + 权限体系对齐 + 工程化与版本治理规划

---

# 1. 总体结论

`go-admin-ui` 当前已经不是一个简单后台模板，现有基础包括：

- Vue 3 + TypeScript
- Vite
- Element Plus
- Vue Router
- Pinia
- Vue I18n
- Axios
- Vitest
- ESLint + Prettier
- Husky + lint-staged
- 动态菜单
- 动态路由
- 按钮权限
- Access Token + Refresh Token
- 静默刷新
- 并发刷新保护
- WebSocket 通知
- 文件中心
- 任务中心
- Mock
- CI
- 基础文档

当前整体方向是合理的。

后续最重要的不是继续堆页面，而是：

1. 和后端 Permission Code 模型统一；
2. 加强 Token 存储安全；
3. 明确前端权限只是 UX，后端才是最终安全边界；
4. 从页面/技术目录逐步向 Module First 演进；
5. 将手写 API 类型逐步升级为 OpenAPI 驱动；
6. 增加 E2E / Browser / Visual Test；
7. 完善依赖升级策略；
8. 正式建立 SemVer + Release + CHANGELOG；
9. 补齐 CONTRIBUTING / SECURITY / CODE_OF_CONDUCT / Issue Template；
10. 前后端版本建立兼容矩阵。

推荐定位：

> **A production-ready Vue 3 admin frontend for gin-layout, with dynamic access control, modular architecture and enterprise-oriented developer experience.**

---

# 2. 当前技术栈与版本现状

当前 `package.json`：

```text
Vue                  ^3.4.29
Vue Router           ^4.4.0
Pinia                ^2.1.7
Element Plus         ^2.7.6
Axios                ^1.7.2
Vue I18n             ^9.14.4

Vite                 ^5.4.21
Vitest               ^2.1.9
TypeScript           ^6.0.3
vue-tsc              ^3.2.6
ESLint               ^9.6.0
```

项目版本：

```text
0.9.2
```

来源：

- `package.json`
- <https://github.com/wannanbigpig/go-admin-ui/blob/master/package.json>

---

# 3. 当前版本治理存在明显不一致

## 3.1 package.json 是 0.9.2，但 GitHub 没有 Release

当前：

```json
"version": "0.9.2"
```

但是 GitHub Releases 当前为空。

来源：

- `package.json`
- <https://github.com/wannanbigpig/go-admin-ui/releases>

这会产生一个问题：

```text
package.json
    ↓
v0.9.2

Git Tag / Release
    ↓
不存在
```

用户无法判断：

- 0.9.2 到底是什么版本；
- 哪个 commit 是稳定版本；
- 应该 clone master 还是某个 tag；
- 如何升级；
- 哪个版本兼容哪个 gin-layout 后端。

## 建议

不要继续只修改 `package.json version`。

从下一版本开始正式建立：

```text
Git Tag
+
GitHub Release
+
CHANGELOG
+
package.json version
```

四者同步。

---

# 4. README 的 Node 版本和 CI 不一致

README 当前写：

```text
Node.js >= 18
npm >= 9
```

但 GitHub Actions 实际测试：

```text
Node 20
Node 22
```

来源：

- `README.md`
- `.github/workflows/node.js.yml`

链接：

- <https://github.com/wannanbigpig/go-admin-ui/blob/master/README.md>
- <https://github.com/wannanbigpig/go-admin-ui/blob/master/.github/workflows/node.js.yml>

这属于开源项目比较典型的版本声明问题。

## 建议

正式定义：

```text
Node >= 20
```

推荐增加：

```json
"engines": {
  "node": ">=20"
}
```

如果决定升级 Vite 8，应该根据 Vite 8 官方 Node 要求一起调整。

同时建议增加：

```text
.nvmrc
.node-version
```

至少保留一个。

例如：

```text
22
```

---

# 5. 当前依赖版本已经明显落后于主流大版本

截至 2026-09-14：

- Vite 8 已于 2026-03 发布，8.1 于 2026-06 发布；
- Vitest 5 已于 2026-09-03 发布；
- Pinia 已经进入 v3；
- Vue 3.5 已经是 Vue 3 的成熟版本线。

当前项目：

```text
Vite 5
Vitest 2
Pinia 2
Vue 3.4
```

因此已经存在多代差距。

参考来源：

- Vite 8：
  <https://vite.dev/blog/announcing-vite8>
- Vite 8.1：
  <https://vite.dev/blog/announcing-vite8-1>
- Vitest 5：
  <https://vitest.dev/blog/vitest-5>
- Pinia v2 → v3：
  <https://pinia.vuejs.org/cookbook/migration-v2-v3.html>
- Vue 3.5：
  <https://blog.vuejs.org/>

## 但不建议一次性全部升级

建议按：

```text
Vue / Router / Pinia
        ↓
Vite
        ↓
Vitest
        ↓
ESLint / Plugin
```

分批做。

推荐先建立：

```text
deps-upgrade
```

分支或 PR，每个 major 独立升级并完整跑：

```bash
npm run lint
npm run type-check
npm run test:run
npm run build:production
```

---

# 6. 是否要立即升级 Vite 8？

建议：

> **可以规划升级，但不要作为当前最高优先级。**

Vite 8 最大变化是统一采用 Rolldown，官方称构建性能显著提升。

来源：

- <https://vite.dev/blog/announcing-vite8>

但你的后台项目目前最大的价值不是构建性能，而是：

```text
权限
API Contract
模块化
测试
版本治理
```

因此升级顺序建议：

```text
P0 权限和版本规范
P1 API / Module Architecture
P1 测试
P2 Vite 8
```

---

# 7. 当前前端权限设计整体是正确的

当前路由流程：

```text
Login
  ↓
Token
  ↓
refreshUserInfo()
  ↓
User Info + Menu Tree
  ↓
convertRoute()
  ↓
router.addRoute()
```

来源：

- `src/router/guard.ts`
- `src/router/dynamicRoutes.ts`
- `src/stores/auth.ts`

链接：

- <https://github.com/wannanbigpig/go-admin-ui/blob/master/src/router/guard.ts>
- <https://github.com/wannanbigpig/go-admin-ui/blob/master/src/router/dynamicRoutes.ts>
- <https://github.com/wannanbigpig/go-admin-ui/blob/master/src/stores/auth.ts>

这个设计适合：

> 后端驱动复杂权限后台。

Vue Vben Admin 也明确支持：

- frontend access mode
- backend access mode
- mixed mode

其 backend mode 同样由后端返回菜单并使用 `router.addRoute` 动态生成路由。

参考：

- <https://github.com/vbenjs/vue-vben-admin/blob/main/docs/src/guide/in-depth/access.md>

因此你现在选择的后端动态菜单路线没有问题。

---

# 8. component_key 设计值得保留

当前后端菜单只保存：

```text
component_key
```

例如：

```text
system:config
```

前端：

```text
componentMap
     ↓
真实 Vue Component
```

而不是把：

```text
/src/views/system/config/index.vue
```

直接存在数据库。

来源：

- `src/router/dynamicRoutes.ts`

这是一个比较好的设计。

好处：

```text
数据库
不依赖前端真实目录
```

以后前端重构：

```text
views/system/config.vue
→
modules/system/config/page.vue
```

数据库无需修改。

建议继续强化：

```text
component_key
route_name
permission_code
```

作为前后端稳定契约。

---

# 9. 动态路由实现已经考虑不少异常情况

当前代码已经处理：

- button 节点不生成路由；
- route name 重复；
- route path 重复；
- component_key 缺失；
- 开发模式启动预校验；
- component 不存在降级 404；
- 动态 route 清理；
- 动态 route 重建；
- 第一个有效页面自动解析。

来源：

- `src/router/dynamicRoutes.ts`

这一块不建议推倒重写。

应该基于现有实现继续增加：

```text
Permission Code
Route Contract Validation
403 Page
Tenant Context
Feature Flag
```

---

# 10. 最大权限改进：按钮 code 应升级为统一 Permission Code

当前 `authStore` 已经支持：

```text
buttonPermissions
buttonPermissionMap
shouldShowButton(code)
```

来源：

- `src/stores/auth.ts`

这说明你其实已经有“权限码”的雏形。

建议把现在的：

```text
Button Code
```

正式提升为：

```text
Permission Code
```

与后端统一。

例如：

```text
order.view
order.create
order.update
order.refund

product.view
product.create
product.update
product.delete
```

前端：

```vue
<Permission code="order.refund">
  <el-button>退款</el-button>
</Permission>
```

或者：

```vue
<el-button v-permission="'order.refund'">
```

---

# 11. 权限码不应只是按钮权限

未来统一：

```text
Permission Code
     │
     ├── Menu
     ├── Route
     ├── Button
     ├── API
     └── Agent Tool
```

前端只消费：

```text
user.permissions: string[]
```

例如：

```json
[
  "order.view",
  "order.refund",
  "product.view"
]
```

这样：

```text
按钮权限
页面能力
后端 API
未来 Agent
```

全部统一。

参考 Vue Vben：

Vue Vben Admin 当前明确支持接口返回 access codes，例如：

```text
['AC_100100', 'AC_100110']
```

并通过统一 AccessControl 控制按钮。

来源：

- <https://github.com/vbenjs/vue-vben-admin/blob/main/docs/src/guide/in-depth/access.md>

建议你的项目吸收这个思路，但 Permission Code 使用更业务语义化的形式。

---

# 12. 前端权限必须明确：只控制 UX，不负责安全

README / docs 应明确写：

> 前端权限只能控制菜单、路由、按钮展示，不构成真正安全边界。所有实际资源访问必须由 gin-layout 后端重新鉴权。

即：

```text
Frontend Permission
       ↓
用户体验

Backend Permission
       ↓
真正安全
```

千万不要让贡献者形成：

```text
按钮隐藏了
=
用户不能操作
```

的错误理解。

---

# 13. Token 存储是当前比较值得改进的安全点

当前 `authStore`：

```text
access_token
expires_at
userInfo
menu
```

通过：

```text
pinia-plugin-persistedstate
```

持久化到：

```text
localStorage
```

来源：

- `src/stores/auth.ts`

同时 Refresh Token 已经通过 HttpOnly Cookie 使用。

这是：

```text
Refresh Token
  ↓
HttpOnly Cookie

Access Token
  ↓
localStorage
```

## 风险

Access Token 放 localStorage：

```text
XSS
 ↓
JavaScript
 ↓
localStorage
 ↓
Token 泄漏
```

虽然 Access Token 生命周期较短，但仍有风险。

## 建议

推荐改成：

```text
Refresh Token
    ↓
HttpOnly + Secure + SameSite Cookie

Access Token
    ↓
Memory
```

即 Access Token 不持久化。

页面刷新：

```text
APP Load
  ↓
POST /refresh
  ↓
Cookie 自动发送
  ↓
获得 Access Token
  ↓
保存在内存
```

优点：

```text
XSS 无法直接从 localStorage 读取 Token
```

这与后端目前双 Token 设计是匹配的。

---

# 14. 用户和菜单也不建议长期持久化

当前：

```text
userInfo
menu
```

也进入 localStorage。

问题：

后台权限发生变化后：

```text
Server
权限已经收回

Browser
localStorage 仍有旧菜单
```

虽然后端会阻止真正 API，但前端 UX 会暂时不一致。

建议：

```text
persist:
  theme
  locale
  UI preference
```

而：

```text
token
user
menu
permissions
```

优先由启动过程重新获取。

---

# 15. Token 刷新并发控制实现值得保留

当前已有：

```text
refreshingPromise
checkingTokenPromise
authSessionVersion
authStateVersion
```

用于防止：

- 多次用户信息请求；
- 多次 token 检查；
- 老 Session 响应覆盖新 Session；
- 旧刷新结果覆盖新 token。

来源：

- `src/stores/auth.ts`

这是比较成熟的实现思路。

后续只需要：

1. 将 Access Token 改为 Memory；
2. Request 层统一 refresh single-flight；
3. 补完整测试。

不建议完全重写。

---

# 16. 增加 403 页面

目前动态路由不存在会：

```text
404
```

但：

```text
资源存在
用户无权限
```

应该明确区分：

```text
403 Forbidden
404 Not Found
```

推荐：

```text
src/views/error/
├── 403.vue
├── 404.vue
└── 500.vue
```

Vue Vben 已经明确支持：

> 菜单可见但无访问权限时进入 403。

参考：

- <https://github.com/vbenjs/vue-vben-admin/blob/main/docs/src/guide/in-depth/access.md>

---

# 17. Route Contract Validation 建议升级

你已经有：

```text
validateRouteComponents()
```

建议进一步建立：

```ts
type BackendMenu = {
    code: string
    path: string
    name: string
    component_key?: ComponentKey
    permission?: PermissionCode
    type: MenuType
}
```

并使用运行时 Schema：

推荐：

```text
zod
```

例如：

```ts
const MenuSchema = z.object({
  code: z.string(),
  path: z.string(),
  type: z.number(),
  component_key: z.string().optional()
})
```

原因：

TypeScript 只能做编译期类型。

后端真实返回：

```json
{
  "type": "abc"
}
```

TypeScript 无法自动保护运行时。

对于后台动态路由这种核心输入，runtime validation 很值得做。

---

# 18. 目录结构当前仍偏“技术分层”

当前：

```text
src/
├── api
├── components
├── composables
├── directives
├── layout
├── locales
├── mock
├── modules
├── router
├── stores
├── types
├── utils
└── views
```

来源：

- `README.md`
- `src/`

这比纯 `views + api` 已经好一些，因为你已经开始引入 `modules/`。

建议继续往：

```text
Module First
```

走。

---

# 19. 推荐的前端模块结构

未来：

```text
src/
├── app/
│   ├── router/
│   ├── stores/
│   ├── layout/
│   └── bootstrap/
│
├── modules/
│   ├── auth/
│   │   ├── api/
│   │   ├── model/
│   │   ├── service/
│   │   ├── components/
│   │   └── pages/
│   │
│   ├── system/
│   │   ├── user/
│   │   ├── role/
│   │   ├── menu/
│   │   └── department/
│   │
│   ├── file/
│   ├── task/
│   └── notification/
│
├── shared/
│   ├── components/
│   ├── composables/
│   ├── utils/
│   ├── types/
│   └── constants/
│
└── platform/
    ├── http/
    ├── storage/
    ├── access/
    └── realtime/
```

原则：

```text
业务
↓
modules

跨业务复用
↓
shared

框架级能力
↓
platform
```

---

# 20. 不建议当前直接改 Monorepo

Vue Vben 当前是大型 Monorepo：

- pnpm workspace
- Turbo
- 多 Package

参考：

- <https://github.com/vbenjs/vue-vben-admin>

但 `go-admin-ui` 当前没有必要马上复制。

单前端：

```text
Vue + Element Plus
```

采用单仓库反而更简单。

只有未来出现：

```text
admin-web
mobile-admin
component-library
sdk
shared-config
```

多个真实 Package 时，再迁 Monorepo。

---

# 21. API 层是下一阶段核心

推荐从：

```text
手写 API
+
手写 Type
```

逐步升级：

```text
OpenAPI
    ↓
Generator
    ↓
API Client + Types
```

例如：

```text
gin-layout
     ↓
openapi.json
     ↓
openapi-typescript / orval
     ↓
go-admin-ui/src/generated
```

生成：

```text
API Type
Request Type
Response Type
Client
```

业务代码只做：

```text
Service
Transform
UI Logic
```

---

# 22. 为什么 API 自动生成很重要

前后端现在是两个仓库：

```text
gin-layout
go-admin-ui
```

最大的长期风险是：

```text
Backend 改字段
       ↓
Frontend Type 没改
       ↓
运行时问题
```

OpenAPI 能变成中间契约：

```text
Backend
  ↓
OpenAPI
  ↓
Frontend
```

同时未来：

```text
Agent
SDK
第三方 Client
```

也能共用。

---

# 23. 建议建立 API Compatibility Test

CI：

```text
Backend OpenAPI
      ↓
Generate Frontend Client
      ↓
git diff
      ↓
如果有未提交变化则失败
```

这样能强制：

```text
API Contract
=
Frontend Client
```

保持一致。

---

# 24. 测试体系当前不够完整

当前已有：

```text
Vitest
Vue Test Utils
jsdom
```

这是好基础。

但后台前端最容易出问题的是：

```text
Login
Token Refresh
Dynamic Route
Permission
File Upload
Task
```

建议测试分 4 层。

---

# 25. Layer 1：Unit Test

优先：

```text
permission
dynamicRoutes
redirect
request normalization
token lifecycle
i18n helpers
```

目标：

```text
核心 util 80%+
权限逻辑 90%+
```

---

# 26. Layer 2：Component Test

例如：

```text
Permission Component
Upload
Table
Form
Task Status
Notification Bell
```

Vitest 5 目前已经继续加强 Browser Mode；Vitest 4 时 Browser Mode 已正式稳定。

来源：

- <https://vitest.dev/blog/vitest-4>
- <https://vitest.dev/blog/vitest-5>

后续可以逐步从纯 jsdom 增加 Browser Test。

---

# 27. Layer 3：E2E

推荐：

```text
Playwright
```

必须覆盖：

```text
登录
刷新 Token
退出
动态菜单
按钮权限
403
文件上传
任务创建
通知
```

这是当前 CI 最值得补的一层。

---

# 28. Layer 4：Visual Regression

Vitest 4 已支持 Browser Mode Visual Regression。

也可以使用 Playwright Screenshot。

特别适合：

```text
Layout
Login
Dashboard
Common Table
Dialog
Theme
Dark Mode
```

---

# 29. 当前 CI 基础不错

GitHub Actions 已经：

```text
Node 20
Node 22

npm ci
npm run lint
npm run type-check
npm run test:run
npm run build:production
```

来源：

- `.github/workflows/node.js.yml`

这是一个比较好的基础。

---

# 30. CI 建议进一步增加

推荐最终：

```text
install
 ↓
lint
 ↓
typecheck
 ↓
unit
 ↓
coverage
 ↓
build
 ↓
e2e
 ↓
dependency audit
```

增加：

```text
npm audit --audit-level=high
```

或者更稳定地使用：

```text
Dependabot
GitHub Dependency Review
```

同时：

```text
CodeQL JavaScript/TypeScript
```

也值得加入。

---

# 31. 包管理器需要统一

当前 CI：

```text
npm
```

README：

```text
npm
```

这是统一的。

不需要因为 Vben 用 pnpm 就强行换 pnpm。

但是建议加：

```text
packageManager
```

例如：

```json
"packageManager": "npm@11.x"
```

配合 Corepack/CI 形成确定性。

如果决定切换 pnpm，必须作为一个明确版本发布，而不是悄悄更换。

---

# 32. Lockfile Policy

开源应用仓库必须提交：

```text
package-lock.json
```

并要求 CI 使用：

```bash
npm ci
```

你已经这么做。

建议 CONTRIBUTING 明确：

> 修改依赖必须提交 package.json + package-lock.json。

---

# 33. 环境变量治理

当前已有很多：

```text
VITE_APP_TITLE
VITE_ENABLE_I18N
VITE_BASE_API
VITE_USE_PROXY
...
```

来源：

- README

建议增加：

```ts
interface ImportMetaEnv {
  readonly VITE_APP_TITLE: string
  readonly VITE_BASE_API: string
}
```

并在启动时做 Schema Validation。

例如：

```text
env.ts
```

负责：

```text
读取
验证
默认值
错误提示
```

不要在业务代码到处：

```ts
import.meta.env.xxx
```

---

# 34. Feature Flags

未来如果前后端进入 SaaS：

```text
Task Center
File Center
Notification
Agent
```

不同版本可能启用不同模块。

建议引入：

```text
Feature Flags
```

例如：

```ts
features.has('task')
features.has('agent')
```

数据来源可以是：

```text
Backend Capabilities API
```

而不是构建时写死。

---

# 35. Tenant Switcher

如果后端 v0.6 加 Multi-Tenant，前端需要同步：

```text
Current Tenant
Tenant Switcher
Tenant Context
```

建议：

```text
X-Tenant-ID
```

或后端约定的 Tenant Context 由 Request Client 统一注入。

不要每个 API 自己传：

```ts
tenant_id
```

前端结构：

```text
Auth
 │
Tenant
 │
Permission
 │
API
```

---

# 36. Request 层建议成为真正 Platform Service

当前：

```text
src/utils/request.ts
```

未来建议迁为：

```text
src/platform/http/
```

拆：

```text
client.ts
interceptors.ts
refresh.ts
errors.ts
types.ts
```

避免一个 `request.ts` 越来越大。

---

# 37. 错误模型前后端统一

后端应该返回：

```json
{
  "code": "AUTH_PERMISSION_DENIED",
  "message": "...",
  "trace_id": "..."
}
```

前端：

```text
Error Code
   ↓
i18n
   ↓
UI
```

不要过度依赖：

```text
后端 message
```

作为 UI 文案。

这样：

```text
中文
英文
未来日文
```

能保持一致。

---

# 38. Trace ID 前端展示

如果后端按前面的建议增加：

```text
trace_id
```

前端错误页面/弹窗最好提供：

```text
错误编号：abc123
```

用户反馈问题时：

```text
提供 trace_id
```

后端可直接定位日志。

---

# 39. CSP 与前端安全

建议生产部署增加：

```text
Content-Security-Policy
X-Frame-Options
X-Content-Type-Options
Referrer-Policy
Permissions-Policy
```

尤其后台系统使用：

```text
iframe
```

需要明确：

```text
VITE_IFRAME_ALLOWED_ORIGINS
```

不能只是前端校验。

服务器 CSP：

```text
frame-src
```

也应该同步。

---

# 40. 不要把敏感配置放 VITE_

任何：

```text
VITE_*
```

都会进入前端构建产物。

必须在文档明确：

> VITE 环境变量全部视为公开配置，不允许存储 Secret、AccessKey、数据库密码等。

---

# 41. 开源规范目前明显不足

当前 `.github` 只有：

```text
workflows/
```

没有看到：

```text
PR template
Issue template
CONTRIBUTING
SECURITY
CODE_OF_CONDUCT
```

来源：

- <https://github.com/wannanbigpig/go-admin-ui/tree/master/.github>

因此前端当前的：

```text
代码成熟度
>
开源治理成熟度
```

---

# 42. 前后端开源规范必须统一

强烈建议两个仓库使用同一套：

```text
CONTRIBUTING.md
SECURITY.md
CODE_OF_CONDUCT.md
CHANGELOG.md
ROADMAP.md
.github/ISSUE_TEMPLATE
.github/pull_request_template.md
```

并统一：

```text
Commit Convention
Branch Convention
Release Convention
Security Reporting
```

否则社区参与两个仓库会像参与两个完全不同的项目。

---

# 43. 推荐 Commit Convention

统一 Conventional Commits：

```text
feat(auth): use in-memory access token

feat(access): add permission component

fix(router): prevent duplicate dynamic routes

fix(request): avoid duplicate refresh requests

refactor(api): migrate user api to generated client

test(auth): cover token refresh

docs: update node requirements

chore(deps): upgrade pinia
```

---

# 44. vue-pure-admin 的社区规范值得参考

vue-pure-admin README 中明确提供：

```text
Fork
Create feat/xxxx branch
Commit
Push
PR
```

同时定义：

```text
feat
fix
style
perf
refactor
revert
test
docs
chore
workflow
ci
types
```

来源：

- <https://github.com/xiaoxian521/vue-pure-admin>

这种规范非常适合你前后端统一。

---

# 45. Release 版本建议

前端当前 `package.json` 已经到了：

```text
0.9.2
```

但没有 Release。

建议不要直接把它当正式历史版本。

可以从一次“版本治理重启”开始。

推荐：

```text
v0.10.0
```

作为首个正式 GitHub Release。

为什么不是 v1.0：

因为还有：

```text
Permission Model
Token Storage
API Contract
Module Architecture
Dependency Upgrade
Open-source Governance
```

几个较大的变化。

---

# 46. 推荐前端版本路线

## v0.10 — Governance & Security

完成：

```text
正式 Release
CHANGELOG
CONTRIBUTING
SECURITY
Issue Templates

Node Version 对齐
engines
packageManager

Access Token Memory-only
403 Page
Permission Code
```

对应后端：

```text
gin-layout v0.5 Access Foundation
```

---

## v0.11 — API Contract

完成：

```text
OpenAPI Client
Generated Types
Runtime Schema Validation
Unified Error Model
Trace ID
```

---

## v0.12 — Module Architecture

完成：

```text
app/
modules/
shared/
platform/

system module
file module
task module
notification module
```

以渐进方式迁移。

---

## v0.13 — Multi-Tenant UI

对应后端 Tenant：

```text
Tenant Context
Tenant Switcher
Tenant Capability
Tenant Permission
```

---

## v0.14 — Testing & Quality

完成：

```text
Vitest Upgrade
E2E Playwright
Browser Test
Visual Regression
Coverage Gate
```

---

## v0.15 — Toolchain Upgrade

完成：

```text
Vue 3.5+
Pinia 3
Vite 8
Vitest 5
```

具体顺序按兼容性拆 PR。

---

## v0.16 — Developer Experience

完成：

```text
Page Generator
Module Generator
API Generator
Permission Generator
Docs Site
Example Module
```

---

## v1.0

v1.0 的含义：

```text
Stable Permission Contract
Stable Menu Contract
Stable API Contract
Stable Module Layout
Stable Environment Variables
Stable Release Process
Stable gin-layout Compatibility
```

---

# 47. 前后端版本兼容矩阵

建议 README 明确：

| go-admin-ui | gin-layout | Status |
|---|---|---|
| 0.10.x | 0.5.x | Supported |
| 0.11.x | 0.5.x / 0.6.x | Supported |
| 0.13.x | 0.6.x | Supported |
| 1.x | 1.x | Stable |

不要简单要求：

```text
前端 v0.10
必须等于
后端 v0.10
```

因为两个项目迭代节奏不同。

应该维护：

```text
Compatibility Matrix
```

---

# 48. 前后端 Contract Version

甚至可以 API 返回：

```json
{
  "api_version": "1",
  "capabilities": [
    "permission-code",
    "task-center",
    "tenant"
  ]
}
```

前端启动时检查：

```text
Backend Capability
       ↓
Frontend Feature
```

如果不兼容，明确提示：

```text
当前 go-admin-ui 需要 gin-layout >= x.x
```

避免出现奇怪运行时错误。

---

# 49. GitHub Release 内容模板

每个版本建议写：

```markdown
## Highlights

- Permission Code
- Memory-only access token

## Added

...

## Changed

...

## Fixed

...

## Security

...

## Breaking Changes

...

## Backend Compatibility

- gin-layout >= v0.5.0

## Upgrade Guide

...
```

---

# 50. CHANGELOG

推荐：

```text
Keep a Changelog
+
Semantic Versioning
```

例如：

```markdown
## [0.10.0] - 2026-XX-XX

### Added
- Permission component
- 403 page

### Changed
- Access token is no longer persisted

### Security
- Removed access token from localStorage

### Breaking
- Permission code replaces button-only code
```

---

# 51. PR Template

建议：

```markdown
## What

## Why

## Changes

## Screenshots

## Testing

- [ ] lint
- [ ] type-check
- [ ] unit tests
- [ ] build

## Breaking Change

## Backend Compatibility
```

前端 PR 有一个后端没有的特殊要求：

```text
Screenshots / Video
```

UI 改动必须附图。

---

# 52. Issue Template

至少：

```text
Bug Report
Feature Request
UI/UX Proposal
Compatibility Issue
```

Bug 要求：

```text
go-admin-ui version
gin-layout version
Node version
Browser
Steps
Expected
Actual
Screenshot
Console Error
```

这对前后端联调项目尤其重要。

---

# 53. Dependabot / Renovate

强烈建议配置其中一个。

推荐：

```text
Renovate
```

因为前端依赖数量多，适合：

```text
group minor
group devDependencies
schedule weekly
```

避免：

```text
半年不升级
 ↓
一次跨 4 个 major
```

现在 Vite 5 / Vitest 2 的状态就是典型例子。

---

# 54. 依赖升级策略

建议：

```text
Patch
自动

Minor
每周/双周

Major
人工单独 PR
```

Major PR 必须：

```text
Migration Guide
CI
E2E
Build
Manual Smoke Test
```

---

# 55. Browser Support

README 需要明确：

```text
Chrome latest 2
Edge latest 2
Firefox latest 2
Safari latest 2
```

或者通过：

```text
browserslist
```

明确构建目标。

不要隐式依赖开发者自己的浏览器。

---

# 56. Accessibility

成熟后台经常忽视 A11y。

建议至少：

```text
Keyboard Navigation
Dialog Focus
aria-label
Form Error Association
Color Contrast
Reduced Motion
```

后续可在 CI 增加：

```text
axe
```

检查核心页面。

---

# 57. UI Design Token

当前 Element Plus 主题后续建议统一：

```text
Design Tokens
```

例如：

```text
color
spacing
radius
font
z-index
layout size
```

业务页面不要自己到处写：

```css
margin: 13px;
color: #409eff;
```

形成：

```text
theme tokens
```

这样以后 Dark Mode / Brand Theme 更稳定。

---

# 58. 前端 Generator

后端已经有 CRUD Generator。

长期建议前后端打通：

```text
Backend Module Spec
        ↓
Generator
   ┌────┴────┐
 Backend   Frontend
```

例如：

```yaml
module: product

fields:
  name:
    type: string
    searchable: true

  status:
    type: enum
```

自动生成：

```text
Backend
model/repo/service/handler

Frontend
api/page/form/table/permission
```

这是未来整个项目非常强的竞争力。

---

# 59. 对第三方项目的总体取舍

## Vue Vben Admin

值得参考：

- Access Mode
- Permission Code
- Monorepo 模块边界
- Docs
- 通用组件

不建议当前照搬：

- 大型 Monorepo 复杂度。

来源：

- <https://github.com/vbenjs/vue-vben-admin>
- <https://github.com/vbenjs/vue-vben-admin/blob/main/docs/src/guide/in-depth/access.md>

---

## vue-pure-admin

值得参考：

- Element Plus 生态
- Git Commit 规范
- CHANGELOG
- Contribution Guide
- Admin UX

来源：

- <https://github.com/xiaoxian521/vue-pure-admin>

---

## Gin-Vue-Admin Web

值得参考：

- 前后端统一 Generator
- Swagger
- 动态权限
- 文档 / Demo / 社区

来源：

- <https://github.com/flipped-aurora/gin-vue-admin>

---

# 60. 最终推荐架构

```text
                     go-admin-ui

┌──────────────────────────────────────────────┐
│                    app                       │
│                                              │
│ bootstrap │ router │ layout │ root stores    │
└──────────────────────┬───────────────────────┘
                       │
┌──────────────────────▼───────────────────────┐
│                  platform                    │
│                                              │
│ http │ access │ realtime │ storage │ config  │
└──────────────────────┬───────────────────────┘
                       │
┌──────────────────────▼───────────────────────┐
│                   modules                    │
│                                              │
│ auth │ system │ file │ task │ notification   │
└──────────────────────┬───────────────────────┘
                       │
┌──────────────────────▼───────────────────────┐
│                   shared                     │
│                                              │
│ components │ composables │ types │ utils     │
└──────────────────────────────────────────────┘
```

权限：

```text
Backend
   │
   ▼
Menu + Permission Codes
   │
   ▼
Auth Store
   │
   ├── Dynamic Route
   │
   ├── Menu
   │
   └── Access Control
```

认证：

```text
HttpOnly Refresh Cookie
        │
        ▼
Refresh Endpoint
        │
        ▼
Access Token
        │
        ▼
Memory Only
```

---

# 61. 优先级

| Priority | 项目 |
|---|---|
| P0 | 正式 Release / CHANGELOG |
| P0 | Node / engines / CI 版本统一 |
| P0 | Access Token 移出 localStorage |
| P0 | Permission Code 与后端统一 |
| P0 | 403 / Permission Component |
| P1 | OpenAPI Generated Client |
| P1 | Runtime API Schema Validation |
| P1 | Trace ID / Unified Error |
| P1 | E2E Playwright |
| P1 | CONTRIBUTING / SECURITY |
| P1 | 前后端 Compatibility Matrix |
| P2 | Module First |
| P2 | Pinia 3 / Vue 3.5 |
| P2 | Vite 8 / Vitest 5 |
| P2 | Visual Regression |
| P2 | Renovate |
| P2 | Docs Site |
| P3 | Monorepo |
| P3 | Independent Component Package |

---

# 62. 建议近期第一批 Issue

可以直接在 GitHub 开：

```text
feat(access): introduce unified permission codes

security(auth): keep access token in memory only

feat(router): add 403 forbidden page

refactor(http): split request client and refresh manager

feat(api): generate TypeScript client from OpenAPI

test(e2e): add Playwright login and permission flow

chore(node): define engines and node version

chore(release): establish semantic versioning

docs: add frontend/backend compatibility matrix

chore(repo): add contributing and security policies

chore(deps): migrate Pinia 2 to Pinia 3

chore(deps): evaluate Vite 8 migration
```

这些 Issue 本身也非常适合直接形成 Roadmap。

---

# 63. 最终建议

`go-admin-ui` 不需要追求：

> 功能最多的 Vue Admin。

更好的路线是和 `gin-layout` 形成一个完整产品组合：

```text
gin-layout
=
Backend Foundation

go-admin-ui
=
Frontend Foundation
```

二者共同提供：

```text
权限契约
API 契约
任务体系
文件体系
通知体系
Tenant
Generator
Release Contract
```

这样项目会从：

> “一个 Go 后台 + 一个 Vue 后台模板”

变成：

> **一套真正有架构约束、有版本契约、可用于长期业务开发的开源中后台 Foundation。**
