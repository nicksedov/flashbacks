# Proposal

## Why

The `faces` microservice shipped with code, specs and tests but with none of
its operational surfaces: the staging/production deploy skills still build only
`core`, `exif`, `ocr` and `registry`, no compose service exists for `faces`,
the seed configuration and local dev tooling ignore it, and the SDD definition
of done has no criterion that would catch the omission. The result is a service
that cannot be deployed at all, while `core` silently falls back to
`http://localhost:5173` inside its own container and the face-recognition job
fails on every image.

## What Changes

- Add a mandatory **§6.4 "New microservice → deployment artifacts"** step to
  [`docs/agent/sdd-workflow.md`](../../../docs/agent/sdd-workflow.md:1) with a
  concrete, per-file checklist for the staging deploy skill, the production
  deploy skill, local dev tooling and the shared documentation.
- Add a **deployment-assets completion criterion** to the workflow
  definition of done (§10) and a matching anti-pattern (§11).
- Introduce the new `service-deployment` capability spec that makes operational
  parity a specified behavior instead of tribal knowledge.
- Apply the new workflow to `faces`:
  - **tracked local dev**: `scripts/dev.sh`, `Makefile`, `scripts/bootstrap.sh`
    and `.env.example` start, build and configure `faces`;
  - **tracked image**: `backend/faces/Dockerfile` gains a non-root runtime and a
    real health probe;
  - **local operational assets (gitignored)**: the staging and production deploy
    skills gain a `faces` service — image build/publish, compose service with
    registry bootstrap + healthcheck + model-cache volume, consumer wiring and
    release-state/change-detection entries.
- Add a `pytest` evidence runner and a tracked `tests/deploy` suite that proves
  the workflow doc and local dev assets cover every backend service.

## Capabilities

### New Capabilities

- `service-deployment`: the operational contract for backend microservices —
  every service is buildable/publishable, present in the staging and production
  deploy assets, startable in local dev, and the SDD workflow forces those
  assets to be updated whenever a new microservice is introduced.

### Modified Capabilities

- (none)

## Non-goals

- No change to `faces` runtime, ML models or REST contracts.
- No new deployment environment or orchestration platform.
- No rewrite of the existing `core`/`exif`/`ocr` deploy behavior beyond adding
  `faces` consistently next to it.
- Committing the gitignored `.roo/` skill assets to version control — they stay
  local operational assets; the SDD workflow documents how to update them.

## Impact

- **Workflow/docs**: `docs/agent/sdd-workflow.md`, `openspec/specs/service-deployment/spec.md`.
- **Evidence**: `.focused-spec/config.yaml` (new `pytest` runner),
  `.focused-spec/evidence.yaml`, `tests/deploy/**`.
- **Local dev (tracked)**: `scripts/dev.sh`, `Makefile`, `scripts/bootstrap.sh`, `.env.example`.
- **Image (tracked)**: `backend/faces/Dockerfile`.
- **Deploy skills (local, gitignored)**: `.roo/skills/deploy-staging-backend/**`,
  `.roo/skills/deploy-production-backend/**`.
- **Consumers**: `core` already exposes `FACES_SERVICE_URL` and discovers the
  `faces` service; no core code change is required.
