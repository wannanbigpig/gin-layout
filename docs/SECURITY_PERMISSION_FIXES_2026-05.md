# Security and Permission Fixes - 2026-05

## Scope

- Backend project: `go-layout`
- Frontend project: `x-l-admin-vue3`
- Goal: keep existing business behavior, fix permission/token consistency risks, and improve readability around high-risk flows.

## Backend Changes

- User token revocation now runs after user mutation transactions commit successfully, avoiding blacklist state that is newer than database state.
- Disabled users can no longer refresh tokens.
- Token validation checks database revocation records when Redis reports a miss, reducing the window caused by cache loss or Redis flushes.
- Menu and role deletion now clean related mappings and synchronize affected user permissions inside the same transaction.
- API permission updates refresh route cache and synchronize affected user permissions after successful transaction commit.
- Menu uniqueness checks only allow known fields, preventing accidental dynamic column misuse.
- Admin user phone updates now validate and persist the final `full_phone_number` value, including clearing the value when phone number is cleared.

## Frontend Changes

- Login redirect targets are normalized through a shared helper to prevent external redirect injection.
- Auth store reset now removes the correct persisted key and guards stale `refreshUserInfo` responses.
- Route generation rejects duplicate route names and paths, and missing components fall back to a controlled `NotFound` component.
- Permission directive defaults unauthorized elements to hidden state and avoids duplicate disabled-click bindings.
- Sensitive admin-user field reveal now has per-row loading state, failure rollback, and a short-lived row cache.
- Blob JSON API errors now go through normal response handling, so 401 and business errors behave consistently.
- Upload requests no longer force `Content-Type`; the browser can attach the correct multipart boundary.
- Menu button form submission no longer silently overwrites `is_show`.

## Verification

- Backend: `go test ./...`
- Frontend: `npm run type-check`
- Frontend: `npm test -- --run`
- Frontend: `npm run lint`
- Frontend: `npm run build:production`

## Notes

- No intentional business behavior changes were introduced.
- Permission cache synchronization still depends on the existing coordinator and policy reload mechanisms.
