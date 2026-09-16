# Changelog

本项目遵循 [Keep a Changelog](https://keepachangelog.com/) 和 SemVer。

## [Unreleased]

### Changed

- 权限变更与 Casbin 策略刷新采用 fail-closed，并收紧数据范围资源访问。
- Generator 输出适配当前后端、前端和菜单表结构。
- 列表查询错误不再伪装为空数据成功响应。
- 注册 `command gen-openapi`，统一从路由 Catalog 生成接口文档。

### Security

- 自定义数据范围拒绝空部门和无效部门。
- 菜单/API 权限同步与业务事务保持一致。

## [0.9.2]

- 当前开发基线。
