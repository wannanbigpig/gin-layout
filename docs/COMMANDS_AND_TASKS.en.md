# Commands, Scheduled Jobs, and Queue Usage

This document covers:

- Available project commands and what they do
- How to start `service`, `worker`, and `cron`
- How to add scheduled jobs
- How to create, publish, and consume async queue jobs
- How to send jobs to a specific queue and configure multiple queues

If you are new to the project, start with "Runtime Model" and "Commands", then move to "Scheduled Jobs" and "Queue Usage".

## Runtime Model

The project currently splits runtime responsibilities into 3 process types:

- `service`
  - serves the HTTP API
  - handles login, permissions, menus, uploads, logs, and business requests
  - can publish async jobs in some flows

- `worker`
  - consumes async jobs
  - currently handles async request-audit persistence

- `cron`
  - handles recurring schedules
  - currently uses `robfig/cron`
  - fits fixed-time tasks such as "run every day at 2 AM"

In short:

- `service` serves requests
- `worker` consumes jobs
- `cron` triggers recurring tasks

## Commands

The root command entry is in [cmd/root.go](/Users/liuml/data/go/src/go-layout/cmd/root.go).

### 1. Show Help

```bash
go run main.go -h
go run main.go command -h
```

### 2. Start the API Service

```bash
go run main.go service
```

Used for:

- starting the Gin HTTP server
- loading config, logger, database, and other base resources
- serving business APIs

Common scenarios:

- local development
- single-node API deployment
- main application process in a container

### 3. Start the Async Worker

```bash
go run main.go worker
```

Entry file:

- [cmd/worker/worker.go](/Users/liuml/data/go/src/go-layout/cmd/worker/worker.go)

Used for:

- connecting to Redis
- registering all async job handlers
- starting the Asynq worker and consuming jobs

Currently registered:

- `audit:request_log.write`

### 4. Start the Scheduler

```bash
go run main.go cron
```

Entry file:

- [cmd/cron/cron.go](/Users/liuml/data/go/src/go-layout/cmd/cron/cron.go)

Used for:

- starting the scheduler
- registering the current recurring jobs
- shutting down gracefully on process signals

### 5. Run One-Off Commands

```bash
go run main.go command api-route
go run main.go command rebuild-user-permissions
go run main.go command init-system
```

Entry file:

- [cmd/command/command.go](/Users/liuml/data/go/src/go-layout/cmd/command/command.go)

Supported subcommands:

- `api-route`
  - scans the declarative route tree and rebuilds the `api` route table
- `rebuild-user-permissions`
  - rebuilds final user API permissions from database relationships
- `init-system`
  - rolls back migrations, reruns migrations, initializes API routes, and rebuilds user permissions
- `demo`
  - example command

### 6. Show Version

```bash
go run main.go version
```

## Common Startup Combinations

### 1. API Only

Useful when:

- you only need to debug HTTP APIs locally
- your scenario does not depend on async jobs

```bash
go run main.go service
```

### 2. API + Worker

Useful when:

- async jobs need to be consumed
- Redis is enabled

```bash
go run main.go service
go run main.go worker
```

### 3. API + Worker + Cron

Useful when:

- you need the API, async jobs, and recurring tasks at the same time

```bash
go run main.go service
go run main.go worker
go run main.go cron
```

## Scheduled Jobs

The current scheduling code is split into 3 files:

- [cmd/cron/cron.go](/Users/liuml/data/go/src/go-layout/cmd/cron/cron.go)
  - startup and shutdown only
- [cmd/cron/schedule.go](/Users/liuml/data/go/src/go-layout/cmd/cron/schedule.go)
  - scheduling DSL
- [cmd/cron/tasks.go](/Users/liuml/data/go/src/go-layout/cmd/cron/tasks.go)
  - the centralized recurring-job list

### Current Style

The project now recommends defining jobs in `defineSchedule`:

```go
func defineSchedule(schedule *Scheduler) {
	resetService := system.NewResetService()

	schedule.Call("demo", runTask).
		EveryFiveSeconds().
		WithoutOverlapping()

	schedule.CallE("reset-system-data", resetService.ReinitializeSystemData).
		DailyAt("02:00:00").
		WithoutOverlapping()
}
```

Why this is simpler:

- all jobs are declared in one place
- name, schedule, and overlap behavior are visible at a glance
- no need to repeat `AddJob`, `Chain`, and `Recover` manually

### Available Methods

Currently supported:

- `Call(name, func())`
  - register a function with no return value
- `CallE(name, func() error)`
  - register a function returning `error`
- `Cron(spec)`
  - use a raw cron expression
- `EveryFiveSeconds()`
  - run every 5 seconds
- `DailyAt("02:00:00")`
  - run at a fixed time every day
- `WithoutOverlapping()`
  - skip the next run if the current run is still active
- `AllowOverlap()`
  - allow overlapping executions

### Example 1: Run Every 10 Minutes

```go
schedule.Call("cleanup-cache", cleanupCache).
	Cron("0 */10 * * * *").
	WithoutOverlapping()
```

### Example 2: Run Every Day At 3 AM

```go
schedule.CallE("sync-report", reportService.SyncDailyReport).
	DailyAt("03:00:00").
	WithoutOverlapping()
```

### Example 3: Allow Overlap

```go
schedule.Call("heartbeat", heartbeat).
	Cron("0/30 * * * * *").
	AllowOverlap()
```

### Steps To Add A Scheduled Job

1. Prepare the task function in the business layer
2. If the function returns `error`, use `CallE`
3. Add one line in `defineSchedule` inside [tasks.go](/Users/liuml/data/go/src/go-layout/cmd/cron/tasks.go)
4. Restart the `cron` process

## Queue Usage

The queue system is built on top of `Asynq`, but business code does not talk to `asynq.Client` directly. It uses the project-level queue API instead.

Core files:

- [internal/queue/queue.go](/Users/liuml/data/go/src/go-layout/internal/queue/queue.go)
- [internal/queue/asynqx/asynq.go](/Users/liuml/data/go/src/go-layout/internal/queue/asynqx/asynq.go)
- [internal/jobs/registry.go](/Users/liuml/data/go/src/go-layout/internal/jobs/registry.go)

### Queue Flow

A typical async job flows like this:

1. business code builds a payload
2. it calls `queue.PublishJSON(...)`
3. the job enters the target Redis queue
4. `worker` starts and registers handlers
5. Asynq pulls the job from Redis
6. the corresponding handler runs the business logic

### Recommended API

Publish a job:

```go
_, err := queue.PublishJSON(ctx, taskType, queueName, payload, opts...)
```

Register a consumer:

```go
queue.RegisterJSON(registry, taskType, func(ctx context.Context, payload PayloadType) error {
	return handle(payload)
})
```

Compared to the older pattern, this is easier because:

- you do not need to implement a full custom `Job` type
- you do not need to manually `json.Marshal` and `json.Unmarshal`
- payloads are easier to read and reuse
- new jobs are easier to add by copying a small template

## How To Add A New Queue Job

The example below uses "send email".

### 1. Define The Payload

```go
type EmailPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

func (p EmailPayload) Validate() error {
	if p.To == "" {
		return errors.New("to is required")
	}
	return nil
}
```

If the payload implements `Validate() error`, it will be validated automatically before the handler runs. Validation failures are treated as non-retryable.

### 2. Publish The Job

```go
func EnqueueEmail(ctx context.Context, payload EmailPayload) error {
	_, err := queue.PublishJSON(
		ctx,
		"notify:email.send",
		"default",
		payload,
		queue.WithMaxRetry(5),
		queue.WithTimeout(30*time.Second),
	)
	return err
}
```

The parameters mean:

- `"notify:email.send"`
  - task type
- `"default"`
  - queue name
- `payload`
  - task data
- `WithMaxRetry`
  - maximum retry count
- `WithTimeout`
  - execution timeout

### 3. Register The Consumer

```go
func registerEmail(registry queue.Registry) {
	queue.RegisterJSON(registry, "notify:email.send", func(ctx context.Context, payload EmailPayload) error {
		return sendEmail(payload)
	})
}
```

### 4. Register It In The Unified Entry

Current recommended pattern:

```go
func RegisterAll(registry queue.Registry) {
	queue.RegisterJSON(registry, AuditLogTaskType, handleAuditLog)
	registerEmail(registry)
}
```

### 5. Worker Consumes It Automatically

When the worker starts, it does this:

```go
registry := jobs.NewRegistry()
server, mux, err := asynqx.NewServer(cfg, registry)
```

So if your handler is already registered in `RegisterAll`, the worker will consume it automatically.

## How To Send A Job To A Specific Queue

The queue name is the third argument of `PublishJSON`:

```go
queue.PublishJSON(ctx, "notify:email.send", "critical", payload)
```

Here `"critical"` is the queue name.

Typical usage:

- audit logs go to `audit`
- normal jobs go to `default`
- high-priority jobs go to `critical`
- cleanup and low-priority jobs go to `low`

## How To Configure Multiple Queues

Example config:

```yaml
queue:
  enable: true
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

Meaning:

- `queues`
  - which queues the worker should listen to
- numbers
  - relative weights for queue scheduling
- `concurrency`
  - worker concurrency
- `strict_priority`
  - whether to strictly prefer higher-priority queues

### Multi-Queue Example

```yaml
queue:
  queues:
    critical: 5
    default: 3
    audit: 2
    low: 1
```

One possible split:

- `critical`
  - permission repair, account-state sync
- `default`
  - normal async business jobs
- `audit`
  - request audit logs
- `low`
  - cleanup, reporting, compensation jobs

## Real Example In This Project

The currently integrated job is the audit log job:

- task type: `audit:request_log.write`
- queue: `audit`

File:

- [internal/jobs/audit_log.go](/Users/liuml/data/go/src/go-layout/internal/jobs/audit_log.go)

Producer:

```go
func EnqueueAuditLog(ctx context.Context, kind string, snapshot *auditsvc.AuditLogSnapshot) error {
	payload, err := NewAuditLogPayload(kind, snapshot)
	if err != nil {
		return err
	}
	_, err = queue.PublishJSON(ctx, AuditLogTaskType, AuditQueueName, payload, auditLogOptions()...)
	return err
}
```

Consumer registration:

```go
func RegisterAll(registry queue.Registry) {
	queue.RegisterJSON(registry, AuditLogTaskType, handleAuditLog)
}
```

Worker startup:

```go
registry := jobs.NewRegistry()
server, mux, err := asynqx.NewServer(cfg, registry)
```

## When To Use Cron vs Queue

Recommended split by responsibility:

- `cron`
  - decides when something should run
  - good for recurring schedules
- `queue`
  - decides how something runs asynchronously
  - good for retries, decoupling, and background consumption

### Typical Cases

Use `cron` for:

- resetting system data every day
- clearing cache every hour
- syncing statistics every 5 minutes

Use `queue` for:

- audit-log persistence
- email sending
- webhook delivery
- incremental permission sync

Use both together when needed:

- `cron` triggers on schedule
- `cron` publishes a queue job
- `worker` performs the actual work

## FAQ

### 1. Why Was The Job Published But Not Consumed?

Check these first:

- Redis is available
- `queue.enable` is enabled
- `worker` is running
- the task is registered in `RegisterAll`

### 2. Why Do Invalid Payloads Not Retry?

Because `RegisterJSON` treats JSON decode failures and `Validate()` failures as invalid input. Retrying those usually does not help.

### 3. Why Does The Physical Queue Name Include A Prefix?

Because the current implementation supports `namespace`, for example:

- configured queue name: `audit`
- actual Redis queue name: `go_layout:audit`

Business code should keep using the logical queue name.

### 4. When Should I Implement `queue.Job` Myself?

Most business scenarios do not need it.

Only consider it when you need:

- a custom non-JSON payload flow
- a special payload generation path
- unusual job object behavior

By default, prefer:

- `queue.PublishJSON`
- `queue.RegisterJSON`

## Recommended Practices

- keep scheduled jobs in [tasks.go](/Users/liuml/data/go/src/go-layout/cmd/cron/tasks.go)
- keep async jobs under `internal/jobs`
- keep one clear payload per task type
- keep payloads small and stable
- separate high-priority and low-priority jobs by queue
- avoid calling Asynq APIs directly from business code
- prefer the project-level helpers: `PublishJSON` and `RegisterJSON`
