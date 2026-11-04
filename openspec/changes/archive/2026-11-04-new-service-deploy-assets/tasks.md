# Tasks

## 1. Evidence toolchain

- [x] 1.1 Register the official `pytest` runner in `.focused-spec/config.yaml` alongside `vitest`/`go`/`goregistry`; verify `node_modules/.bin/focused-spec validate --syntax-only` loads the configuration without a runner/config error
- [x] 1.2 Create the `tests/deploy` pytest package with a shared repo-root helper and `pytest.ini`; verify `cd <repo-root> && python -m pytest tests/deploy -q` collects with no import errors

## 2. SDD workflow documentation

- [x] 2.1 Add §6.4 "New microservice → deployment artifacts" to `docs/agent/sdd-workflow.md` with the concrete per-file checklist (staging skill `SKILL.md`/compose/seed/seed-script/deploy-script, production skill plus release state, faces-style dedicated Dockerfile, `scripts/dev.sh`, `Makefile`, `scripts/bootstrap.sh`, `.env.example`, `docs/agent/go-services.md`, consumer `*_SERVICE_URL` + registry discovery) and the verification commands; verify the section text is present with `grep -n "§6.4" docs/agent/sdd-workflow.md`
- [x] 2.2 Add the deployment-assets criterion to §10 Definition of done and the matching anti-pattern to §11; verify both anchors are present in `docs/agent/sdd-workflow.md`
- [x] 2.3 Add `tests/deploy/test_service_deployment.py::test_workflow_has_deploy_step` and `::test_workflow_dod_has_deploy_criterion`; verify `python -m pytest tests/deploy -q` passes

## 3. Local dev parity

- [x] 3.1 Add `faces` to `scripts/dev.sh` (`ALL_SERVICES`, `svc_dir`, `svc_port`, `svc_health_path`, `svc_public_url`, `uvicorn` start branch, `FACES_SERVICE_URL`/`FACES_PORT` defaults and export); verify `bash -n scripts/dev.sh` and `scripts/dev.sh status faces`
- [x] 3.2 Add the faces service section and `FACES_SERVICE_URL` to `.env.example`; verify the keys are present with `grep -n "FACES_SERVICE_URL" .env.example`
- [x] 3.3 Add `build-faces`, `run-faces`, `dev-faces`, `test-faces`, `setup-faces` targets to `Makefile` and its `.PHONY`; verify `make -n build-faces` and `make -n test-faces`
- [x] 3.4 Add a `faces` target to `scripts/bootstrap.sh` that creates `backend/faces/.venv` and installs `-e ".[dev]"`; verify `bash -n scripts/bootstrap.sh`
- [x] 3.5 Add `tests/deploy/test_service_deployment.py::test_dev_stack_manages_faces` and `::test_make_has_faces_targets`; verify `python -m pytest tests/deploy -q` passes

## 4. Faces image hardening

- [x] 4.1 Update `backend/faces/Dockerfile` to run as a non-root user with `HOME` under the app home, create the InsightFace model directory and declare a `HEALTHCHECK` over `/faces/health`; verify the static assertions in 4.2 pass (a full `docker build backend/faces` is exercised on the deploy host)
- [x] 4.2 Add `tests/deploy/test_service_deployment.py::test_faces_image_healthcheck_and_user`; verify `python -m pytest tests/deploy -q` passes

## 5. Deploy skills (local, gitignored)

- [x] 5.1 Add the `flashbacks-faces-staging` service to `.roo/skills/deploy-staging-backend/deploy/docker-compose.yml` (registry bootstrap, `backend`+`outbound` networks, `python` healthcheck, model-cache volume) and a `faces_models` volume, and add a `service_healthy` dependency from core; verify `docker compose --env-file .roo/skills/deploy-staging-backend/deploy/flashbacks-staging.env -f .roo/skills/deploy-staging-backend/deploy/docker-compose.yml config -q`
- [x] 5.2 Extend `.roo/skills/deploy-staging-backend/scripts/deploy-staging-backend.sh` with `faces` in `VALID_SERVICES`, a per-service build spec (`backend/faces/Dockerfile`, no target), the consumer/wait/log lists and the usage text; verify `bash -n` and `... --dry-run --no-push` lists the faces image
- [x] 5.3 Update `.roo/skills/deploy-staging-backend/SKILL.md` images table, deployment assets and prerequisites for `faces`; verify the faces rows are present
- [x] 5.4 Add the `flashbacks-faces` service to `.roo/skills/deploy-production-backend/deploy/docker-compose.yml` with `${FACES_IMAGE_TAG:-latest}`, the registry bootstrap, the healthcheck, the model-cache volume and the core dependency; verify `docker compose ... config -q`
- [x] 5.5 Extend `.roo/skills/deploy-production-backend/scripts/deploy-production-backend.sh` with `faces` in `VALID_SERVICES`, the change-detection path map, `compose_name`, the per-service build spec and the consumer/wait lists; verify `bash -n` and `... --dry-run --no-push` shows the faces version plan
- [x] 5.6 Update `.roo/skills/deploy-production-backend/SKILL.md` images/versioning/change-detection/rollback sections for `faces`; verify the faces rows are present

## 6. Integration checks

- [x] 6.1 Run `node_modules/.bin/focused-spec validate --strict` and `run` for every `service-deployment.*` scope and record the per-scope results and mutation-check pairs
- [x] 6.2 Run `python -m pytest backend/faces/tests -q`, `bash -n scripts/dev.sh`, `bash -n scripts/bootstrap.sh`, `make -n test-faces`, and `docker compose config -q` for both deploy skills; record the results
- [x] 6.3 Run `openspec validate --specs` after the delta is synced and record the result

## Workflow follow-up

- Sync the delta spec into `openspec/specs/service-deployment/spec.md` and archive the change after review.
