# <div align="center">gin-layout</div>

<div align="center">
  <a href="./README.md">中文</a> | <strong>English</strong>
</div>

<br />

<div align="center">
  <strong>A Gin-based admin backend scaffold</strong>
</div>

<div align="center">
  Built with JWT auth, RBAC, request/login logging, file upload, readiness probes, validation, request-locale i18n, declarative routing, and CLI initialization commands.
</div>

<br />

<div align="center">
  <img src="https://github.com/wannanbigpig/gin-layout/actions/workflows/go.yml/badge.svg" alt="go" />
  <img src="https://github.com/wannanbigpig/gin-layout/actions/workflows/codeql.yml/badge.svg" alt="codeql" />
  <img src="https://goreportcard.com/badge/github.com/wannanbigpig/gin-layout" alt="go report card" />
  <img src="https://img.shields.io/github/license/wannanbigpig/gin-layout" alt="license" />
  <img src="https://img.shields.io/badge/Go-%3E%3D1.23-blue.svg" alt="go version" />
</div>

<br />

## Why This Exists

Most admin projects start with the same goal: get login, permissions, menus, uploads, and logs working quickly. In practice, the same engineering problems show up again and again:

- Auth, permissions, logging, and file handling are split across too many places
- Route declarations, menus, and API permissions drift apart over time
- The same admin infrastructure is rewritten repeatedly across projects
- Config, command, migration, and deployment workflows lack a clear baseline

`gin-layout` is built to turn these repeated backend concerns into a reusable, production-oriented foundation for admin systems.

## Highlights

| Capability | Description |
| --- | --- |
| Auth | JWT login, token verification, auto refresh, and blacklist support |
| RBAC | Admin, role, department, menu, and API permission management |
| Route Metadata | Declarative route tree generates both Gin routes and API metadata |
| Logs | Built-in login logs, request logs, and unified response structure |
| File Access | File upload and public / private file access |
| I18n | Error messages and menu titles are resolved by `Accept-Language` (`zh-CN` / `en-US`) |
| Health Probes | Built-in `/ping` and `/health/readiness` for liveness and dependency-readiness checks |
| Tooling | CLI commands for initialization, route sync, permission rebuild, and migrations |
| Hot Reload | Partial config hot reload with fallback to previous live instances on failure |

## Related Resources

- Frontend project: [go-admin-ui](https://github.com/wannanbigpig/go-admin-ui)
- Online docs: [Apifox](https://wannanbigpig.apifox.cn/)
- Demo: [Live Demo](https://x-l-admin.wannanbigpig.com/)
- Commands and jobs guide: [docs/COMMANDS_AND_TASKS.en.md](./docs/COMMANDS_AND_TASKS.en.md)
- Migration command guide: [docs/MIGRATE_COMMANDS.en.md](./docs/MIGRATE_COMMANDS.en.md)

## Quick Start

### 1. Requirements

- `Go >= 1.23`
- `MySQL >= 5.7`
- `Redis >= 5.0` (optional)

### 2. Install

```bash
git clone https://github.com/wannanbigpig/gin-layout.git
cd gin-layout
go mod download
```

### 3. Run Migrations

Recommended project commands:

```bash
go run main.go command migrate        # defaults to migrate up
go run main.go command migrate check
```

After migrations finish, a default baseline dataset is inserted, including the super admin account `super_admin / 123456`. It is only recommended for local initialization. Change the password immediately after the first login.

For migration file creation, timestamp naming rules, and full `down/goto/force/version` usage, see [docs/MIGRATE_COMMANDS.en.md](./docs/MIGRATE_COMMANDS.en.md).

### 4. Configure

For source runs, `GO_ENV=development` is recommended. Without `-c`:

- development mode uses `config.yaml` in the current working directory
- if missing, it copies from `config/config.yaml.example` in the current working directory
- non-development mode resolves `config.yaml` next to the executable and copies from sibling `config.yaml.example` when needed

You can also copy and edit the config file manually.

Minimal example:

```yaml
app:
  app_env: local
  debug: true
  language: zh_CN
  trusted_proxies:
    - 127.0.0.1
  watch_config: true
  # allow_degraded_startup: false

jwt:
  ttl: 7200
  refresh_ttl: 3600
  secret_key: change-me-to-a-random-secret

mysql:
  enable: true
  host: 127.0.0.1
  port: 3306
  database: go_layout
  username: root
  password: your_password

redis:
  enable: true
  host: 127.0.0.1
  port: 6379
  password: ""
  database: 0

queue:
  enable: true
  use_default_redis: true
  namespace: go_layout
  concurrency: 8
  strict_priority: false
  queues:
    critical: 4
    default: 2
    audit: 2
    low: 1
  audit_max_retry: 3
  audit_timeout_seconds: 10
```

Notes:

- `jwt.secret_key` is required and cannot be empty
- If you only run the API and do not need async jobs, set `queue.enable` to `false`
- If `queue.enable=true` but you do not want to reuse `redis.*`, set `queue.use_default_redis` to `false` and fill in `queue.redis.*`

### 5. Start Service

```bash
GO_ENV=development go run main.go service
```

To explicitly set the listen host or port:

```bash
GO_ENV=development go run main.go service -H 127.0.0.1 -P 9001
```

If `queue.enable=true`, start the worker in a separate process as well:

```bash
GO_ENV=development go run main.go worker
```

### 6. Verify

```bash
curl http://127.0.0.1:9001/ping
curl http://127.0.0.1:9001/health/readiness
```

- `/ping` returns `pong` when the HTTP process is alive.
- `/health/readiness` returns `ready=true` when required dependencies are ready for the current runtime mode.

## Core Ideas

### Prefer Declarative Routing

Admin routes are maintained in a single declarative route tree. The current entry is `AdminRouteTree()` in `internal/routers/admin_router.go`. Gin route registration and API metadata initialization are both generated from that tree so route code and permission metadata do not drift apart.

### Database Relations Are The Source Of Truth

The current permission model treats database relations as the source of truth, while Casbin performs final API authorization checks. The `rebuild-user-permissions` command rebuilds each user's final API permissions from user, department, role, menu, and API relationships stored in the database.

### Hot Reload Is Tiered

The project supports config hot reload, but not every setting can be applied live. Supported resources are rebuilt when possible. If a rebuild fails, the previous live instance is kept so the service can continue running.

### Request-Locale Driven Texts

The request pipeline parses `Accept-Language` (currently `zh-CN` / `en-US`) and applies normalized fallback behavior:

- Error messages are resolved by request locale, with fallback to default locale (`zh-CN`).
- Menu list / user menu tree returns localized `title` only.
- Menu detail returns `title_i18n` for edit backfill, not `title`.
- Menu create/update writes `title_i18n`; at least one of `zh-CN` / `en-US` must be non-empty.

## Commands

Help:

```bash
go run main.go -h
go run main.go command -h
go run main.go service --help
```

Common commands:

| Command | Description |
| --- | --- |
| `go run main.go version` | Print the current version |
| `go run main.go service` | Start the API service |
| `go run main.go service -H 0.0.0.0 -P 9001` | Explicitly set the listen host and port |
| `go run main.go worker` | Start the Asynq async worker |
| `go run main.go cron` | Start scheduled jobs |
| `go run main.go command demo` | Run the demo command |
| `go run main.go command api-route` | Scan the declarative route tree and rebuild the `api` route table |
| `go run main.go command rebuild-user-permissions` | Rebuild final user API permissions from database relationships |
| `go run main.go command init-system` | Roll back and rerun migrations, rebuild API routes, and rebuild user permissions |
| `go run main.go -c ./config.yaml command task scan-async` | Scan async task registration against the task-definition mirror |
| `go run main.go -c ./config.yaml command task scan-cron` | Scan cron task definitions against the task-definition mirror |
| `go run main.go -c ./config.yaml command migrate check` | Validate migration naming and up/down pairing |
| `go run main.go -c ./config.yaml command migrate up` | Apply all pending migrations |
| `go run main.go -c ./config.yaml command migrate down 1` | Roll back one migration version |

If the config file is not in the default location:

```bash
go run main.go -c /path/to/config.yaml service
go run main.go -c /path/to/config.yaml command init-system
```

See [docs/MIGRATE_COMMANDS.en.md](./docs/MIGRATE_COMMANDS.en.md) for full migration command details.

## Configuration

### Config Resolution

Config lookup order:

1. Explicit `-c` / `--config`
2. `config.yaml` in the current working directory for development mode
3. if missing, copy from `config/config.yaml.example` in the current working directory
4. `config.yaml` next to the executable for non-development mode
5. if missing, copy from sibling `config.yaml.example`

### Key Settings

| Key | Description |
| --- | --- |
| `app.base_path` | Base directory for logs, uploaded files, and other local paths; when not set it follows `GO_ENV` (development=current working directory, otherwise executable directory) |
| `app.allow_degraded_startup` | Only applies to the `service` command; allows the HTTP service to start when dependency initialization fails, exposing the not-ready state through readiness and route guards |
| `app.base_url` | URL prefix used to generate public file access URLs |
| `app.trusted_proxies` | Trusted proxy addresses or CIDRs that affect `ClientIP()` and log IPs |
| `jwt.secret_key` | Required; do not use weak placeholder values in production |
| `jwt.ttl` / `jwt.refresh_ttl` | Token expiration and auto-refresh threshold |
| `mysql` | Database enable flag and connection settings |
| `redis` | Cache, blacklist, and distributed lock settings |
| `queue.use_default_redis` | `true` reuses `redis.*`; `false` uses the independent `queue.redis.*` connection |
| `queue` | Asynq enable flag, queue concurrency, priorities, and audit-log retry settings |
| `logger` | Log output, rotation, and retention strategy |

If requests pass through Nginx, Ingress, or a load balancer, keep `app.trusted_proxies` aligned or client IP logging may be inaccurate.

### Worker And Cron

- `service` serves the HTTP API.
- `worker` consumes Asynq jobs. The current first phase only moves request audit-log persistence to Asynq.
- When `queue.enable=false`, you do not need to start `worker`; request audit logs are persisted synchronously in the current request flow.
- `cron` owns scheduled jobs. The demo cron task is controlled by the system config `task.cron_demo_enabled` and is disabled by default after initialization.
- `reset-system-data` is only registered when `app.enable_reset_system_cron=true` is explicitly configured, and startup logs a high-risk warning.
- Do not register the same recurring business task in both `cron` and the async worker flow, or it will run twice.

Note: `reset-system-data` currently calls `system.ReinitializeSystemData()`, which rolls back migrations and rebuilds system data. Keep `app.enable_reset_system_cron=false` in production, and only enable it temporarily when you explicitly need to rebuild system data.

### Hot Reload

Enable it with:

```yaml
app:
  watch_config: true
```

Hot-reload supported:

- `logger.*`
- `mysql.*`
- `redis.*`
- `app.base_url`
- `app.cors_*`
- `jwt.ttl`
- `jwt.refresh_ttl`

Detected but requires restart:

- `app.trusted_proxies`
- `app.language`
- `app.allow_degraded_startup`
- `jwt.secret_key`
- service listen address and port
- route structure

Notes:

- `watch_config=true` only enables file watching; it does not mean every setting is safely swappable
- MySQL, Redis, and Casbin instances are rebuilt from the new config and the old instance is kept on failure
- JWT secret hot reload is not currently supported; changes are logged and only take effect after restart

## Development

### Add A New Endpoint

1. Write the controller in `internal/controller/`
2. Write the business logic in `internal/service/`
3. Define request params in `internal/validator/form/`
4. Declare the route in `AdminRouteTree()`
5. Run `go run main.go command api-route` if the API route table needs to be refreshed

### Validation Conventions

- For enum-like fields (for example `status`, `is_auth`, `method`), prefer explicit `oneof` constraints.
- For ID arrays (for example `role_ids`, `menu_list`, `api_list`), prefer `dive,gt=0` so invalid IDs such as `0` are rejected at validation stage.
- When adding/changing validation rules, add both positive and negative tests to prevent regressions.

### Test

```bash
go test ./...
go test ./tests/admin_test
```

Tests prefer the root `config.yaml`. If MySQL or Redis is unavailable in the current environment, the test setup falls back to example config paths for cases that can run without those external services.

## Deployment

### Build

```bash
go build -o go-layout main.go
./go-layout service
```

If `-c` is not provided, use `GO_ENV=development` in development and ensure `config/config.yaml.example` exists in the current working directory or `config.yaml` has already been generated. For binary deployment, keep the config file next to the executable.

### Supervisor

```ini
[program:go-layout]
command=/path/to/go-layout -c /path/to/config.yaml service
directory=/path/to/go-layout
autostart=true
autorestart=true
startsecs=5
user=www-data
redirect_stderr=true
stdout_logfile=/path/to/go-layout/supervisord.log
```

### Nginx

```nginx
server {
    listen 80;
    server_name api.example.com;

    location / {
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_pass http://127.0.0.1:9001;
    }
}
```

If you are behind a reverse proxy, add the proxy address or CIDR to `app.trusted_proxies`.

## Project Layout

```text
gin-layout/
├── cmd/                    # CLI entrypoints
├── config/                 # Config structs and example config
├── data/                   # MySQL / Redis and migrations
├── docs/                   # Supplementary docs and resources
├── internal/
│   ├── access/             # Access and permission infrastructure
│   ├── controller/         # Controllers
│   ├── middleware/         # Middlewares
│   ├── model/              # Data models
│   ├── resources/          # Resource transformers
│   ├── routers/            # Declarative routing
│   ├── service/            # Business services
│   └── validator/          # Request validation
├── pkg/                    # Shared utilities
├── storage/                # File storage
├── tests/                  # Route and integration tests
└── README.md
```

## 💝 Support This Project

Thanks for using `gin-layout`.

If this project helps you, you can support its ongoing development and maintenance.

<a href="./docs/DONATE.en.md">
  <img src="https://img.shields.io/badge/BUY_ME_A_COFFEE-SUPPORT_AUTHOR-f08a24?style=for-the-badge&logo=buymeacoffee&logoColor=ffdd00&labelColor=4a4a4a" alt="Support the author" />
</a>

## License

This project is released under the MIT License. See [LICENSE](LICENSE).

## Contributing

Issues and pull requests are welcome.

## Disclaimer

This project is provided **“as is”**, without any express or implied warranty. It may contain defects, security vulnerabilities, or implementations that do not fit a specific business scenario. Before using it in production, you should perform your own code review, security hardening, configuration review, permission validation, and data backup. Any issues caused by using, relying on, deploying, modifying, or operating this project are the responsibility of the user.
