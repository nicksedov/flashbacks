## Purpose

Defines how user accounts are created and approved, including a configurable creation mode, a pending/rejected/active approval lifecycle, self-service registration, admin approval, and the corresponding frontend flows.

## ADDED Requirements

### Requirement: Configurable account creation mode

The system SHALL support three account-creation modes configured via the `ACCOUNT_CREATION_MODE` environment variable: `self_service`, `self_service_with_approval`, and `admin_only`. The default SHALL be `admin_only`. The service MUST fail to start when the value is not one of the three allowed modes.

#### Scenario: Default mode is admin_only

- **WHEN** `ACCOUNT_CREATION_MODE` is not set
- **THEN** the service starts with account creation mode `admin_only`

#### Scenario: Invalid mode rejected at startup

- **WHEN** `ACCOUNT_CREATION_MODE` is set to an unsupported value
- **THEN** the service fails to start with an error identifying the invalid value

### Requirement: Account approval lifecycle

A user account SHALL have an `accountStatus` field with values `active`, `pending`, and `rejected`, and an `isActive` boolean switch. Access to the application SHALL be granted only when `isActive` is `true` AND `accountStatus` is `active`.

#### Scenario: Pending account cannot log in

- **WHEN** a user with `accountStatus` `pending` attempts to log in with valid credentials
- **THEN** login is refused with HTTP 403 and `accountStatus` stays `pending`

#### Scenario: Rejected account cannot log in

- **WHEN** a user with `accountStatus` `rejected` attempts to log in with valid credentials
- **THEN** login is refused with HTTP 403

#### Scenario: Deactivated account cannot log in

- **WHEN** a user with `isActive` `false` attempts to log in with valid credentials
- **THEN** login is refused with HTTP 401

#### Scenario: Access requires active and enabled

- **WHEN** a user with `accountStatus` `active` and `isActive` `true` attempts to log in
- **THEN** login succeeds and an authenticated session is established

### Requirement: Self-service registration endpoint

The system SHALL expose a public `POST /api/auth/register` endpoint that accepts `login`, `displayName`, and `password` and always creates accounts with role `user`. The response MUST depend on the active creation mode.

#### Scenario: Registration in self_service mode

- **WHEN** `ACCOUNT_CREATION_MODE` is `self_service` and a valid registration request is posted
- **THEN** the account is created with `accountStatus` `active`, an authenticated session is established (session cookie set), and the response is HTTP 201 with the user DTO

#### Scenario: Registration in self_service_with_approval mode

- **WHEN** `ACCOUNT_CREATION_MODE` is `self_service_with_approval` and a valid registration request is posted
- **THEN** the account is created with `accountStatus` `pending`, no session is established, and the response is HTTP 201 with a `pending` flag and message

#### Scenario: Registration in admin_only mode

- **WHEN** `ACCOUNT_CREATION_MODE` is `admin_only` and a registration request is posted
- **THEN** no account is created and the response is HTTP 403 with error `auth.registration_disabled`

#### Scenario: Registration during bootstrap

- **WHEN** no users exist in the database (bootstrap mode) and a registration request is posted
- **THEN** no account is created and the response is HTTP 409 with error `auth.bootstrap_mode`

### Requirement: Registration validation and security

Self-service registration SHALL hash the password with bcrypt, SHALL always assign role `user` regardless of any role value in the request, SHALL normalize the login by trimming and lowercasing it, SHALL enforce a case-insensitive unique login, and SHALL be rate-limited per IP address.

#### Scenario: Duplicate login rejected

- **WHEN** a registration request uses a login that already exists case-insensitively
- **THEN** the response is HTTP 400 with error `user_service.user_exists`

#### Scenario: Short password rejected

- **WHEN** a registration request uses a password shorter than the minimum length
- **THEN** the response is HTTP 400 with error `auth.password_length`

#### Scenario: Registration rate limited

- **WHEN** too many registration attempts come from the same IP address
- **THEN** further attempts return HTTP 429 with error `auth.rate_limited`

### Requirement: Auth status exposes creation mode

The system SHALL include `accountCreationMode` in the `GET /api/auth/status` response so the frontend can decide whether to show registration.

#### Scenario: Status returns creation mode

- **WHEN** `GET /api/auth/status` is called
- **THEN** the response JSON contains `accountCreationMode` matching the active mode

### Requirement: Admin approval and rejection endpoints

The system SHALL expose admin-only `POST /api/admin/users/:id/approve` and `POST /api/admin/users/:id/reject` endpoints, protected by admin authorization and CSRF. Approval and rejection MUST apply only to `pending` accounts.

#### Scenario: Approve a pending account

- **WHEN** an admin approves a `pending` user
- **THEN** the account becomes `active` and the response contains the updated user DTO

#### Scenario: Reject a pending account with reason

- **WHEN** an admin rejects a `pending` user with a body `{ "reason": "..." }`
- **THEN** the account becomes `rejected` with `rejectionReason` set and the response contains the updated user DTO

#### Scenario: Approve or reject a non-pending account

- **WHEN** an admin attempts to approve or reject a user whose `accountStatus` is not `pending`
- **THEN** the response is HTTP 409 with error `auth.user_not_pending`

#### Scenario: Reject without reason

- **WHEN** an admin rejects a `pending` user without a `reason`
- **THEN** the response is HTTP 400 with error `auth.invalid_request_format`

### Requirement: Admin user list status filter

The system SHALL support an optional `status` query parameter on `GET /api/admin/users` to filter users by `accountStatus`.

#### Scenario: Filter pending users

- **WHEN** an admin calls `GET /api/admin/users?status=pending`
- **THEN** only users with `accountStatus` `pending` are returned

### Requirement: User DTO account status fields

The `UserDTO` returned by user-related endpoints SHALL include `accountStatus`, `statusChangedAt`, and `rejectionReason` (camelCase JSON fields), aligned with the Go JSON tags and webapp TypeScript types.

#### Scenario: User DTO includes status fields

- **WHEN** a user object is serialized in an API response
- **THEN** the JSON contains `accountStatus`, `statusChangedAt`, and `rejectionReason`

### Requirement: Login failure message disclosure

The system MUST reveal `pending`/`rejected`/`deactivated` login failure reasons only after the password has been verified, to avoid disclosing account existence, and MUST use distinct messages rather than a generic invalid-credentials error.

#### Scenario: Pending login failure

- **WHEN** a `pending` user submits correct credentials
- **THEN** the response is HTTP 403 with error `auth.account_pending_approval`

#### Scenario: Rejected login failure

- **WHEN** a `rejected` user submits correct credentials
- **THEN** the response is HTTP 403 with error `auth.account_rejected`

#### Scenario: Deactivated login failure

- **WHEN** a deactivated (`isActive` `false`) user submits correct credentials
- **THEN** the response is HTTP 401 with error `auth.account_deactivated`

#### Scenario: Wrong password hides status

- **WHEN** a user submits an incorrect password
- **THEN** the response is the generic invalid-credentials error and does NOT reveal the `pending`/`rejected`/`deactivated` status

### Requirement: Audit of account lifecycle actions

The system SHALL record audit entries for `register_user`, `approve_user`, and `reject_user` actions.

#### Scenario: Audit entries recorded

- **WHEN** a registration, approval, or rejection action occurs
- **THEN** an audit entry with the corresponding action type is recorded

### Requirement: Data migration compatibility

Existing user accounts SHALL remain able to log in after the new status fields are introduced, with a default `accountStatus` of `active`, and a case-insensitive unique index on login SHALL be created.

#### Scenario: Existing users become active

- **WHEN** the schema is migrated with existing users
- **THEN** all existing users have `accountStatus` `active` and can log in unchanged

#### Scenario: Case-insensitive unique login index

- **WHEN** the migration runs after any pre-existing case-conflicting logins have been resolved
- **THEN** a unique index on `lower(login)` exists

### Requirement: Mode switching behavior

Changing `ACCOUNT_CREATION_MODE` SHALL affect only new registrations and MUST NOT auto-migrate existing account statuses.

#### Scenario: Switching mode does not alter existing accounts

- **WHEN** the service restarts with a different `ACCOUNT_CREATION_MODE`
- **THEN** existing accounts and pending requests keep their current status, and only the behavior of new registrations changes

### Requirement: Frontend registration flow

The frontend SHALL show registration UI only when `accountCreationMode` is not `admin_only`, SHALL collect `login`, `displayName`, `password`, and password confirmation, SHALL auto-login after `self_service` registration, and SHALL show a pending-approval screen after `self_service_with_approval` registration.

#### Scenario: Self-service registration auto-login

- **WHEN** a user registers while the mode is `self_service`
- **THEN** the user is authenticated immediately and enters the application

#### Scenario: Approval registration shows pending screen

- **WHEN** a user registers while the mode is `self_service_with_approval`
- **THEN** the user is shown a pending-approval screen and is not authenticated

#### Scenario: Admin-only hides registration

- **WHEN** the mode is `admin_only`
- **THEN** no registration link or form is shown on the auth screen

### Requirement: Admin approval queue UI

The admin panel SHALL display a queue of `pending` registration requests with login, display name, request date, and Approve/Reject actions, SHALL show a status badge (`active`/`pending`/`rejected`) per user, SHALL allow rejecting with an optional reason, and SHALL allow re-approving a `rejected` account.

#### Scenario: Admin sees pending queue

- **WHEN** an admin opens the admin panel with pending requests
- **THEN** the pending requests are listed with Approve and Reject actions

#### Scenario: Reject with reason

- **WHEN** an admin rejects a request
- **THEN** a dialog prompts for an optional reason before confirming

#### Scenario: Status badges

- **WHEN** the user list is displayed
- **THEN** each user shows a badge for `active`, `pending`, or `rejected`

#### Scenario: Re-approve a rejected account

- **WHEN** an admin approves a `rejected` account
- **THEN** the account becomes `active`

### Requirement: Localized user-facing text

All new user-facing text SHALL be provided in both English and Russian, in sync.

#### Scenario: en/ru parity

- **WHEN** the frontend displays a new registration or approval message
- **THEN** both English and Russian translations exist and are in sync

The following key strings MUST exist in both locales:

| Context | English (en) | Russian (ru) |
|---|---|---|
| Pending screen title | Account pending approval | Учётная запись ожидает одобрения |
| Pending screen body | Your account is awaiting administrator approval. | Ваша учётная запись ожидает одобрения администратора. |
| Rejected login message | Your account has been rejected. | Ваша учётная запись отклонена. |
| Approve action | Approve | Утвердить |
| Reject action | Reject | Отклонить |
| Status badge active | Active | Активна |
| Status badge pending | Pending | Ожидает |
| Status badge rejected | Rejected | Отклонена |
