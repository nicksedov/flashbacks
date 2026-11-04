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
