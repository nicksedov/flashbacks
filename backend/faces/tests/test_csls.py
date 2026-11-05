"""Hub-correction (CSLS) tests.

A face that is moderately close to everything is the bridge that turns single
linkage into one gallery-wide component. The hub-corrected score must pull that
link below the core floor while keeping a genuine same-person link above it.
"""

from __future__ import annotations

import pytest

from app.engine import FaceEngine


def _hub_graph() -> tuple[dict[int, dict[int, float]], dict[tuple[int, int], float]]:
    # 1<->2 and 3<->4 are two identities at 0.80; face 0 links to all of them
    # at 0.55. Face 0 is the hub: moderately close to everything.
    neighbour_scores = {
        0: {1: 0.55, 2: 0.55, 3: 0.55, 4: 0.55},
        1: {2: 0.80, 0: 0.55},
        2: {1: 0.80, 0: 0.55},
        3: {4: 0.80, 0: 0.55},
        4: {3: 0.80, 0: 0.55},
    }
    links = {
        (0, 1): 0.55,
        (0, 2): 0.55,
        (0, 3): 0.55,
        (0, 4): 0.55,
        (1, 2): 0.80,
        (3, 4): 0.80,
    }
    return neighbour_scores, links


def test_hub_correction_uses_the_means_of_the_candidate_lists() -> None:
    neighbour_scores, links = _hub_graph()

    corrected = FaceEngine.hub_correction(neighbour_scores, links)

    # hub mean = 0.55, identity face mean = (0.80 + 0.55) / 2 = 0.675
    assert corrected[(0, 1)] == pytest.approx(2 * 0.55 - 0.55 - 0.675)
    assert corrected[(1, 2)] == pytest.approx(2 * 0.80 - 0.675 - 0.675)


def test_hub_correction_pushes_the_hub_link_below_the_intra_identity_link() -> None:
    neighbour_scores, links = _hub_graph()

    corrected = FaceEngine.hub_correction(neighbour_scores, links)

    assert corrected[(1, 2)] > corrected[(0, 1)]
    assert corrected[(0, 1)] < 0.0


def _mutual(pairs: list[tuple[int, int]], similarity: float) -> list[tuple[int, int, float]]:
    edges = []
    for left, right in pairs:
        edges.append((left, right, similarity))
        edges.append((right, left, similarity))
    return edges


def _complete(faces: list[int], similarity: float) -> list[tuple[int, int]]:
    return [
        (left, right)
        for index, left in enumerate(faces)
        for right in faces[index + 1 :]
    ]


def test_a_hub_face_does_not_bridge_two_identities() -> None:
    """The raw 0.60 links of the hub are not core edges once corrected."""
    first = [1, 2, 3, 4, 5]
    second = [11, 12, 13, 14, 15]
    hub = 100
    face_ids = first + second + [hub]
    photos = list(range(1, 6)) + list(range(11, 16)) + [99]

    edges = _mutual(_complete(first, 0.85), 0.85)
    edges += _mutual(_complete(second, 0.85), 0.85)
    # The hub is moderately close to every identity, and both directions exist.
    edges += _mutual([(hub, face) for face in first + second], 0.60)

    labels = FaceEngine.cluster_graph(
        face_ids, edges, algorithm="graph-v2", photo_ids=photos
    )

    first_labels = labels[:5]
    second_labels = labels[5:10]
    assert len(set(first_labels)) == 1 and first_labels[0] != -1
    assert len(set(second_labels)) == 1 and second_labels[0] != -1
    assert first_labels[0] != second_labels[0]
    # The hub matches both identities equally well, so it stays unassigned
    # instead of joining one on a weak, ambiguous vote.
    assert labels[10] == -1
