"""Health endpoint tests.

``/faces/health`` is how an operator confirms which execution provider is live
(CPU or CUDA) and reads the calibrated clustering thresholds the caller must
not re-implement.
"""

from __future__ import annotations

from conftest import FakeEngine
from fastapi.testclient import TestClient

from app.config import (
    CLUSTER_MIN_SIMILARITY,
    RECOGNITION_MARGIN,
    RECOGNITION_MIN_SIMILARITY,
)
from app.main import create_app


def test_health_returns_ok(client) -> None:
    body = client.get("/faces/health").json()

    assert body["status"] == "ok"
    assert body["thresholds"] == {
        "clusterMinSimilarity": CLUSTER_MIN_SIMILARITY,
        "recognitionMinSimilarity": RECOGNITION_MIN_SIMILARITY,
        "recognitionMargin": RECOGNITION_MARGIN,
    }


def test_health_reports_provider() -> None:
    engine = FakeEngine(
        providers=("cuda", "cpu"),
        available=("CUDAExecutionProvider", "CPUExecutionProvider"),
    )
    client = TestClient(create_app(engine=engine))

    body = client.get("/faces/health").json()

    assert body["status"] == "ok"
    assert body["activeProvider"] == "cuda"
    assert body["requestedProviders"] == ["cuda", "cpu"]


def test_health_reports_the_cpu_default(client) -> None:
    body = client.get("/faces/health").json()

    assert body["activeProvider"] == "cpu"
    assert body["requestedProviders"] == ["cpu"]
