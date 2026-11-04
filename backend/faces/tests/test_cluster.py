"""Clustering endpoint tests."""

from __future__ import annotations

import numpy as np

from app.engine import EMBEDDING_DIM


def _identity_embeddings() -> tuple[list[list[float]], list[list[float]]]:
    """Build two deterministic identity clusters (six embeddings each)."""
    rng = np.random.default_rng(42)
    dim = EMBEDDING_DIM

    base_a = np.zeros(dim)
    base_a[0] = 1.0
    base_b = np.zeros(dim)
    base_b[1] = 1.0

    group_a = []
    group_b = []
    for _ in range(6):
        group_a.append((base_a + rng.normal(0, 0.02, dim)).tolist())
        group_b.append((base_b + rng.normal(0, 0.02, dim)).tolist())
    return group_a, group_b


def test_cluster_groups_same_identity(client) -> None:
    group_a, group_b = _identity_embeddings()
    response = client.post(
        "/faces/cluster",
        json={"embeddings": group_a + group_b, "minClusterSize": 2},
    )
    assert response.status_code == 200
    labels = response.json()["labels"]

    # The same-identity embeddings must share one non-noise label.
    first_label = labels[0]
    assert first_label != -1
    assert all(label == first_label for label in labels[:6])
    # The other identity must form a different, non-noise label.
    second_label = labels[6]
    assert second_label != -1
    assert second_label != first_label
    assert all(label == second_label for label in labels[6:])
