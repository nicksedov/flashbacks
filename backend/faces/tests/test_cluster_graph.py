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
