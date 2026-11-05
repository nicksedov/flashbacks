"""Calibration invariants between verification and clustering thresholds.

The recognition floor and the clustering floors are all raw cosine similarities
over the same ArcFace embeddings, so their order matters: a clustering floor
below the recognition floor would let a pair count as the same person for
verification and not for clustering. The LFW calibration may raise
``RECOGNITION_MIN_SIMILARITY`` only up to the lowest clustering floor.

``CLUSTER_CORE_SIMILARITY`` is deliberately absent: ``graph-v2`` compares
hub-corrected (CSLS) scores, where a pair sitting at its neighbourhood mean
scores 0, so that constant lives on a different scale and cannot be ordered
against a raw cosine.
"""

from __future__ import annotations

from app.config import (
    CLUSTER_ASSIGN_SIMILARITY,
    CLUSTER_MERGE_SIMILARITY,
    CLUSTER_MERGE_SUPPORT_SIMILARITY,
    CLUSTER_MIN_SIMILARITY,
    RECOGNITION_MIN_SIMILARITY,
)

#: Raw-cosine clustering floors, aligned with ``cluster-graph`` behaviour.
CLUSTERING_FLOORS: dict[str, float] = {
    "CLUSTER_MIN_SIMILARITY": CLUSTER_MIN_SIMILARITY,
    "CLUSTER_ASSIGN_SIMILARITY": CLUSTER_ASSIGN_SIMILARITY,
    "CLUSTER_MERGE_SIMILARITY": CLUSTER_MERGE_SIMILARITY,
    "CLUSTER_MERGE_SUPPORT_SIMILARITY": CLUSTER_MERGE_SUPPORT_SIMILARITY,
}


def test_no_clustering_floor_sits_below_the_recognition_threshold() -> None:
    for name, floor in CLUSTERING_FLOORS.items():
        assert floor >= RECOGNITION_MIN_SIMILARITY, (
            f"{name} ({floor}) is below RECOGNITION_MIN_SIMILARITY "
            f"({RECOGNITION_MIN_SIMILARITY})"
        )
