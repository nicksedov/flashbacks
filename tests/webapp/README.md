# Webapp HTTP Integration Tests

Vitest suite that boots the real `core` Go service against the dedicated **test**
PostgreSQL database (`flashbacks-test`) and drives the webapp's HTTP API end to
end.

## Layout

```
tests/webapp/
├── integration/
│   ├── support.ts                # Boot the service, DB resets, HTTP client helpers
│   ├── bootstrap-login.test.ts   # Bootstrap login flow
│   ├── account-creation.test.ts  # self_service / self_service_with_approval / admin_only
│   └── gallery-folders.test.ts   # Gallery folders settings (add/list/remove)
└── tsconfig.json                 # Type-checking for the suite
```

The Vitest runner config lives in [`webapp/vitest.integration.config.ts`](../../webapp/vitest.integration.config.ts)
because that is where the `vitest` and `pg` packages are installed; it anchors
`test.root` at the repository root and points `include` at this directory.

## Requirements

- Go toolchain (the suite compiles `backend/core` with `go build`)
- `webapp` npm dependencies installed (`make setup-webapp`)
- A reachable PostgreSQL instance with the `flashbacks-test` database (see `test.env`)
- PostgreSQL superuser credentials for the one-off `vector` extension bootstrap

## Running

```bash
# Via Makefile
make test-webapp-integration

# Directly (from the webapp directory)
cd webapp && npm run test:integration

# A single file / test name
cd webapp && npx vitest run --config vitest.integration.config.ts account-creation
```

## Configuration

Values are read from the process environment first, then from the `.env` base
profile, then from the tracked `test.env` overlay, then from the defaults below.
Shared configuration lives in exactly one place:

```
.env        # base profile, git-ignored, created with `make env`
test.env    # tracked overlay, only the test-specific overrides
```

`test.env` currently overrides a single value (`DB_NAME=flashbacks-test`) — it
must never be a copy of `.env`.

| Variable | Default | Purpose |
|---|---|---|
| `DB_NAME` | `flashbacks-test` | Test database under test |
| `IT_ALLOW_NON_STAGING` | *(unset)* | Set to `1` to allow a non-staging `DB_NAME` |
| `POSTGRES_SUPERUSER` | `postgres` | Superuser used once to enable the `vector` extension |
| `POSTGRES_PASSWORD` | `DB_PASSWORD` | Password of that superuser |
| `IT_BOOTSTRAP_PASSWORD` | `ItBootstrap!2345` | Password applied by `bootstrap/setup` |
| `IT_BOOTSTRAP_DISPLAY_NAME` | `Integration Admin` | Display name applied by `bootstrap/setup` |

**Safety:** the suite refuses to run when the resolved database is the
production database (`flashbacks`). It truncates `users` (CASCADE) to force the
service back into bootstrap mode, so it must only ever target a dedicated test
database.

## Type-checking

```bash
cd webapp && npx tsc -p ../tests/webapp
```
