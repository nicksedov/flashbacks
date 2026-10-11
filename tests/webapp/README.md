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
│   └── account-creation.test.ts  # self_service / self_service_with_approval / admin_only
└── tsconfig.json                 # Type-checking for the suite
```

The Vitest runner config lives in [`webapp/vitest.integration.config.ts`](../../webapp/vitest.integration.config.ts)
because that is where the `vitest` and `pg` packages are installed; it anchors
`test.root` at the repository root and points `include` at this directory.

## Requirements

- Go toolchain (the suite compiles `backend/core` with `go build`)
- `webapp` npm dependencies installed (`make setup-webapp`)
- A reachable PostgreSQL instance with the `flashbacks-test` database (see `.env`)
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

Configuration is read from the process environment first, then from the
repository `.env`, then from development defaults.

| Variable | Default | Purpose |
|---|---|---|
| `IT_DB_NAME` | `DB_NAME` from `.env` (`flashbacks-test`) | Test database under test |
| `IT_ALLOW_NON_STAGING` | *(unset)* | Set to `1` to allow a non-staging `DB_NAME` |
| `POSTGRES_SUPERUSER` | `postgres` | Superuser used once to enable the `vector` extension |
| `POSTGRES_PASSWORD` | `DB_PASSWORD` from `.env` | Password of that superuser |
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
