"""Size criteria tests: what counts as too small to cluster, and as too weak.

A face whose smaller side is below ``MIN_CLUSTERABLE_FACE_PX`` is never
clustered — it stays a detection and is never assigned to a person. A face below
``SMALL_FACE_WIDTH`` is *weak*: it may join a person founded by a full-size face,
but it can never found one itself, never becomes an exemplar and never a cover.

The behaviour is exercised through the same entry points core uses: the
candidate-graph pass, the in-memory pass, and the public endpoints that carry the
``widths`` array (the smaller side of each box).
"""

from __future__ import annotations

import pytest

from app.config import MIN_CLUSTERABLE_FACE_PX, SMALL_FACE_WIDTH
from app.engine import FaceEngine, _drop_unfounded_labels, _face_roles, _face_sizes

FULL = SMALL_FACE_WIDTH + 26  # 90 px: a full participant
WEAK = MIN_CLUSTERABLE_FACE_PX + 8  # 40 px: may join, may not found
TINY = MIN_CLUSTERABLE_FACE_PX - 12  # 20 px: never clustered


def _graph(
    client,
    face_ids: list[int],
    edges: list[tuple[int, int, float]],
    widths: list[float] | None = None,
    solo_faces: list[bool] | None = None,
) -> list[int]:
    payload: dict = {
        "faceIds": face_ids,
        "edges": [
            {"faceId": face_id, "candidateId": candidate_id, "similarity": similarity}
            for face_id, candidate_id, similarity in edges
        ],
    }
    if widths is not None:
        payload["widths"] = widths
    if solo_faces is not None:
        payload["soloFaces"] = solo_faces
    response = client.post("/faces/cluster-graph", json=payload)
    assert response.status_code == 200
    return response.json()["labels"]


def test_roles_follow_the_two_size_limits() -> None:
    """A face is full, weak or too small, by its smaller side only."""
    clusterable, may_found = _face_roles([TINY, WEAK, FULL], MIN_CLUSTERABLE_FACE_PX)

    assert clusterable == [False, True, True]
    assert may_found == [False, False, True]


def test_missing_widths_treat_every_face_as_full_size() -> None:
    """An older core sends no sizes, and then no size rule applies."""
    sizes = _face_sizes(3, None)

    assert sizes == [float("inf")] * 3


def test_a_face_below_the_floor_is_never_labelled(client) -> None:
    """The tiny face of an otherwise identical trio stays unassigned."""
    labels = _graph(
        client,
        [1, 2, 3],
        [(1, 2, 0.9), (1, 3, 0.9), (2, 3, 0.9)],
        widths=[FULL, FULL, TINY],
    )

    assert labels[0] == labels[1] != -1, labels
    assert labels[2] == -1, labels


def test_weak_faces_cannot_found_a_person(client) -> None:
    """A trio of weak faces with tight links still makes nobody."""
    labels = _graph(
        client,
        [1, 2, 3],
        [(1, 2, 0.9), (2, 3, 0.9), (1, 3, 0.9)],
        widths=[WEAK, WEAK, WEAK],
        solo_faces=[True, True, True],
    )

    assert labels == [-1, -1, -1], labels


def test_weak_faces_join_the_person_a_full_face_founds(client) -> None:
    """The same trio is one person once a full-size face is part of it."""
    labels = _graph(
        client,
        [1, 2, 3],
        [(1, 2, 0.9), (2, 3, 0.9), (1, 3, 0.9)],
        widths=[FULL, WEAK, WEAK],
    )

    assert labels[0] == labels[1] == labels[2] != -1, labels


def test_without_widths_the_weak_trio_still_forms_a_person(client) -> None:
    """Compatibility: no sizes means the previous behaviour, unchanged."""
    labels = _graph(client, [1, 2, 3], [(1, 2, 0.9), (2, 3, 0.9), (1, 3, 0.9)])

    assert labels[0] == labels[1] == labels[2] != -1, labels


def test_graph_v2_refuses_core_points_among_weak_faces() -> None:
    """A weak pair never becomes a component, however tight its links."""
    labels = FaceEngine.cluster_graph(
        [1, 2],
        [(1, 2, 0.95), (2, 1, 0.95)],
        solo_faces=[True, True],
        widths=[WEAK, WEAK],
        algorithm="graph-v2",
    )

    assert labels == [-1, -1], labels


def test_in_memory_pass_keeps_a_tiny_face_unassigned(client) -> None:
    """The fallback path applies the same floor as the candidate graph."""
    base = [1.0] + [0.0] * 511
    payload = {
        "embeddings": [base, base],
        "minClusterSize": 2,
        "widths": [FULL, TINY],
    }
    response = client.post("/faces/cluster", json=payload)
    assert response.status_code == 200
    labels = response.json()["labels"]

    assert labels[0] != -1, labels
    assert labels[1] == -1, labels


def test_in_memory_pass_refuses_weak_only_persons(client) -> None:
    """Two weak faces are not a person even when they are identical."""
    base = [1.0] + [0.0] * 511
    payload = {
        "embeddings": [base, base],
        "minClusterSize": 2,
        "widths": [WEAK, WEAK],
    }
    response = client.post("/faces/cluster", json=payload)
    assert response.status_code == 200

    assert response.json()["labels"] == [-1, -1]


def test_unfounded_label_is_dropped_but_members_keep_their_faces() -> None:
    """A cluster with no full-size founder loses its label, nothing else."""
    labels = _drop_unfounded_labels([0, 0, 1, 1], [True] * 4, [False, False, True, True])

    assert labels == [-1, -1, 1, 1]


def test_health_reports_the_size_floors(client) -> None:
    thresholds = client.get("/faces/health").json()["thresholds"]

    assert thresholds["minClusterableFacePx"] == MIN_CLUSTERABLE_FACE_PX
    assert thresholds["weakFacePx"] == SMALL_FACE_WIDTH


@pytest.mark.parametrize("size", [0.0, MIN_CLUSTERABLE_FACE_PX - 0.5])
def test_a_face_exactly_below_the_floor_is_too_small(size: float) -> None:
    clusterable, _ = _face_roles([size], MIN_CLUSTERABLE_FACE_PX)

    assert clusterable == [False]


def test_a_face_at_the_floor_is_clusterable() -> None:
    clusterable, may_found = _face_roles([MIN_CLUSTERABLE_FACE_PX], MIN_CLUSTERABLE_FACE_PX)

    assert clusterable == [True]
    assert may_found == [False]
