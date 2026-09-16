# <div align="center">gin-layout</div>

<div align="center">
  <a href="./README.md">中文</a> | <strong>English</strong>
</div>

<br />

<div align="center">
  <strong>A Gin-based admin backend scaffold</strong>
</div>

<div align="center">
  Built with JWT dual-token auth, RBAC, request/login logging, file upload, a task center, system notifications, readiness probes, validation, request-locale i18n, declarative routing, and CLI commands.
</div>

<br />

<div align="center">
  <img src="https://github.com/wannanbigpig/gin-layout/actions/workflows/go.yml/badge.svg" alt="go" />
  <img src="https://github.com/wannanbigpig/gin-layout/actions/workflows/codeql.yml/badge.svg" alt="codeql" />
  <img src="https://goreportcard.com/badge/github.com/wannanbigpig/gin-layout" alt="go report card" />
  <img src="https://img.shields.io/github/license/wannanbigpig/gin-layout" alt="license" />
  <img src="https://img.shields.io/badge/Go-%3E%3D1.27-blue.svg" alt="go version" />
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
| Auth | JWT dual-token login, access-token verification, refresh-token cookie refresh, and blacklist support |
| RBAC | Admin, role, department, menu, and API permission management |
| Route Metadata | Declarative route tree generates both Gin routes and API metadata |
| Logs | Built-in login logs, request logs, and unified response structure |
| File Access | File upload and public / private file access |
| Task Center | Built-in async jobs, cron task definitions, run records, retries, and operation settings |
| Realtime | System notifications, WebSocket temporary tickets, and real-time message connections |
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

- `Go >= 1.27`
- `MySQL >= 8.0`
- `Redis >= 6.2` (optional; required for WebSocket tickets, caching, queues, or distributed locks)

### 2. Install

```bash
git clone https://github.com/wannanbigpig/gin-layout.git
cd gin-layout
go mod download
```

### 3. Run Migrations

Recommended project commands:

```bash
go run main.go -c ./config.yaml command migrate        # defaults to migrate up
go run main.go -c ./config.yaml command migrate check
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
  watch_config: false
  # allow_degraded_startup: false

jwt:
  ttl: 1800
  refresh_ttl: 0
  refresh_token_ttl: 604800
  refresh_cookie_name: refresh_token
  refresh_cookie_path: /admin/v1/auth
  refresh_cookie_same_site: Lax
  refresh_cookie_secure: false
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
    export: 2
    low: 1
  audit_max_retry: 3
  audit_timeout_seconds: 10
```

Notes:

- `jwt.secret_key` is required and cannot be empty
- `jwt.refresh_ttl` is deprecated; the current dual-token flow uses `jwt.refresh_token_ttl` and `jwt.refresh_cookie_*`
- If you only run the API and do not need async jobs, set `queue.enable` to `false`
- If `queue.enable=true` but you do not want to reuse `redis.*`, set `queue.use_default_redis` to `false` and fill in `queue.redis.*`
- The active storage driver and sensitive storage settings are persisted in `sys_config` (`storage.active_driver` / `storage.config`) and managed through `/admin/v1/system/storage/*`; `config.yaml` mainly covers process-level dependencies and runtime knobs

### 5. Start Service

```bash
GO_ENV=development go run main.go -c ./config.yaml service
```

To explicitly set the listen host or port:

```bash
GO_ENV=development go run main.go -c ./config.yaml service -H 127.0.0.1 -P 9001
```

If `queue.enable=true`, start the worker in a separate process as well:

```bash
GO_ENV=development go run main.go -c ./config.yaml worker
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

Admin routes are maintained in one declarative route tree rooted at `AdminRouteTree()` in `internal/routers/admin_router.go`. `internal/routers/catalog.go` compiles full paths, groups, security policies, and handler chains once; Gin registration, API metadata sync, and OpenAPI all consume that Catalog so independent walkers cannot drift.

Security policies are declared on route groups and inherited by child routes. `api.is_auth` remains a compatible `0/1/2` projection generated by `api-route`; it is not editable from API permission maintenance. New routes should be placed under `SecurityPublic`, `SecurityRefreshCookie`, `SecurityWebSocketTicket`, `SecurityAccessToken`, or `SecurityAccessTokenPermission` as appropriate.

### Database Relations Are The Source Of Truth

The current permission model treats database relations as the source of truth, while Casbin performs final API authorization checks. The `rebuild-user-permissions` command rebuilds each user's final API permissions from user, department, role, menu, and API relationships stored in the database.

Casbin policy reload supports multiple `service` instances. Policy rows and a global policy version are committed in the same MySQL transaction. Authorization requests read the version from Redis first and synchronously reload from MySQL before enforcement when the local snapshot is behind. A permission write sets a Redis dirty marker before commit, then updates the cached version and publishes a notification after commit. Requests fall back to MySQL when Redis is unavailable, the version is missing, or a dirty marker exists; concurrent fallback reads are coalesced per instance with `singleflight`. The Redis version has a 30-second safety expiry, and periodic reconciliation remains as an additional fallback. Version-read or reload failures fail closed. This provides near-real-time propagation on the normal path and bounded eventual consistency on failure paths, rather than a cross-replica acknowledgement barrier.

When upgrading to a release with policy-version checks, run `command migrate up` to create `permission_policy_versions` before starting or rolling out `service` instances. If the table is missing, authorization fails closed and the service log reports the missing migration.

All replicas must use the same MySQL database and Redis logical database, and must share the same `jwt.secret_key`, `app.app_env`, and `queue.namespace`. Policy, system-config cache, and other internal Pub/Sub channels are isolated using `queue.namespace`, `app.app_env`, and `redis.database`; do not use different values among processes in one deployment. The system-config cache also reloads from the database at startup, subscribes to refresh notifications, and reloads every 30 seconds as a fallback.

If a rolling upgrade changes channel naming, old and new versions do not notify one another; a short mixed-version window can delay policy or cache refresh. Coordinate restarts of all `service`, `worker`, and `cron` processes before resuming traffic.

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
| `go run main.go -c ./config.yaml service` | Start the API service |
| `go run main.go -c ./config.yaml service -H 0.0.0.0 -P 9001` | Explicitly set the listen host and port |
| `go run main.go -c ./config.yaml worker` | Start the Asynq async worker |
| `go run main.go -c ./config.yaml cron` | Start scheduled jobs |
| `go run main.go command demo` | Run the demo command |
| `go run main.go -c ./config.yaml command gen --from=gen/specs/product.yaml` | Generate CRUD code from a table or YAML spec |
| `go run main.go -c ./config.yaml command gen-openapi` | Generate `docs/openapi.yaml` from the declarative route Catalog |
| `go run main.go -c ./config.yaml command api-route` | Scan the declarative route tree and rebuild the `api` route table |
| `go run main.go -c ./config.yaml command rebuild-user-permissions` | Rebuild final user API permissions from database relationships |
| `go run main.go -c ./config.yaml command init-system` | Drop objects in the configured database and rebuild system data |
| `go run main.go -c ./config.yaml command task sync-definitions` | Non-destructively sync built-in task definitions into `task_definitions` |
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

Notes:

- `api-route` and `rebuild-user-permissions` prompt for confirmation by default; automation can pass `-y`.
- `init-system` is destructive. It drops objects in the configured database and rebuilds system data. Before running, it prints a red warning with app environment and MySQL host, port, database, username, and table prefix, then requires a backup confirmation and the current database name.
- `init-system -y` only skips the ordinary prompt. Automation must also pass `--backup-confirmed --confirm-database <mysql.database>`, for example:

```bash
go run main.go -c ./config.yaml command init-system -y --backup-confirmed --confirm-database go_layout_test
```

- After adding built-in async or cron tasks in a deployed environment, do not run `init-system`. Run `command task sync-definitions` to sync task definitions, then use `scan-async` / `scan-cron` to verify that tasks exist in the table. The sync only updates code-owned definition fields and preserves operational settings such as `status`, `cron_spec`, manual trigger, retry, and record policies.
- `20260916000001_add_refresh_session_lookup_index` is an independent refresh-token lookup-index migration. During deployment, first run `go run main.go -c ./config.yaml command migrate up`, then run `go run main.go -c ./config.yaml command api-route -y` in an authorized target environment to sync the API table and permissions for the new route.

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
| `app.max_request_body_mb` | General API request-body limit, `16` MB by default |
| `app.max_upload_body_mb` | Total request-body limit for local `multipart` uploads, `256` MB by default; includes all files and form fields and does not replace the per-file `storage.max_file_size_mb` limit |
| `app.allow_degraded_startup` | Only applies to the `service` command; allows the HTTP service to start when dependency initialization fails, exposing the not-ready state through readiness and route guards |
| `app.base_url` | URL prefix used to generate public file access URLs |
| `app.trusted_proxies` | Trusted proxy addresses or CIDRs that affect `ClientIP()` and log IPs |
| `jwt.secret_key` | Required; do not use weak placeholder values in production |
| `jwt.ttl` / `jwt.refresh_token_ttl` / `jwt.refresh_cookie_*` | Access-token lifetime, refresh-token cookie lifetime, and browser cookie policy; `jwt.refresh_ttl` is deprecated |
| `mysql` | Database enable flag and connection settings |
| `redis` | Cache, blacklist, and distributed lock settings |
| `queue.use_default_redis` | `true` reuses `redis.*`; `false` uses the independent `queue.redis.*` connection |
| `queue` | Asynq enable flag, queue concurrency, priorities, and audit-log retry settings |
| `storage.active_driver` / `storage.config` | Runtime file-storage driver and sensitive storage config in `sys_config`, managed through `/admin/v1/system/storage/*` |
| `logger` | Log output, rotation, and retention strategy |

If requests pass through Nginx, Ingress, or a load balancer, keep `app.trusted_proxies` aligned or client IP logging may be inaccurate.

### Worker And Cron

- `service`, `worker`, and `cron` are three process roles. Scale the API by adding `service` replicas; they are not three API instances.
- `service` serves the HTTP API.
- `worker` consumes Asynq jobs. The current built-ins cover request-audit persistence, request-log export, and file-list export.
- To use exports, `queue.queues` must include a consumable `export` queue and a `worker` must be running; registering a task or starting `service` alone does not consume export jobs.
- When `queue.enable=false`, you do not need to start `worker`; request audit logs are persisted synchronously, but export-related async capabilities are also unavailable.
- `cron` owns scheduled jobs. Built-in task definitions are synced into `task_definitions`, and runtime truth comes from operational fields such as `status`, `cron_spec`, `allow_manual`, and `allow_retry`, with recent sync/run state tracked through in-process cache plus `cron_task_states`.
- The demo cron task is disabled by default after initialization. `reset-system-data` is also disabled by default and logs a high-risk warning when it is enabled for execution.
- Do not register the same recurring business task in both `cron` and the async worker flow, or it will run twice.
- High-risk cron tasks use a Redis distributed lock for cross-instance mutual exclusion. Ordinary tasks only have process-local `WithoutOverlapping` protection and may still run concurrently across multiple `cron` instances. Run one `cron` instance until ordinary tasks are idempotent.

Note: `reset-system-data` currently calls `system.ReinitializeSystemData()`, which drops database objects and rebuilds system data from the current migrations. Keep `task_definitions.status=0` for this task in production, and only enable it temporarily when you explicitly need to rebuild system data.

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
- `jwt.*` (`jwt.secret_key` changes are detected, but still require restart to take effect)

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
4. Declare the route under the appropriate security-policy group in `AdminRouteTree()`
5. Run `go run main.go -c ./config.yaml command api-route` if the API route table needs to be refreshed

### Validation Conventions

- For enum-like fields (for example `status`, `is_auth`, `method`), prefer explicit `oneof` constraints.
- For ID arrays (for example `role_ids`, `menu_list`, `api_list`), prefer `dive,gt=0` so invalid IDs such as `0` are rejected at validation stage.
- When adding/changing validation rules, add both positive and negative tests to prevent regressions.

### Test

```bash
go test ./...
go test ./tests/admin_test
```

HTTP integration tests under `tests` use only the root `config.yaml` and require a real, reachable MySQL instance. Missing configuration, unavailable MySQL, or missing migration tables fails the run instead of falling back to the example config or skipping database flows. Use a rebuildable dedicated test database; see `tests/README.md`.

File-upload contract: `GET /admin/v1/system/file/upload/options` requires `file:create` permission and returns only the minimum current-driver, size/type allowlist, and direct/multipart capability settings; it does not return credentials, `bucket`, or endpoint. Upload initialization creates a server-side session and signs a `complete_token`; complete accepts only server metadata bound to that token, and multipart abort accepts only `complete_token`. On direct-upload completion, the server streams the temporary object into an independent final key and validates the actual content with SHA-256; the client hash is an integrity declaration and is not used for instant server-side reuse. Legacy sessions created for unverified instant reuse are rejected after upgrade; clients must initialize a new upload.

Audit logs use the `request_logs.change_diff` field. Writes with field-level differences can be filtered in list and export APIs with `has_change_diff=1`. Low-risk successful read-only requests may be sampled using `queue.audit_sample_get_keep_every`; writes are not sampled. Queue, database, or shutdown fallback paths do not guarantee zero loss on process crashes, and non-database triggers do not cover manual SQL.

## Deployment

### Build

```bash
go build -o go-layout main.go
./go-layout service
```

If `-c` is not provided, use `GO_ENV=development` in development and ensure `config/config.yaml.example` exists in the current working directory or `config.yaml` has already been generated. For binary deployment, keep the config file next to the executable.

### Load Balancing And Multiple Instances

Multiple API instances can be exposed through a load balancer or Kubernetes Service. The load balancer must support WebSocket Upgrade. Use `/health/readiness` for readiness and `/ping` for liveness; readiness returns a non-2xx status when required dependencies are unavailable, so the instance should not receive traffic.

The default `local` storage is for a single instance. For multiple instances, enable OSS or another shared object store and use the same storage configuration for `service`, `worker`, and `cron`. All processes must also share the same MySQL/Redis logical database, JWT secret, and environment namespace.

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
map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

server {
    listen 80;
    server_name api.example.com;

    location / {
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_pass http://127.0.0.1:9001;
    }
}
```

Place the `map` in the Nginx `http` block. If a cloud load balancer or Ingress is used, enable its WebSocket Upgrade support and long-connection timeout as well.

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
