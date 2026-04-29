# Backend Guidelines for System Config and Dictionary

This document defines the `sys_config` and `sys_dict` boundary from the backend perspective. The main goal is to prevent backend-owned core rules, such as permissions, state machines, data structure meaning, and security boundaries, from becoming admin-editable configuration, while still providing a clear, auditable, and testable integration path for runtime policies and display-oriented enums.

## Core Principle

- `sys_config` owns runtime policy. Backend reads must have type constraints, defaults, and fallback behavior.
- `sys_dict` owns display options. Backend code should use it at most to return display data such as labels, tags, and colors.
- Model constants, validators, service state machines, and migrations own core rules.

Admins may adjust selected operational thresholds and display behavior, but they must not bypass code-owned permission checks, state transitions, field meanings, audit semantics, or security constraints.

## When to Use `sys_config`

Use `sys_config` from backend code for:

- Runtime switches for low-risk features.
- Operational thresholds, such as login lock limits, lock duration, and log retention windows.
- Security policy configuration, such as request-log masking fields.
- Parameters that affect behavior, may be adjusted without a release, and have explicit fallback behavior.

Every new config should include:

- A config key constant. Do not scatter raw key strings in business code.
- A value type, default value, and allowed range.
- A domain read helper, such as `BoolValue`, `IntValue`, or a dedicated service method.
- Fallback behavior for read failures, missing config, or type errors.
- A sensitivity flag when needed.
- Cache refresh, startup warmup, or runtime reload behavior.
- Seed data in migrations.
- Unit tests or critical-path tests.

Backend `sys_config` integration must guarantee:

- The service layer keeps business fallbacks and does not treat config values as trusted input.
- Validators remain responsible for request parameter legality and are not replaced by config-table values.
- Sensitive configs must not leak through generic detail, value, request-log, or change-diff paths.
- Protected built-in configs must not allow stable fields such as key, value type, or group code to be changed.
- When config changes affect runtime behavior, it must be clear whether cache refresh or process restart is required.

## When Not to Use `sys_config`

Do not put these into `sys_config`:

- Permission rules.
- Route definitions.
- Database schema meaning.
- Core state transitions.
- The scheduler source of truth, unless scheduler has explicitly been redesigned to read DB definitions.
- Rules where an admin-side edit would silently change core system behavior and make troubleshooting harder.

These should remain enforced by code constants, validators, service state machines, database constraints, and migrations.

## When to Use `sys_dict`

Use `sys_dict` from backend code to provide:

- Frontend select options.
- Table tag labels, colors, and tag types.
- Localized display labels.
- Enumerations whose changes mainly affect display, not backend rules.

Recommended display dictionaries:

- `common_status`: enabled/disabled labels.
- `yes_no`: yes/no labels.
- `menu_type`: menu type labels.
- `api_auth_mode`: API auth mode labels.
- `http_method`: HTTP method labels.
- `task_kind`: task kind labels.
- `task_source`: task source labels.
- `task_run_status`: task run status labels.

When backend code provides dictionary options, it is only responsible for display data output. Even if dictionary items are disabled, deleted, or edited incorrectly, backend core APIs must still enforce legality through validators, model constants, and service logic.

## When Not to Use `sys_dict`

Do not use `sys_dict` as the backend source of truth for:

- Validator legal values.
- Task-run state transitions.
- Role, menu, or user enable/disable checks.
- Actual permission-auth-mode execution.
- Core audit diff decisions.

Backend code may reuse dictionary data for display, but dictionary edits must not weaken business rules. For example, `status=2` must not become valid just because a new dictionary option was added.

## Backend Implementation Requirements

- Put new config or dictionary seed data in initialization migrations first.
- During unreleased development, do not add permanent runtime sync logic for seed data unless necessary.
- For shipped features or shared environments, evolve data through new migrations or explicit idempotent sync entry points.
- Keep config reads in services or helpers. Controllers should not assemble business rules directly.
- Core enums must be explicitly expressed in form validators and model constants.
- Audit diffs may use label mappings for readability, but diff fields and security masking rules must be controlled by code.
- API documentation must describe config/dictionary field semantics, enum meanings, and PATCH/replace update semantics.

## Frontend Collaboration Constraints

- Frontend table filters, tags, and normal selects may prefer `dict/options`.
- Pages must keep local fallbacks so offline usage or dictionary API failures do not break the UI.
- For fields such as `status` and `is_*`, frontend pages may use dictionaries for display, while backend validators and model constants remain authoritative.
- When frontend code adds a dependency on a dictionary type code, backend seed data, API documentation, and fallback conventions should be updated together.

## Decision Table

| Need | Recommendation |
| --- | --- |
| Select options, table tags, colors, localized labels | Use `sys_dict` |
| Login lock limits, request-log masking fields, operational thresholds | Use `sys_config` |
| Validator enums, permission modes, state transitions | Use code constants and validators |
| Real cron enable/disable from admin UI | Decide scheduler source of truth first |
| Backend needs runtime policy reads | Read `sys_config` through domain helpers and keep defaults |
| Reduce frontend enum hardcoding | Use `dict/options` with local fallback |
