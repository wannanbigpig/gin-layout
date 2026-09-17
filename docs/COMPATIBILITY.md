# 前后端兼容与版本策略

## SemVer

- `0.x` 阶段：破坏性接口调整提升 minor，兼容修复提升 patch。
- `1.x` 阶段：遵循标准 SemVer；破坏性调整提升 major。
- Git Tag 使用 `vMAJOR.MINOR.PATCH`，Tag 对应 GitHub Release；发布流水线会校验 Tag、`internal/global/common.go` 的源码版本和 `CHANGELOG.md` 条目一致，Release Notes 从对应版本整理。
- 提交使用 Conventional Commits。`feat!:` 或 `BREAKING CHANGE:` 必须同时说明迁移方式和前端影响。

## 兼容矩阵

| gin-layout | x-l-admin-vue3 | 支持状态 | 说明 |
| --- | --- | --- | --- |
| `0.10.x` | `0.10.x` | 当前支持 | 个人工作台/请求日志、全局指标权限、JWT refresh cookie、动态菜单和按钮 `Menu.Code` 契约一致 |
| `0.9.x` | `0.9.x` | 开发基线 | 首版公开发布前的开发基线，不承诺与 `0.10.x` 跨 minor 混用 |

同一 minor 内 patch 应保持接口兼容。后端新增可选字段可以独立 patch 发布；删除/重命名字段、改变认证模式、菜单结构或错误码语义时，前后端必须提升到同一 minor，并在 Release Notes 中写迁移说明。

`v0.10.0` 是首个公开版本候选。候选阶段部署会重置系统数据，因此本轮直接收敛了初始化迁移，不提供从未发布 `0.9.x` 开发数据库原地升级的兼容承诺；正式 `v0.10.0` tag 后，已发布迁移不可修改，后续版本必须新增成对迁移并记录升级、回滚与数据转换限制。

本轮新增登录级个人工作台和个人请求日志接口，同时保留工作台全局指标与管理员日志的 RBAC 权限。`0.10.x` 前端依赖这些接口和权限语义，不能与 `0.9.x` 后端组成受支持部署。

当前 OpenAPI 能可靠生成路由、HTTP 方法、分组和认证策略，但没有完整请求/业务响应 schema。现阶段适合做文档和路由漂移检查，不建议直接生成完整 TypeScript Client；补齐 schema 元数据后优先生成 Types，再决定是否生成 Client。
