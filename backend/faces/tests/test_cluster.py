"""Clustering endpoint tests.

Embeddings are grouped by an absolute cosine-similarity floor: faces closer
than ``1 - CLUSTER_MIN_SIMILARITY`` become one person, and a person needs at
least ``minClusterSize`` faces to form a cluster (otherwise the face stays
unassigned as noise ``-1``).

Faces the strict floor leaves over are then recognised against *portraits* at
the relaxed ``RECOGNITION_MIN_SIMILARITY`` floor, and only a portrait may anchor
a new person.
"""

from __future__ import annotations

import math

import numpy as np

from app.engine import CLUSTER_MIN_SIMILARITY, EMBEDDING_DIM


def _face(identity_axis: int, deviation_axis: int, cosine: float = 1.0) -> list[float]:
    """Return a unit vector on *identity_axis*, tilted toward *deviation_axis*.

    The tilt models the pose/lighting variation between two photos of the same
    person: ``cosine`` is exactly the similarity to the identity's other face
    on the same axis.
    """
    vector = np.zeros(EMBEDDING_DIM)
    vector[identity_axis] = cosine
    vector[deviation_axis] = math.sqrt(max(0.0, 1.0 - cosine**2))
    return vector.tolist()


def _at(reference: list[float], cosine: float, axis: int) -> list[float]:
    """Return a unit vector at exactly *cosine* similarity to *reference*."""
    vector = np.zeros(EMBEDDING_DIM)
    vector += cosine * np.asarray(reference)
    vector[axis] += math.sqrt(max(0.0, 1.0 - cosine**2))
    return vector.tolist()


def _similarity(a: list[float], b: list[float]) -> float:
    return float(np.dot(a, b))


def _cluster(
    client,
    embeddings: list[list[float]],
    min_cluster_size: int = 2,
    solo_faces: list[bool] | None = None,
) -> list[int]:
    payload: dict = {"embeddings": embeddings, "minClusterSize": min_cluster_size}
    if solo_faces is not None:
        payload["soloFaces"] = solo_faces
    response = client.post("/faces/cluster", json=payload)
    assert response.status_code == 200
    return response.json()["labels"]


def test_cluster_groups_same_identity(client) -> None:
    rng = np.random.default_rng(42)
    base_a = np.zeros(EMBEDDING_DIM)
    base_a[0] = 1.0
    base_b = np.zeros(EMBEDDING_DIM)
    base_b[1] = 1.0

    group_a = [(base_a + rng.normal(0, 0.02, EMBEDDING_DIM)).tolist() for _ in range(6)]
    group_b = [(base_b + rng.normal(0, 0.02, EMBEDDING_DIM)).tolist() for _ in range(6)]

    labels = _cluster(client, group_a + group_b)

    # The same-identity embeddings must share one non-noise label.
    first_label = labels[0]
    assert first_label != -1
    assert all(label == first_label for label in labels[:6])
    # The other identity must form a different, non-noise label.
    second_label = labels[6]
    assert second_label != -1
    assert second_label != first_label
    assert all(label == second_label for label in labels[6:])


def test_cluster_groups_a_person_and_leaves_singletons_as_noise(client) -> None:
    """A person with several photos clusters; a face seen once stays unassigned."""
    # Three photos of one person: every pair keeps a cosine above the floor.
    person = [_face(0, 1), _face(0, 1, 0.7), _face(0, 2, 0.7)]
    # A second person with two photos.
    other = [_face(3, 4), _face(3, 4, 0.7)]
    # Two unrelated faces, each photographed once.
    lonely_one = _face(5, 6)
    lonely_two = _face(7, 8)

    labels = _cluster(client, person + other + [lonely_one, lonely_two])

    assert labels[0] == labels[1] == labels[2] != -1, labels
    assert labels[3] == labels[4] != labels[0], labels
    assert labels[5] == -1 and labels[6] == -1, labels


def test_cluster_does_not_merge_faces_below_the_similarity_floor(client) -> None:
    """Two unrelated faces just under the floor are not one person."""
    reference = _face(0, 1)
    below_floor = CLUSTER_MIN_SIMILARITY - 0.1
    impostor = _at(reference, below_floor, axis=2)
    assert _similarity(reference, impostor) < CLUSTER_MIN_SIMILARITY

    labels = _cluster(client, [reference, impostor])

    assert labels == [-1, -1], labels


def test_cluster_marks_a_single_face_as_noise(client) -> None:
    """One face is not a person: it must keep the noise label."""
    labels = _cluster(client, [_face(0, 1)], min_cluster_size=2)

    assert labels == [-1], labels


def test_leftover_face_joins_the_group_it_matches(client) -> None:
    """A face the strict floor rejects joins the person it clearly matches.

    This is the small-face case: a face seen in a group photo scores ~0.3
    against the same person's portrait, far below ``CLUSTER_MIN_SIMILARITY``.
    """
    person = [_face(0, 1), _face(0, 1, 0.9)]
    leftover = _at(_face(0, 1), 0.30, axis=2)

    labels = _cluster(client, person + [leftover], solo_faces=[True, True, False])

    assert labels[0] == labels[1] == labels[2] != -1, labels


def test_group_shot_members_are_not_recognition_anchors(client) -> None:
    """A face from a group photo must not attract faces the way a portrait does.

    People standing next to each other in one photo are similar, so using them as
    anchors pulled unrelated members of the same group photo into one person.
    """
    person = [_face(0, 1), _face(0, 1, 0.9)]
    neighbour = _at(_face(0, 1), 0.30, axis=2)

    labels = _cluster(client, person + [neighbour], solo_faces=[False, False, False])

    assert labels[2] == -1, labels


def test_leftover_pair_with_a_portrait_anchor_forms_a_person(client) -> None:
    """A portrait plus the face matched from a group shot become one person.

    This is the group-photo case: the portrait is the only face of its photo and
    the second embedding is a small face whose similarity (~0.29) is far below
    the strict clustering floor.
    """
    portrait = _face(0, 1)
    group_face = _at(portrait, 0.30, axis=2)

    labels = _cluster(client, [portrait, group_face], solo_faces=[True, False])

    assert labels[0] == labels[1] != -1, labels


def test_leftover_pair_without_a_portrait_stays_noise(client) -> None:
    """Two group-shot faces that merely look alike must not become a person."""
    first = _face(0, 1)
    second = _at(first, 0.30, axis=2)

    labels = _cluster(client, [first, second])

    assert labels == [-1, -1], labels


def test_leftovers_do_not_chain_through_each_other(client) -> None:
    """A face joins only what it matches itself, never a neighbour's person.

    ``middle`` matches the portrait at 0.30 and ``far`` matches ``middle`` at
    0.30 while scoring 0.05 against the portrait. Chained recognition would move
    ``far`` into the portrait's person and pull a whole group photo along with it.
    """
    portrait = _face(0, 1)
    middle = _at(portrait, 0.30, axis=2)
    far = np.zeros(EMBEDDING_DIM)
    far[0] = 0.05
    far[2] = 0.2987
    far[3] = 0.953
    far /= np.linalg.norm(far)
    assert abs(_similarity(far.tolist(), middle) - 0.30) < 0.01

    labels = _cluster(client, [portrait, middle, far.tolist()], solo_faces=[True, False, False])

    assert labels[0] == labels[1] != -1, labels
    assert labels[2] == -1, labels


def test_ambiguous_leftover_stays_noise(client) -> None:
    """A face that fits two people equally well must not be guessed."""
    group_a = [_face(0, 1), _face(0, 1, 0.9)]
    group_b = [_face(3, 5), _face(3, 5, 0.9)]
    candidate = np.zeros(EMBEDDING_DIM)
    candidate[0] = 0.30
    candidate[3] = 0.28
    candidate[7] = math.sqrt(1.0 - 0.30**2 - 0.28**2)

    labels = _cluster(
        client,
        group_a + group_b + [candidate.tolist()],
        solo_faces=[True, True, True, True, False],
    )

    assert labels[4] == -1, labels


def test_leftover_below_the_recognition_floor_stays_noise(client) -> None:
    """A weak match is not a recognition: it keeps the noise label."""
    person = [_face(0, 1), _face(0, 1, 0.9)]
    weak = _at(_face(0, 1), 0.20, axis=2)

    labels = _cluster(client, person + [weak], solo_faces=[True, True, False])

    assert labels[2] == -1, labels


def test_default_min_cluster_size_groups_a_person_photographed_twice(client) -> None:
    """The default minClusterSize must let a two-photo person form a person.

    A larger default (5) labelled every face of a small gallery as noise, which
    left the People view empty even though faces were detected and embedded.
    """
    pair = [_face(0, 1), _face(0, 1, 0.9)]

    response = client.post("/faces/cluster", json={"embeddings": pair})

    assert response.status_code == 200
    labels = response.json()["labels"]
    assert labels[0] == labels[1] != -1, labels


def test_cluster_of_an_empty_gallery_is_empty(client) -> None:
    assert _cluster(client, []) == []


def test_cluster_is_deterministic(client) -> None:
    rng = np.random.default_rng(7)
    base = np.zeros(EMBEDDING_DIM)
    base[0] = 1.0
    embeddings = [(base + rng.normal(0, 0.02, EMBEDDING_DIM)).tolist() for _ in range(8)]

    assert _cluster(client, embeddings) == _cluster(client, embeddings)
