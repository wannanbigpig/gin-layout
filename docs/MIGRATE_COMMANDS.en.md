# Migration Command Guide

This document explains the built-in `command migrate` command group, including:

- why the project uses its own command entry
- migration filename rules
- concrete usage for `create/check/up/down/goto/force/version`
- practical execution notes

The command is registered in [cmd/command/command.go](/Users/liuml/data/go/src/go-layout/cmd/command/command.go), and implemented in [internal/console/migrate/migrate.go](/Users/liuml/data/go/src/go-layout/internal/console/migrate/migrate.go).

## Why Use The Project Command

Use:

```bash
go run main.go -c ./config.yaml command migrate ...
```

This keeps migration operations:

- independent from an extra root-level `migrate` binary
- independent from shell wrapper scripts in `scripts/`
- aligned with the project's own config loading and migration path resolution
- unified across `create/check/up/down/goto/force/version`

## Filename Convention

Default timestamp-based filenames:

```text
YYYYMMDDHHMMSS_desc.up.sql
YYYYMMDDHHMMSS_desc.down.sql
```

Example:

```text
20260425143015_add_task_center_tables.up.sql
20260425143015_add_task_center_tables.down.sql
```

Rules:

- `version` must be a unique increasing integer
- `desc` is normalized into lower_snake_case
- every version must have exactly one `up` and one `down`

Default time format: `20060102150405`  
Default timezone: `UTC`

## Command Overview

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

Notes:

- `go run main.go command migrate` defaults to `go run main.go command migrate up`

Shared flags:

- `--path`, `-p`
  - migration directory path
  - defaults to auto-resolved `data/migrations`
- `--yes`, `-y`
  - skip confirmation prompts
  - mainly for destructive actions such as `down --all`

## create

Create a migration pair:

```bash
go run main.go -c ./config.yaml command migrate create add_task_center_tables
```

Default behavior:

- normalizes the name to `add_task_center_tables`
- creates:
  - `data/migrations/<version>_add_task_center_tables.up.sql`
  - `data/migrations/<version>_add_task_center_tables.down.sql`

Default template:

```sql
BEGIN;

-- TODO: write migration up SQL.

COMMIT;
```

Optional flags:

- `--seq`
  - switch to sequential numbering
- `--digits`
  - digit width for sequential numbering, default `6`
- `--format`
  - timestamp format, default `20060102150405`
- `--tz`
  - timestamp timezone, default `UTC`
- `--ext`
  - file extension, default `sql`

Examples:

```bash
go run main.go -c ./config.yaml command migrate create add_menu_seed --format 20060102150405 --tz Asia/Shanghai
go run main.go -c ./config.yaml command migrate create backfill_demo_data --seq --digits 6
```

Recommendation:

- avoid `--seq` for parallel development
- when a feature is still under initial development and not released yet, update the existing migration files directly instead of creating extra migrations for small iterative changes

## check

Validate migration filenames and version pairing:

```bash
go run main.go -c ./config.yaml command migrate check
```

What it validates:

- filename pattern `^(\d+)_(.+)\.(up|down)\.([^.]+)$`
- exactly one `up` and one `down` per version
- first version, last version, and file counts

Optional flag:

- `--strict`
  - default `true`
  - if disabled, non-matching files can be tolerated more loosely

## up

Apply migrations:

```bash
go run main.go -c ./config.yaml command migrate up
```

Apply a specific count:

```bash
go run main.go -c ./config.yaml command migrate up 1
```

Notes:

- without `N`, all pending migrations are applied
- requires a valid database config in `config.yaml`

## down

Rollback migrations:

```bash
go run main.go -c ./config.yaml command migrate down 1
```

Rollback all:

```bash
go run main.go -c ./config.yaml command migrate down --all -y
```

Notes:

- without an argument, it rolls back one version
- `--all` rolls back to nil version and should usually be paired with `-y`

## goto

Migrate to a specific version:

```bash
go run main.go -c ./config.yaml command migrate goto 20260425143015
```

Useful for:

- local debugging against an intermediate version
- moving back to a known version before another verification step

## force

Force-set the migration version without running SQL:

```bash
go run main.go -c ./config.yaml command migrate force 20260425143015
```

Use cases:

- repairing a dirty migration state
- correcting the version cursor when the actual database state is already known to be correct

Notes:

- this is a repair command, not a normal migration workflow step
- verify the real database schema before using it

## version

Show the current migration version:

```bash
go run main.go -c ./config.yaml command migrate version
```

Example output:

```text
version: 20260425143015, dirty: false
```

If no migration has been applied yet:

```text
version: none, dirty: false
```

## Custom Migration Path

By default the command auto-resolves `data/migrations`. To override it:

```bash
go run main.go -c ./config.yaml command migrate --path ./data/migrations check
go run main.go -c ./config.yaml command migrate --path ./data/migrations up
```

## Recommended Workflow

### Add a new migration

```bash
go run main.go -c ./config.yaml command migrate create add_sys_notice_tables
go run main.go -c ./config.yaml command migrate check
go run main.go -c ./config.yaml command migrate up
```

### Adjust migrations during initial development

If a feature is still under development and has not been released:

- update the existing migration files for that feature directly
- do not create extra migration files for iterative local adjustments
- make sure `check` passes before merging

### Repair a dirty state

```bash
go run main.go -c ./config.yaml command migrate version
go run main.go -c ./config.yaml command migrate force <version>
```

## Notes

- prefer explicit `-c ./config.yaml` when using `go run`
- use timestamp versions by default for parallel development
- use `force` only when you are certain about the real database state
- `down --all` is destructive

## References

- [README.en.md](/Users/liuml/data/go/src/go-layout/README.en.md)
- [docs/COMMANDS_AND_TASKS.en.md](/Users/liuml/data/go/src/go-layout/docs/COMMANDS_AND_TASKS.en.md)
- [golang-migrate CLI README](https://github.com/golang-migrate/migrate/blob/master/cmd/migrate/README.md)
