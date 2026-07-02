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
- Sensitive configs must not leak through generic list, `PublicValue`, request-log, or change-diff paths. `detail` may return the raw value only for authenticated admin edit backfill, and callers must not blindly re-display it.
- Protected built-in configs must not allow stable fields such as key, value type, or group code to be changed.
- When config changes affect runtime behavior, it must be clear whether cache refresh or process restart is required.

## When Not to Use `sys_config`

Do not put these into `sys_config`:

- Permission rules.
- Route definitions.
- Database schema meaning.
- Core state transitions.
- Scheduler handler/spec safety boundaries. Operational cron enable/disable state belongs in `task_definitions`.
- Rules where an admin-side edit would silently change core system behavior and make troubleshooting harder.

These should remain enforced by code constants, validators, service state machines, database constraints, and migrations.

## When to Use `sys_dict`

Use `sys_dict` from backend code to provide:

- Display-layer select options.
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
- `task_record_success_mode`: task success-detail policy labels.
- `task_detail_record_mode`: task detail-record mode labels.

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
- Sensitive config list responses and the `value` endpoint return only a masked placeholder. The `detail` endpoint returns the raw value for admin edit backfill. When an update receives the unchanged masked placeholder, backend code must keep the original value instead of writing the placeholder as the real config value.
- Runtime file-storage settings such as `storage.active_driver` and `storage.config` are still stored in `sys_config`, but they should be managed through the dedicated `/admin/v1/system/storage/*` endpoints: the driver is hidden but non-sensitive, the JSON payload is hidden and sensitive, saves must refresh the `sys_config` cache, and secret reveals go through `/admin/v1/system/storage/secret`.
- Menu-title i18n updates use Replace semantics: for the same `menu_id`, locale rows omitted from the current request are deleted and only submitted locales remain.
- i18n updates for system config names, dictionary type names, and dictionary item labels use PATCH semantics: only submitted locales are inserted or updated, and omitted locales keep their existing values.
- API documentation must explicitly state whether an entity uses Replace or PATCH semantics. "Supports i18n updates" is not specific enough.

## i18n Update Semantics

To prevent callers from accidentally deleting localized rows, the backend splits i18n update semantics into two categories. The table below reflects the current implementation in `internal/model/menu_i18n.go` and `internal/model/sys_i18n.go`, with unit tests in `internal/model/i18n_test.go`.

| Entity table | Update method | Semantics | Behavior |
| --- | --- | --- | --- |
| `menu_i18n` | `MenuI18n.UpsertMenuTitles` | Replace | For a given `menu_id`, locale rows that are not part of the current request are deleted inside the same transaction; only submitted locales survive. |
| `sys_config_i18n` | `SysConfigI18n.UpsertConfigNames` | PATCH | Only the submitted `(config_id, locale)` rows go through `ON CONFLICT DO UPDATE`; omitted locale rows are kept untouched. |
| `sys_dict_type_i18n` | `SysDictTypeI18n.UpsertTypeNames` | PATCH | Same as above, restricted to submitted `(dict_type_id, locale)`. |
| `sys_dict_item_i18n` | `SysDictItemI18n.UpsertLabels` | PATCH | Same as above, restricted to submitted `(dict_item_id, locale)`. |

All four methods run `strings.TrimSpace` on inputs before writing:

- If either the `locale` or the corresponding text field (`title` / `config_name` / `type_name` / `label`) becomes empty after `TrimSpace`, the row is silently skipped. It is neither inserted nor used to overwrite an existing value with an empty string.
- When every submitted locale is empty, the method returns early without touching the database. The menu Replace semantics will not destroy existing rows just because the caller submitted an empty map.

Caller integration guidance:

- The menu edit form must round-trip all locales. Any locale missing from the submitted payload is effectively a deletion, so the UI should clearly show which languages currently exist and which ones will be preserved.
- System config, dictionary type, and dictionary item edit forms may patch a subset of locales without resubmitting everything. To clear a locale, callers must use the matching `DeleteByXxxIDs` entry point instead of submitting an empty string.
- When the service layer needs Replace behavior on top of PATCH semantics, it must call the corresponding `DeleteByXxxIDs` explicitly before the Upsert. This responsibility must not be pushed down to the frontend.

Interface tests and caller-side acceptance should cover these facts at minimum:

- When a menu update omits a locale, that locale must disappear from later detail responses and database rows.
- When a system config, dictionary type, or dictionary item update omits a locale, that locale must keep its previous value in later detail responses and database rows.
- Submitting an empty string is not a supported way to clear a translation. The row is skipped instead; explicit delete flows must be used for locale removal.

## Display API Boundary

- `dict/options` returns display options only. It does not return backend business rules.
- Fields such as `status`, `is_*`, and task statuses may use dictionary labels for display, while legal values remain enforced by backend validators and model constants.
- When a dictionary type code is added, backend seed data, API documentation, and API tests must be updated together. Display fallback behavior belongs to the caller project's documentation.
- Backend business semantics must not change when dictionary APIs fail, dictionary items are edited incorrectly, or dictionary items are missing.

## Decision Table

| Need | Recommendation |
| --- | --- |
| Select options, table tags, colors, localized labels | Use `sys_dict` |
| Login lock limits, request-log masking fields, operational thresholds | Use `sys_config` |
| Validator enums, permission modes, state transitions | Use code constants and validators |
| Real cron enable/disable from admin UI | Use `task_definitions.status`; `cron` executes from an in-process cache, refreshes on admin-change notifications, and falls back to periodic reconcile. Handler safety still comes from the code registry, while `cron_spec` is a DB-owned operational setting |
| Backend needs runtime policy reads | Read `sys_config` through domain helpers and keep defaults |
| Display layer wants less enum hardcoding | Backend provides `dict/options`; caller fallbacks are maintained by the caller project |
