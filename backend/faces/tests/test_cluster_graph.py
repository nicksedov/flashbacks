"""Candidate-graph clustering endpoint tests.

Core asks the vector store for each unassigned face's neighbours inside the
clustering radius and posts those links. The service still owns every
threshold: the strict floor (0.40), the recognition floor (0.25) and the
runner-up margin (0.05).
"""

from __future__ import annotations


def _graph(client, face_ids, edges, solo_faces=None) -> list[int]:
    payload: dict = {
        "faceIds": face_ids,
        "edges": [
            {"faceId": face_id, "candidateId": candidate_id, "similarity": similarity}
            for face_id, candidate_id, similarity in edges
        ],
    }
    if solo_faces is not None:
        payload["soloFaces"] = solo_faces
    response = client.post("/faces/cluster-graph", json=payload)
    assert response.status_code == 200
    return response.json()["labels"]


def test_candidates_scope_graph(client) -> None:
    """Only the neighbour inside the radius joins the group."""
    labels = _graph(client, [1, 2, 3], [(1, 2, 0.9), (1, 3, 0.2)], [True, True, True])

    assert labels[0] == labels[1] != -1, labels
    assert labels[2] == -1, labels


def test_relaxed_candidate_joins_group(client) -> None:
    """A leftover joins the group its best portrait candidate names at 0.30."""
    labels = _graph(client, [1, 2, 3], [(1, 2, 0.9), (3, 1, 0.30)], [True, True, False])

    assert labels[0] == labels[1] == labels[2] != -1, labels


def test_ambiguous_candidate_stays_unassigned(client) -> None:
    """Two close candidate groups (0.30 vs 0.285) are not guessed."""
    labels = _graph(
        client,
        [1, 2, 3, 4, 5],
        [(1, 2, 0.9), (4, 5, 0.9), (3, 1, 0.30), (3, 4, 0.285)],
        [True, True, False, True, True],
    )

    assert labels[0] == labels[1] != -1, labels
    assert labels[3] == labels[4] != -1, labels
    assert labels[2] == -1, labels


def test_unknown_faces_are_ignored(client) -> None:
    """A link naming a face outside the request cannot create a group."""
    labels = _graph(client, [1, 2], [(1, 2, 0.9), (1, 99, 0.95)])

    assert labels[0] == labels[1] != -1, labels


def test_empty_graph_is_empty(client) -> None:
    assert _graph(client, [], []) == []


def test_one_way_candidates_still_form_a_group(client) -> None:
    """The strict pass follows every candidate link, mutual or not."""
    labels = _graph(
        client,
        [1, 2, 3, 4],
        [(1, 2, 0.9), (2, 3, 0.9), (3, 4, 0.9)],
        [True, True, True, True],
    )

    assert len(set(labels)) == 1 and labels[0] != -1, labels


def _identity_chain(first: int, size: int, similarity: float = 0.9) -> list[tuple]:
    """A chain of mutual links joining *size* faces of one identity."""
    return [
        pair
        for face in range(first, first + size - 1)
        for pair in ((face, face + 1, similarity), (face + 1, face, similarity))
    ]


def test_chained_group_is_split_into_identities(client) -> None:
    """A bridge that chains two identities is broken by the person size cap.

    Two 150-face identities are joined into one 301-face component by a single
    face whose two links sit at 0.42 — just above the strict floor. That group
    cannot be one person, so it is re-split on mutual links only, where the
    bridge does not exist.
    """
    first = _identity_chain(1, 150)
    second = _identity_chain(151, 150)
    bridge = [(1, 301, 0.42), (301, 151, 0.42)]

    labels = _graph(client, list(range(1, 302)), first + second + bridge)

    assert labels[0] == labels[149] != -1, labels[:5]
    assert labels[150] == labels[299] != -1, labels[150:155]
    assert labels[0] != labels[150], labels[:5]
    # The bridging face matches both identities equally well, so it is noise
    # rather than a member of either.
    assert labels[300] == -1, labels[298:]


def test_group_under_the_cap_keeps_its_identity(client) -> None:
    """A single identity within the size cap is never split."""
    labels = _graph(client, list(range(1, 121)), _identity_chain(1, 120))

    assert set(labels) == {0}, labels[:5]


def _mutual_edges(
    pairs: list[tuple[int, int]], similarity: float
) -> list[tuple[int, int, float]]:
    edges: list[tuple[int, int, float]] = []
    for left, right in pairs:
        edges.append((left, right, similarity))
        edges.append((right, left, similarity))
    return edges


def _complete_edges(
    faces: list[int], similarity: float
) -> list[tuple[int, int, float]]:
    pairs = [
        (left, right)
        for index, left in enumerate(faces)
        for right in faces[index + 1 :]
    ]
    return _mutual_edges(pairs, similarity)


def _v2_labels(face_ids, edges, photo_ids) -> list[int]:
    from app.engine import FaceEngine

    return FaceEngine.cluster_graph(
        face_ids, edges, algorithm="graph-v2", photo_ids=photo_ids
    )


def test_burst_sequence_yields_one_person_not_several() -> None:
    """Six faces of one person across two photos form one person.

    A four-face burst on a single photo must not become a person of its own:
    a person spans at least two photos.
    """
    face_ids = list(range(1, 11))
    photos = [10, 10, 10, 11, 11, 11, 20, 20, 20, 20]
    edges = _complete_edges(list(range(1, 7)), 0.85) + _complete_edges(
        list(range(7, 11)), 0.85
    )

    labels = _v2_labels(face_ids, edges, photos)

    assert len(set(labels[:6])) == 1, labels[:6]
    assert labels[0] != -1
    assert labels[6:] == [-1, -1, -1, -1], labels[6:]


def _merge_graph(first: list[int], second: list[int]):
    """Two star-shaped groups whose hubs are clear medoids."""
    links: dict[tuple[int, int], float] = {}
    directed: set[tuple[int, int]] = set()

    def add(left: int, right: int, similarity: float) -> None:
        key = (min(left, right), max(left, right))
        links[key] = max(links.get(key, 0.0), similarity)
        directed.add((left, right))
        directed.add((right, left))

    for members in (first, second):
        hub = members[0]
        for member in members[1:]:
            add(hub, member, 0.9)
    for left in first[:5]:
        for right in second[:5]:
            add(left, right, 0.7)
    return links, directed


def test_merge_stage_respects_the_gallery_share_cap() -> None:
    """Two 200-face groups are one identity by evidence but exceed 10% here."""
    from app.engine import FaceEngine

    first = list(range(1, 201))
    second = list(range(201, 401))
    links, directed = _merge_graph(first, second)
    photos = list(range(1, 401))

    groups = FaceEngine._merge_groups(
        [first, second], links, directed, photos, 0.5, 2, 0.4
    )

    assert len(groups) == 2, [len(group) for group in groups]


def test_merge_stage_rejoins_the_pieces_of_one_person() -> None:
    """Two components the core stage kept apart merge on medoid and support.

    Cross links sit at 0.60: below the corrected core floor, so the core stage
    forms two components, but the medoid similarity clears the merge floor and
    the four distinct photo pairs give the required cross-component support.
    """
    first = [1, 2, 3, 4, 5]
    second = [11, 12, 13, 14, 15]
    face_ids = first + second
    photos = [101, 101, 102, 102, 102, 201, 201, 202, 202, 202]

    edges = _complete_edges(first, 0.85) + _complete_edges(second, 0.85)
    edges += _mutual_edges([(left, right) for left in first for right in second], 0.60)

    labels = _v2_labels(face_ids, edges, photos)

    assert len(set(labels)) == 1, labels
    assert labels[0] != -1
