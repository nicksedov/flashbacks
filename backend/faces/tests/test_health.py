"""Health endpoint tests.

``/faces/health`` is how an operator confirms which execution provider is live
(CPU or CUDA) and reads the calibrated clustering thresholds the caller must
not re-implement.
"""

from __future__ import annotations

from conftest import FakeEngine
from fastapi.testclient import TestClient

from app.config import (
    CLUSTER_ALGO,
    CLUSTER_ASSIGN_SIMILARITY,
    CLUSTER_CANDIDATES_PER_PHOTO,
    CLUSTER_CORE_NEIGHBOURS,
    CLUSTER_CORE_SIMILARITY,
    CLUSTER_MERGE_SIMILARITY,
    CLUSTER_MERGE_SUPPORT,
    CLUSTER_MERGE_SUPPORT_SIMILARITY,
    CLUSTER_MIN_PHOTOS,
    CLUSTER_MIN_SIMILARITY,
    MIN_CLUSTERABLE_FACE_PX,
    RECOGNITION_MARGIN,
    RECOGNITION_MIN_SIMILARITY,
    SMALL_FACE_WIDTH,
)
from app.main import create_app


def test_health_returns_ok(client) -> None:
    body = client.get("/faces/health").json()

    assert body["status"] == "ok"
    assert body["thresholds"] == {
        "clusterMinSimilarity": CLUSTER_MIN_SIMILARITY,
        "recognitionMinSimilarity": RECOGNITION_MIN_SIMILARITY,
        "recognitionMargin": RECOGNITION_MARGIN,
        # The size floors travel with the similarity floors: core needs them to
        # skip clustering a face that can never be assigned and to detach the
        # ones already attached.
        "minClusterableFacePx": MIN_CLUSTERABLE_FACE_PX,
        "weakFacePx": SMALL_FACE_WIDTH,
        "clusterAlgo": CLUSTER_ALGO,
        "clusterCoreSimilarity": CLUSTER_CORE_SIMILARITY,
        "clusterCoreNeighbours": CLUSTER_CORE_NEIGHBOURS,
        "clusterAssignSimilarity": CLUSTER_ASSIGN_SIMILARITY,
        "clusterMergeSimilarity": CLUSTER_MERGE_SIMILARITY,
        "clusterMergeSupport": CLUSTER_MERGE_SUPPORT,
        "clusterMergeSupportSimilarity": CLUSTER_MERGE_SUPPORT_SIMILARITY,
        "clusterCandidatesPerPhoto": CLUSTER_CANDIDATES_PER_PHOTO,
        "clusterMinPhotos": CLUSTER_MIN_PHOTOS,
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
    assert body["devices"] == [{"deviceId": 0, "provider": "cuda"}]


def test_health_reports_the_cpu_default(client) -> None:
    body = client.get("/faces/health").json()

    assert body["activeProvider"] == "cpu"
    assert body["requestedProviders"] == ["cpu"]
