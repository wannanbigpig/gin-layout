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

## Architecture Evolution（架构演进规划）

- **限流引擎配置化双驱动**：
  - 支持 `app.rate_limit_driver` 配置驱动切换：`memory`（单实例默认，零外部依赖，极高性能）与 `redis`（多实例分布式，集群统一核算配额）。
  - 集成 Redis 故障或网络抖动时自动降级为本地内存令牌桶的高可用熔断机制（Fallback）。
  - 接入 `config/runtime.go` 配置热更新监听，支持在线无缝平滑切换驱动。
- **敏感字段通用透明脱敏门禁**：
  - 复用并提升现有系统参数「请求日志脱敏」中的「通用字段」（`audit.sensitive_fields.common`）为全系统敏感数据规则库。
  - 在 `internal/pkg/auditdiff` 底层构建切面门禁，在生成 `change_diff` 业务实体变更快照时，根据通用敏感规则自动执行掩码脱敏，业务层零侵入、完全透明生效。
- **导出中心方法属主防御与越权治理**：
  - 收敛底层无过滤的 `ExportCenterService.List` 为包内私有方法，对外仅暴露编译期强绑定 `CurrentUser.UID` 的 `ListMyExports`，彻底杜绝越权隐患（BOLA）。
  - 超管与安全审计视角的全局导出记录查询未来若开放，独立命名为 `ListAllExportsForAudit`，显式挂载独立 Casbin 权限与审计追踪。
- **`sys_config` 复杂配置解耦与独立实体表治理**：
  - 分类治理：基础开关、系统阈值、脱敏字段名单等纯标量/名单数据继续保留在 `sys_config`。
  - 云厂商凭证与复杂驱动独立建表：拆分文件存储驱动表 `sys_storage_config`（存储驱动独立记录、密钥统一应用层 AES 密文存储）与短信渠道/模板表 `sys_sms_channel` + `sys_sms_template`（支持多业务场景模板映射与故障容灾）。

## Optional

多租户、ABAC、OpenTelemetry、Agent、微服务等只作为独立扩展点评估，不进入默认核心，也不引入 Permission 表替代现有 `Menu.Code` 和菜单/API 关系。
