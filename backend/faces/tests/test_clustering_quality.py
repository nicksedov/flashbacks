"""Hermetic clustering-quality regressions for the density-aware variant.

These cases use synthetic candidate graphs on purpose: they are the behavioural
gate.
"""

from __future__ import annotations

from app.engine import FaceEngine


def _mutual(
    pairs: list[tuple[int, int]], similarity: float
) -> list[tuple[int, int, float]]:
    edges: list[tuple[int, int, float]] = []
    for left, right in pairs:
        edges.append((left, right, similarity))
        edges.append((right, left, similarity))
    return edges


def _complete(faces: list[int], similarity: float) -> list[tuple[int, int, float]]:
    pairs = [
        (left, right)
        for index, left in enumerate(faces)
        for right in faces[index + 1 :]
    ]
    return _mutual(pairs, similarity)


def test_distinct_identities_stay_separate() -> None:
    """Two identities with no candidate link between them never share a person."""
    first = [1, 2, 3, 4, 5]
    second = [11, 12, 13, 14, 15]
    face_ids = first + second
    photos = [101, 102, 103, 104, 105, 201, 202, 203, 204, 205]

    edges = _complete(first, 0.85) + _complete(second, 0.85)

    labels = FaceEngine.cluster_graph(
        face_ids, edges, algorithm="graph-v2", photo_ids=photos
    )

    assert len(set(labels[:5])) == 1 and labels[0] != -1
    assert len(set(labels[5:])) == 1 and labels[5] != -1
    assert labels[0] != labels[5]
