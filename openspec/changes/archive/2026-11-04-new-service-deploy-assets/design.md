# Design

## Context

See [`proposal.md`](proposal.md) for motivation. Constraints that shape the
approach:

- **Deployment assets are gitignored.** `.roo/` is listed in
  [`.gitignore`](../../../.gitignore:151) and has zero tracked files, so the
  staging/production skills and their `deploy/` assets live only on the
  operator host. They cannot appear in a tracked change diff, and automated
  evidence committed to the repository cannot depend on them.
- **Two build models coexist.** `core`/`exif`/`ocr`/`registry` are Go binaries
  built from the shared [`backend/Dockerfile`](../../../backend/Dockerfile:1)
  with `--target <service>` and a `backend/` context. `faces` is a Python
  service with its own [`backend/faces/Dockerfile`](../../../backend/faces/Dockerfile:1)
  and a `backend/faces/` context.
- **`faces` is already a first-class runtime peer.** `core` reads
  `FACES_SERVICE_URL` with a `http://localhost:5173` default and discovers the
  registry service name `faces`; the faces app self-registers and exposes
  `/faces/health`.
- **The evidence toolchain is registered per runner.** `.focused-spec/config.yaml`
  currently registers `vitest`, `go` and `goregistry`, but not `pytest`, while
  `@focused-spec/runner-pytest` is already a dev dependency.

## Goals / Non-Goals

**Goals:**

- Make "a new microservice must update its deployment and local-dev surfaces" a
  checked workflow rule, not tribal knowledge.
- Wire `faces` into every tracked surface that can be verified in CI-like runs.
- Apply the workflow to the gitignored deploy skills as an operational task
  verified by runnable commands.

**Non-Goals:**

- Making `.roo/` version-controlled.
- Changing faces ML/runtime behavior or the core→faces REST contract.
- Pre-baking InsightFace models into the image (see Risks).

## Decisions

### D1 — Put the rule in the workflow doc; prove it with a tracked pytest suite

The behavioral contract for "new microservice → deploy assets" lives in
[`docs/agent/sdd-workflow.md`](../../../docs/agent/sdd-workflow.md:1) as a new
§6.4 step, a §10 definition-of-done bullet and a §11 anti-pattern. A tracked
`tests/deploy/test_service_deployment.py` asserts the step and the criterion
are present.

*Alternative considered:* enforce only via review (rejected — the faces gap
proved review misses it) or via a lint script (rejected — a doc-content pytest
is simpler and uses the existing evidence mechanism).

### D2 — Register the official `pytest` runner instead of inventing one

`.focused-spec/config.yaml` gains a `pytest` entry using the already-installed
`@focused-spec/runner-pytest`. This keeps the evidence toolchain official and
lets the deploy-parity checks live in `tests/deploy/` next to the other
integration suites (`tests/ocr`, `tests/webapp`).

*Alternative considered:* assert on files from a Go test in `backend/core`
(rejected — a Python service's operational checks do not belong in the core Go
module) or a Vitest test in `webapp` (rejected — same reason).

### D3 — Per-service build spec in the deploy scripts

Both deploy scripts gain a `service_build_spec` mapping returning
`context|dockerfile|target`:

- `faces` → `backend/faces|backend/faces/Dockerfile|` (no `--target`)
- every other service → `backend|backend/Dockerfile|<service>`

This keeps the shared Go Dockerfile untouched and lets the Python image use its
own multi-stage build. `VALID_SERVICES` and the consumer lists gain `faces`.

*Alternative considered:* add a `faces` stage to `backend/Dockerfile` (rejected
— mixing a Python runtime stage into the Go build file couples two toolchains
and breaks the `--target` convention).

### D4 — Faces compose service: internal + outbound, registry bootstrap, health gate

The compose service joins `backend` (registry/peer traffic) and `outbound`
(InsightFace model download on first inference), receives the shared
`x-registry-client` bootstrap, advertises `SERVICE_HOST`, and defines a
`python`-based `/faces/health` probe because `python:slim` has no `curl`. It
depends on the healthy registry; `core` gains a `service_healthy` dependency on
`faces` so discovery resolves it at boot.

### D5 — Image hardening: non-root user, `/home/app` home, model-cache volume

[`backend/faces/Dockerfile`](../../../backend/faces/Dockerfile:1) creates an
unprivileged user, sets `HOME=/home/app` (InsightFace writes models under
`~/.insightface`), declares `HEALTHCHECK` over `/faces/health`, and the compose
service mounts a named volume at `/home/app/.insightface` so models survive
container recreation.

### D6 — Tracked local-dev parity

`scripts/dev.sh` gains `faces` in `ALL_SERVICES` plus `svc_dir`/`svc_port`/
`svc_health_path`/`svc_public_url` cases and an `uvicorn` start branch;
`Makefile` gains `build-faces`/`run-faces`/`dev-faces`/`test-faces`/
`setup-faces`; `scripts/bootstrap.sh` gains a `faces` venv target; and
`.env.example` documents `FACES_SERVICE_URL`/`FACES_PORT`.

## Risks / Trade-offs

- **[Untracked skills cannot be diff-reviewed]** → The workflow step enumerates
  the exact files; the apply report records the operational verification
  commands (`docker compose config -q`, `bash -n`, `--dry-run`) for `.roo/`.
- **[First inference downloads models and can exceed core's 60 s client
  timeout]** → The named volume avoids repeated downloads; pre-baking or a
  startup warm-up is a documented follow-up, not part of this change.
- **[`faces` becomes a hard startup dependency of `core`]** → It is gated on a
  cheap health probe (the engine loads lazily), and a failed faces start is
  visible in `docker compose ps` instead of silent per-image errors.
- **[The gitignored deploy skills are not covered by committed evidence]** →
  The workflow doc (tracked, evidence-backed) enumerates the exact skill files
  to update, and tasks 5.1–5.6 verify each edit with a runnable command
  (`docker compose config -q`, `bash -n`, `--dry-run`) recorded in the apply
  report; the spec deliberately scopes evidence to tracked artifacts.
- **[Adding a runner changes the evidence toolchain]** → It is the official
  adapter already pinned in `package.json`; no new download is required.

## Migration Plan

1. Update `docs/agent/sdd-workflow.md` (§6.4, §10, §11) — tracked.
2. Update tracked local-dev files and `backend/faces/Dockerfile` — tracked.
3. Update the gitignored staging/production skills (compose, scripts, SKILL.md)
   — operational, verified by `docker compose config` / `bash -n` / `--dry-run`.
4. Add `.focused-spec` pytest runner + `tests/deploy` suite — tracked.
5. Rollback: revert the tracked diff; the deploy skills can be reverted from the
   operator host backup. `faces` remains additive, so no data migration exists.
