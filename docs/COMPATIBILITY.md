# 前后端兼容与版本策略

## SemVer

- `0.x` 阶段：破坏性接口调整提升 minor，兼容修复提升 patch。
- `1.x` 阶段：遵循标准 SemVer；破坏性调整提升 major。
- Git Tag 使用 `vMAJOR.MINOR.PATCH`，Tag 对应 GitHub Release；Release Notes 从 `CHANGELOG.md` 的对应版本整理。
- 提交使用 Conventional Commits。`feat!:` 或 `BREAKING CHANGE:` 必须同时说明迁移方式和前端影响。

## 兼容矩阵

| gin-layout | x-l-admin-vue3 | 支持状态 | 说明 |
| --- | --- | --- | --- |
| `0.9.x` | `0.9.x` | 当前支持 | JWT refresh cookie、统一响应、菜单动态路由和按钮 `Menu.Code` 契约一致 |

同一 minor 内 patch 应保持接口兼容。后端新增可选字段可以独立 patch 发布；删除/重命名字段、改变认证模式、菜单结构或错误码语义时，前后端必须提升到同一 minor，并在 Release Notes 中写迁移说明。

当前 OpenAPI 能可靠生成路由、HTTP 方法、分组和认证策略，但没有完整请求/业务响应 schema。现阶段适合做文档和路由漂移检查，不建议直接生成完整 TypeScript Client；补齐 schema 元数据后优先生成 Types，再决定是否生成 Client。
