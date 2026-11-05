"""Contract tests for the clustering endpoints.

The service accepts per-face ``photoIds``/``widths``/``qualities`` aligned with
the face ids. A payload without them is still valid: the service falls back to
one unique photo per face so an older core keeps today's behaviour.
"""

from __future__ import annotations

from conftest import FakeEngine
from fastapi.testclient import TestClient

from app.config import FaceSettings
from app.main import create_app


def _v2_client() -> TestClient:
    return TestClient(
        create_app(engine=FakeEngine(), settings=FaceSettings(cluster_algo="graph-v2"))
    )


def _mutual(first: int, second: int, similarity: float) -> list[dict]:
    return [
        {"faceId": first, "candidateId": second, "similarity": similarity},
        {"faceId": second, "candidateId": first, "similarity": similarity},
    ]


def test_cluster_graph_accepts_aligned_face_facts() -> None:
    client = _v2_client()

    response = client.post(
        "/faces/cluster-graph",
        json={
            "faceIds": [1, 2],
            "photoIds": [10, 11],
            "widths": [120.0, 80.0],
            "qualities": [0.9, 0.8],
            "edges": _mutual(1, 2, 0.9),
        },
    )

    assert response.status_code == 200
    labels = response.json()["labels"]
    assert labels[0] == labels[1] != -1, labels


def test_cluster_graph_legacy_payload_without_photo_ids(client) -> None:
    """The payload an older core sends keeps clustering the same faces."""
    response = client.post(
        "/faces/cluster-graph",
        json={"faceIds": [1, 2], "edges": _mutual(1, 2, 0.9)},
    )

    assert response.status_code == 200
    labels = response.json()["labels"]
    assert labels[0] == labels[1] != -1, labels


def test_cluster_accepts_aligned_face_facts() -> None:
    client = _v2_client()

    response = client.post(
        "/faces/cluster",
        json={
            "embeddings": [[1.0, 0.0], [1.0, 0.0]],
            "photoIds": [10, 11],
            "widths": [120.0, 80.0],
            "qualities": [0.9, 0.8],
        },
    )

    assert response.status_code == 200
    labels = response.json()["labels"]
    assert labels[0] == labels[1] != -1, labels


def test_cluster_legacy_payload_without_photo_ids(client) -> None:
    response = client.post("/faces/cluster", json={"embeddings": [[1.0, 0.0], [1.0, 0.0]]})

    assert response.status_code == 200
    labels = response.json()["labels"]
    assert labels[0] == labels[1] != -1, labels


def test_health_reports_the_configured_algorithm() -> None:
    client = _v2_client()

    thresholds = client.get("/faces/health").json()["thresholds"]

    assert thresholds["clusterAlgo"] == "graph-v2"
