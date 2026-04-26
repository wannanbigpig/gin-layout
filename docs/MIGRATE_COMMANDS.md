# 迁移命令详细说明

本文档说明项目内置的 `command migrate` 命令组，包括：

- 为什么统一使用项目命令
- 迁移文件命名规范
- `create/check/up/down/goto/force/version` 的具体用法
- 常见执行建议与注意事项

命令入口注册于 [cmd/command/command.go](/Users/liuml/data/go/src/go-layout/cmd/command/command.go)，具体实现位于 [internal/console/migrate/migrate.go](/Users/liuml/data/go/src/go-layout/internal/console/migrate/migrate.go)。

## 设计原则

项目已经内置迁移管理能力，推荐统一使用：

```bash
go run main.go -c ./config.yaml command migrate ...
```

这样做的原因：

- 不依赖仓库根目录额外放一个 `migrate` 二进制
- 不依赖 `scripts/` 里的 shell 包装脚本
- 与当前项目配置加载、迁移目录解析逻辑保持一致
- `up/down/goto/force/version` 与 `create/check` 入口统一

## 文件命名规范

默认使用时间戳版本：

```text
YYYYMMDDHHMMSS_desc.up.sql
YYYYMMDDHHMMSS_desc.down.sql
```

示例：

```text
20260425143015_add_task_center_tables.up.sql
20260425143015_add_task_center_tables.down.sql
```

约束：

- `version` 必须是递增且唯一的整数
- `desc` 会被规范化为小写下划线风格
- 每个版本必须恰好存在一对 `up/down`

默认时间格式为 `20060102150405`，默认时区为 `UTC`。

## 命令总览

```bash
go run main.go command migrate
go run main.go -c ./config.yaml command migrate create <name>
go run main.go -c ./config.yaml command migrate check
go run main.go -c ./config.yaml command migrate up [N]
go run main.go -c ./config.yaml command migrate down [N]
go run main.go -c ./config.yaml command migrate down --all
go run main.go -c ./config.yaml command migrate goto <version>
go run main.go -c ./config.yaml command migrate force <version>
go run main.go -c ./config.yaml command migrate version
```

说明：

- `go run main.go command migrate` 默认等价于 `go run main.go command migrate up`

公共参数：

- `--path`, `-p`
  - 指定迁移目录
  - 默认自动解析 `data/migrations`
- `--yes`, `-y`
  - 跳过确认提示
  - 主要用于 `down --all` 这类破坏性操作

## create

创建一对迁移文件：

```bash
go run main.go -c ./config.yaml command migrate create add_task_center_tables
```

默认行为：

- 自动将名称规范化为 `add_task_center_tables`
- 生成：
  - `data/migrations/<version>_add_task_center_tables.up.sql`
  - `data/migrations/<version>_add_task_center_tables.down.sql`

默认模板内容：

```sql
BEGIN;

-- TODO: write migration up SQL.

COMMIT;
```

可选参数：

- `--seq`
  - 改为顺序号模式
- `--digits`
  - 顺序号位数，默认 `6`
- `--format`
  - 时间戳格式，默认 `20060102150405`
- `--tz`
  - 时间戳时区，默认 `UTC`
- `--ext`
  - 文件扩展名，默认 `sql`

示例：

```bash
go run main.go -c ./config.yaml command migrate create add_menu_seed --format 20060102150405 --tz Asia/Shanghai
go run main.go -c ./config.yaml command migrate create backfill_demo_data --seq --digits 6
```

建议：

- 并行开发默认不要使用 `--seq`
- 未发布阶段如需调整同一功能下的迁移，可直接改当前未提交迁移文件，不必为了开发中微调新增多份迁移

## check

校验迁移目录中的文件格式和版本配对关系：

```bash
go run main.go -c ./config.yaml command migrate check
```

校验内容：

- 文件名是否匹配 `^(\d+)_(.+)\.(up|down)\.([^.]+)$`
- 每个 `version` 是否恰好一份 `up` 和一份 `down`
- 输出首个版本、末尾版本、总文件数

可选参数：

- `--strict`
  - 默认为 `true`
  - 若关闭，则遇到不匹配模式的文件时可放宽处理

## up

执行迁移：

```bash
go run main.go -c ./config.yaml command migrate up
```

执行指定数量：

```bash
go run main.go -c ./config.yaml command migrate up 1
```

说明：

- 不传 `N` 时执行全部未应用迁移
- 依赖当前 `config.yaml` 中可用的数据库配置

## down

回滚迁移：

```bash
go run main.go -c ./config.yaml command migrate down 1
```

全部回滚：

```bash
go run main.go -c ./config.yaml command migrate down --all -y
```

说明：

- 不传参数时默认回滚 1 个版本
- `--all` 会回滚到空版本，建议显式带 `-y`

## goto

迁移到指定版本：

```bash
go run main.go -c ./config.yaml command migrate goto 20260425143015
```

适合场景：

- 本地调试某个中间版本
- 需要回到指定版本再继续验证

## force

强制设置数据库迁移版本，不实际执行 SQL：

```bash
go run main.go -c ./config.yaml command migrate force 20260425143015
```

用途：

- 手动修复 dirty migration 状态
- 在已确认数据库状态正确时修正版本游标

注意：

- 这是修复命令，不是正常迁移流程常规入口
- 使用前应先确认数据库真实结构与迁移状态一致

## version

查看当前数据库迁移版本：

```bash
go run main.go -c ./config.yaml command migrate version
```

输出示例：

```text
version: 20260425143015, dirty: false
```

若数据库尚未执行任何迁移，会输出：

```text
version: none, dirty: false
```

## 指定迁移目录

默认会自动解析 `data/migrations`。若需要指定其他目录：

```bash
go run main.go -c ./config.yaml command migrate --path ./data/migrations check
go run main.go -c ./config.yaml command migrate --path ./data/migrations up
```

## 推荐工作流

### 新增迁移

```bash
go run main.go -c ./config.yaml command migrate create add_sys_notice_tables
go run main.go -c ./config.yaml command migrate check
go run main.go -c ./config.yaml command migrate up
```

### 调整未发布功能下的迁移

开发阶段同一功能若尚未提交发布：

- 直接修改当前功能对应的迁移文件
- 不为一次功能内的多次微调新增新的迁移文件
- 合并前确保 `check` 通过

### 排查 dirty 状态

```bash
go run main.go -c ./config.yaml command migrate version
go run main.go -c ./config.yaml command migrate force <version>
```

## 注意事项

- 使用 `go run` 时建议显式带 `-c ./config.yaml`
- 并行开发默认使用时间戳版本，避免多个分支都生成 `000004_*`
- `force` 只能在你明确知道数据库真实状态时使用
- `down --all` 具有破坏性，不要在不明确的环境执行

## 参考

- [README.md](/Users/liuml/data/go/src/go-layout/README.md)
- [docs/COMMANDS_AND_TASKS.md](/Users/liuml/data/go/src/go-layout/docs/COMMANDS_AND_TASKS.md)
- [golang-migrate 官方文档](https://github.com/golang-migrate/migrate/blob/master/cmd/migrate/README.md)
