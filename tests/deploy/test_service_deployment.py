"""Operational-contract checks for backend microservices.

These tests assert the tracked artifacts that keep a backend microservice
deployable and runnable: the SDD workflow instructions, the host-native dev
tooling and the service image. They are the executable evidence for the
`service-deployment` capability.

The gitignored `.roo/` deploy skills are deliberately not asserted here: they
are operator-local assets, verified by their own commands during apply.
"""

from __future__ import annotations

import re
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = "docs/agent/sdd-workflow.md"


def _read(relative: str) -> str:
    return (REPO_ROOT / relative).read_text(encoding="utf-8")


def test_workflow_has_deploy_step() -> None:
    workflow = _read(WORKFLOW)

    assert re.search(r"^###\s+6\.4\b.*deploy", workflow, re.MULTILINE | re.IGNORECASE), (
        "docs/agent/sdd-workflow.md must contain a §6.4 deployment-artifacts step"
    )
    assert "deploy-staging-backend" in workflow, "the step must name the staging deploy skill"
    assert "deploy-production-backend" in workflow, "the step must name the production deploy skill"
    assert "scripts/dev.sh" in workflow, "the step must name the local dev tooling"
    assert "Dockerfile" in workflow, "the step must name the service image definition"


def test_workflow_dod_has_deploy_criterion() -> None:
    workflow = _read(WORKFLOW)

    assert "## 10. Definition of done" in workflow
    dod = workflow.split("## 10. Definition of done", 1)[1].split("## 11.", 1)[0]
    criterion = [line for line in dod.splitlines() if "- [ ]" in line and "deployment" in line.lower()]

    assert criterion, "definition of done must contain a deployment-assets criterion"


def test_dev_stack_manages_faces() -> None:
    dev = _read("scripts/dev.sh")

    services_line = next((line for line in dev.splitlines() if line.startswith("ALL_SERVICES=")), "")
    assert "faces" in services_line, "ALL_SERVICES must include faces"
    assert "/faces/health" in dev, "dev.sh must know the faces health path"
    assert re.search(r"^\s*faces\)", dev, re.MULTILINE), "dev.sh must have a faces case branch"
    assert "FACES_SERVICE_URL" in dev, "dev.sh must default the faces peer URL"

    example = _read(".env.example")
    assert "FACES_SERVICE_URL" in example, ".env.example must document FACES_SERVICE_URL"
    assert "FACES_PORT" in example, ".env.example must document FACES_PORT"


def test_make_has_faces_targets() -> None:
    makefile = _read("Makefile")

    for target in ("build-faces", "run-faces", "dev-faces", "test-faces", "setup-faces"):
        assert re.search(rf"^{re.escape(target)}:", makefile, re.MULTILINE), (
            f"Makefile must define the {target} target"
        )


def test_faces_image_healthcheck_and_user() -> None:
    dockerfile = _read("backend/faces/Dockerfile")

    assert "HEALTHCHECK" in dockerfile, "the faces image must declare a HEALTHCHECK"
    assert "/faces/health" in dockerfile, "the healthcheck must probe /faces/health"
    assert re.search(r"^USER\s+(?!root\b)\S+", dockerfile, re.MULTILINE), (
        "the faces image must run as a non-root user"
    )
    assert re.search(r"\b(useradd|adduser)\b", dockerfile), "the image must create the runtime user"
