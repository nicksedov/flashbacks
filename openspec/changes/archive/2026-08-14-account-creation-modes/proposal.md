## Why

Flashbacks currently supports only admin-created accounts (bootstrap setup + admin panel): there is no self-service signup and no approval workflow for new accounts. This change introduces a configurable account-creation mode and a pending/approval lifecycle so operators can choose between open registration, moderated registration with admin approval, or admin-only account creation.

## What Changes

- Add the `ACCOUNT_CREATION_MODE` environment variable with three values — `self_service`, `self_service_with_approval`, `admin_only` (default `admin_only`).
- Extend the user model with an approval lifecycle field `accountStatus` (`active` / `pending` / `rejected`), orthogonal to the existing `isActive` admin switch.
- Add a public `POST /api/auth/register` endpoint with mode-dependent behavior (auto-login, pending, or disabled).
- Extend `GET /api/auth/status` with `accountCreationMode` so the frontend can decide whether to show registration.
- Add admin-only `POST /api/admin/users/:id/approve` and `POST /api/admin/users/:id/reject`, plus a `status` filter on `GET /api/admin/users`.
- Extend `UserDTO` with `accountStatus`, `statusChangedAt`, `rejectionReason`; add audit actions for registration / approval / rejection.
- Add frontend registration form, a pending-approval screen, and an admin approval queue with per-user status badges.
- Refactor `AdminPanel.tsx` into focused components under `components/auth/admin/` (no observable behavior change).

## Capabilities

### New Capabilities

- `account-registration`: Account creation modes and the account approval lifecycle — registration, `pending`/`rejected`/`active` statuses, admin approve/reject, and the frontend registration and approval-queue flows.

### Modified Capabilities

<!-- none -->

## Impact

- Backend api-service: [`internal/domain/auth.go`](../backend/api-service/internal/domain/auth.go), config, auth application service, auth handlers, DTOs, audit actions, database migrations.
- Frontend webapp: [`LoginScreen.tsx`](../webapp/src/components/auth/LoginScreen.tsx), [`PendingApprovalScreen.tsx`](../webapp/src/components/auth/PendingApprovalScreen.tsx), [`AuthProvider.tsx`](../webapp/src/providers/AuthProvider.tsx), admin panel components under [`components/auth/admin/`](../webapp/src/components/auth/admin/).
- API contract: [`docs/api-contracts/api-service.yaml`](../docs/api-contracts/api-service.yaml) (new/changed endpoints and DTO fields).
- i18n: new en/ru keys for registration, approval, and login-status messages.
- Unaffected: exif, ocr, shared services; MCP contracts.
