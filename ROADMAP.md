# Roadmap

路线图只围绕 Gin 后台管理脚手架，优先级以稳定性、契约完整性和二次开发成本为准。

## Core

- 完善路由 Catalog 的请求参数与响应 schema 元数据，再评估生成 TypeScript Types。
- 保持权限同步、DataScope、缓存失效和多角色合并的回归测试。
- 持续完善 Generator、Migration、CLI、Task、Queue、Cron、File、Notification、日志和 CI。

## Enhancement

- 在现有前端 Mock 登录与动态路由 smoke E2E 基础上，增加真实后端联调 E2E。
- 为普通 Cron 提供可选的跨实例防重策略，同时保留单实例简单部署。
- 为多 `service` 实例增加 Casbin 策略变更广播；在此之前把单实例作为默认部署边界。
- 增加可重复的 Release Notes 与兼容性校验。

## Optional

多租户、ABAC、OpenTelemetry、Agent、微服务等只作为独立扩展点评估，不进入默认核心，也不引入 Permission 表替代现有 `Menu.Code` 和菜单/API 关系。
