"""Calibration-harness tests.

The harness must be trustworthy before its numbers are: the metric helpers are
pure functions, and the embedding cache must embed each photo exactly once and
serve later runs without constructing an engine.
"""

from __future__ import annotations

import csv
import hashlib
import io
import json
from pathlib import Path
from typing import Any

from PIL import Image

from tools import calibrate_clustering as cal


def _face(
    identity: str,
    label: str,
    file: str,
    sha256: str,
    *,
    faces_on_photo: int = 1,
    face_id: int = 0,
) -> cal.FixtureFace:
    return cal.FixtureFace(
        identity=identity,
        label=label,
        file=file,
        image_file_id=face_id,
        face_id=face_id,
        confidence=0.9,
        box={"x": 0.0, "y": 0.0, "width": 10.0, "height": 10.0},
        faces_on_photo=faces_on_photo,
        sha256=sha256,
    )


def test_fragments_count_distinct_people_per_identity() -> None:
    labels = [0, 0, 1, 2, -1]
    identities = ["dima", "dima", "dima", "other", "dima"]

    fragments = cal.fragments_per_identity(labels, identities)

    assert fragments == {"dima": 2, "other": 1}


def test_pairwise_precision_and_recall() -> None:
    """One identity split over two people while two identities share a person."""
    labels = [0, 0, 1, 1]
    identities = ["a", "a", "a", "b"]

    precision, recall = cal.pairwise_scores(labels, identities)

    # predicted pairs: (0,1) same a -> correct; (2,3) a+b -> false merge.
    # true same-identity pair: (0,1) found, (0,2) and (1,2) missed.
    assert precision == 0.5
    assert recall == 1 / 3


def test_pairwise_scores_ignore_unassigned_faces() -> None:
    labels = [-1, 0, 0]
    identities = ["a", "b", "b"]

    assert cal.pairwise_scores(labels, identities) == (1.0, 1.0)


def test_unassigned_share_and_person_count() -> None:
    labels = [0, 0, 1, -1]

    assert cal.unassigned_share(labels) == 0.25
    assert cal.person_count(labels) == 2


def test_production_label_table_reports_the_six_fragments() -> None:
    faces = [
        _face("dima", "1905", "a.jpg", "a", face_id=1),
        _face("dima", "1905", "b.jpg", "b", face_id=2),
        _face("dima", "1906", "c.jpg", "c", face_id=3),
        _face("dima", "5810", "d.jpg", "d", face_id=4),
    ]
    labels = [0, 0, 1, -1]

    rows = cal.production_label_table(labels, faces)

    assert rows == [
        {"label": "1905", "faces": 2, "clusters": 1, "largest": 2, "unassigned": 0},
        {"label": "1906", "faces": 1, "clusters": 1, "largest": 1, "unassigned": 0},
        {"label": "5810", "faces": 1, "clusters": 0, "largest": 0, "unassigned": 1},
    ]


def test_candidate_edges_respect_the_floor_and_limit() -> None:
    embeddings = [[1.0, 0.0], [1.0, 0.0], [0.0, 1.0]]

    edges = cal.candidate_edges(embeddings, [1, 2, 3], floor=0.5)

    assert (1, 2, 1.0) in edges
    assert (2, 1, 1.0) in edges
    assert all(edge[0] != 3 and edge[1] != 3 for edge in edges)


def test_candidate_edges_cap_candidates_per_photo() -> None:
    embeddings = [[1.0, 0.0]] * 4
    photo_ids = [10, 10, 10, 20]

    edges = cal.candidate_edges(
        embeddings, [1, 2, 3, 4], limit=10, photo_ids=photo_ids, max_per_photo=2
    )

    from_photo_10 = [edge for edge in edges if edge[0] == 4 and edge[1] in (1, 2, 3)]
    assert len(from_photo_10) == 2
    assert any(edge[1] == 4 for edge in edges if edge[0] in (1, 2, 3))


class _FakeEngine:
    """Returns one face per image and counts analyses."""

    def __init__(self) -> None:
        self.calls = 0

    def analyze(self, image_b64: str) -> list[dict[str, Any]]:
        self.calls += 1
        return [
            {
                "x": 0.0,
                "y": 0.0,
                "width": 10.0,
                "height": 10.0,
                "confidence": 0.9,
                "embedding": [1.0, 0.0],
            }
        ]


def _write_fixture(dataset: Path) -> list[cal.FixtureFace]:
    dataset.mkdir(parents=True)
    rows = []
    for index, name in enumerate(("one.jpg", "two.jpg")):
        image = Image.new("RGB", (32, 32), color=(index * 40 + 10, 60, 90))
        buffer = io.BytesIO()
        image.save(buffer, format="JPEG")
        payload = buffer.getvalue()
        (dataset / name).write_bytes(payload)
        rows.append(
            {
                "person_id": "1905",
                "file": name,
                "image_file_id": str(index + 1),
                "face_id": str(index + 1),
                "confidence": "0.9",
                "x": "0",
                "y": "0",
                "width": "10",
                "height": "10",
                "faces_on_photo": "1",
                "sha256": hashlib.sha256(payload).hexdigest(),
            }
        )

    with (dataset / "manifest.csv").open("w", newline="", encoding="utf-8") as handle:
        writer = csv.DictWriter(handle, fieldnames=list(rows[0]))
        writer.writeheader()
        writer.writerows(rows)
    return cal.load_manifest(dataset / "manifest.csv")


def test_embeddings_are_cached_by_sha_and_reused(tmp_path: Path) -> None:
    dataset = tmp_path / "dima"
    faces = _write_fixture(dataset)
    cache = dataset / cal.CACHE_NAME

    messages: list[str] = []
    engine = _FakeEngine()
    embeddings = cal.ensure_embeddings(
        faces, dataset, cache, engine_factory=lambda: engine, out=messages.append
    )

    assert messages == ["2 faces embedded, cache written"]
    assert engine.calls == 2
    assert set(embeddings) == {face.sha256 for face in faces}
    payload = json.loads(cache.read_text(encoding="utf-8"))
    assert set(payload["embeddings"]) == {face.sha256 for face in faces}

    def fail() -> Any:  # pragma: no cover - only a cache miss calls this
        raise AssertionError("cache hit must not construct an engine")

    second_messages: list[str] = []
    reused = cal.ensure_embeddings(
        faces, dataset, cache, engine_factory=fail, out=second_messages.append
    )

    assert second_messages == ["cache hit"]
    assert reused == embeddings


def test_baseline_groups_identical_vectors() -> None:
    faces = [
        _face("dima", "1905", "a.jpg", "a", face_id=1),
        _face("dima", "1905", "b.jpg", "b", face_id=2),
        _face("other", "1", "c.jpg", "c", face_id=3),
    ]
    embeddings = {"a": [1.0, 0.0], "b": [1.0, 0.0], "c": [0.0, 1.0]}

    labels = cal.baseline_labels(faces, embeddings)
    score = cal.score_labels(labels, faces)

    assert labels[0] == labels[1] != -1
    assert labels[2] == -1
    assert score.fragments == {"dima": 1}
    assert score.people == 1
    assert score.precision == 1.0
    assert score.recall == 1.0
