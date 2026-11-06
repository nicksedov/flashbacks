"""Hermetic tests for the 1:N clustering evaluation harness.

Nothing here loads a model or touches the network: images are tiny JPEGs written
into ``tmp_path``, the engine is a fake that returns a known vector per payload,
and the labeller is the service's own ``FaceEngine.cluster_graph``.
"""

from __future__ import annotations

import base64
import io
import json
from pathlib import Path

import numpy as np
import pytest

from tools import clustering_benchmark as bench
from tools.clustering_benchmark import (
    ClusterConfig,
    GraphConfig,
    LabelledImage,
    build_edges,
    b_cubed,
    evaluate,
    hard_negatives,
    label_faces,
    load_labelled_images,
    normalised_matrix,
    pairwise,
    rank,
    score_labels,
)


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


def jpeg(seed: int) -> bytes:
    """Distinct, valid JPEG bytes per seed."""
    from PIL import Image

    image = Image.new("RGB", (16, 16), color=(seed % 256, (seed * 7) % 256, 32))
    buffer = io.BytesIO()
    image.save(buffer, format="JPEG")
    return buffer.getvalue()


class FakeEngine:
    """Deterministic engine: one known embedding per image payload."""

    name = "buffalo_l"

    def __init__(self, vectors: dict[bytes, list[float]]) -> None:
        self.vectors = vectors
        self.analyzed: list[bytes] = []

    def analyze(self, image_b64: str) -> list[dict]:
        raw = base64.b64decode(image_b64)
        self.analyzed.append(raw)
        vector = self.vectors.get(raw)
        if vector is None:
            return []
        return [
            {
                "x": 0.0,
                "y": 0.0,
                "width": 10.0,
                "height": 10.0,
                "confidence": 0.99,
                "embedding": vector,
            }
        ]


def write_tree(root: Path, spec: dict[str, int]) -> dict[str, list[Path]]:
    """Write ``count`` images per identity; return the written paths."""
    written: dict[str, list[Path]] = {}
    seed = 1
    for identity, count in spec.items():
        paths: list[Path] = []
        for number in range(1, count + 1):
            path = root / identity / f"{identity}_{number:04d}.jpg"
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(jpeg(seed))
            paths.append(path)
            seed += 1
        written[identity] = paths
    return written


def axis(index: int, dimension: int = 8) -> list[float]:
    """A unit vector on one axis."""
    vector = [0.0] * dimension
    vector[index] = 1.0
    return vector


def blend(left: int, right: int, cosine: float, dimension: int = 8) -> list[float]:
    """A unit vector at *cosine* from axis *left*, tilted toward axis *right*."""
    sine = float(np.sqrt(max(0.0, 1.0 - cosine * cosine)))
    vector = [0.0] * dimension
    vector[left] = cosine
    vector[right] = sine
    return vector


# ---------------------------------------------------------------------------
# Labelled index
# ---------------------------------------------------------------------------


def test_index_keeps_only_multi_image_identities(tmp_path: Path) -> None:
    write_tree(tmp_path, {"Alice": 2, "Bob": 1, "Carol": 3})

    images = load_labelled_images(tmp_path)

    assert {image.identity for image in images} == {"Alice", "Carol"}
    assert len(images) == 5
    assert all(image.photo == image.relative for image in images)


def test_index_reads_flat_filename_prefixes(tmp_path: Path) -> None:
    tmp_path.mkdir(exist_ok=True)
    (tmp_path / "obama_front.jpg").write_bytes(jpeg(1))
    (tmp_path / "obama_profile.jpg").write_bytes(jpeg(2))
    (tmp_path / "curie_portrait.jpg").write_bytes(jpeg(3))

    images = load_labelled_images(tmp_path, flat_names=True)

    assert {image.identity for image in images} == {"obama"}
    assert len(images) == 2


def test_index_limits_identities_alphabetically(tmp_path: Path) -> None:
    write_tree(tmp_path, {"Alice": 2, "Bob": 2, "Carol": 2})

    images = load_labelled_images(tmp_path, limit_identities=2)

    assert {image.identity for image in images} == {"Alice", "Bob"}


def test_index_rejects_a_tree_without_pairs(tmp_path: Path) -> None:
    write_tree(tmp_path, {"Alice": 1})
    with pytest.raises(bench.CalibrationError, match="at least 2 images"):
        load_labelled_images(tmp_path)


# ---------------------------------------------------------------------------
# Graph
# ---------------------------------------------------------------------------


def test_normalised_matrix_ignores_missing_and_zero_vectors() -> None:
    matrix = normalised_matrix([np.array([3.0, 4.0]), None, np.zeros(2)])

    assert matrix.shape == (3, 2)
    assert matrix[0] == pytest.approx([0.6, 0.8])
    assert not np.any(matrix[1]) and not np.any(matrix[2])


def test_build_edges_respects_the_floor_and_top_k() -> None:
    vectors = [axis(0), blend(0, 1, 0.95), axis(2)]
    edges = build_edges(vectors, ["a", "b", "c"], GraphConfig(floor=0.5, top_k=1))

    assert {(left, right) for left, right, _ in edges} == {(0, 1), (1, 0)}
    assert all(similarity >= 0.5 for _, _, similarity in edges)


def test_build_edges_caps_candidates_per_photo() -> None:
    vectors = [axis(0), blend(0, 1, 0.99), blend(0, 1, 0.98), blend(0, 2, 0.80)]
    photos = ["p1", "p1", "p2", "p3"]

    edges = build_edges(
        vectors,
        photos,
        GraphConfig(floor=0.5, top_k=3, per_photo=1),
    )

    from_first = [right for left, right, _ in edges if left == 0]
    assert photos[from_first[0]] != photos[from_first[1]]
    assert len(from_first) == 3


def test_build_edges_rejects_misaligned_input() -> None:
    with pytest.raises(bench.CalibrationError, match="photo id"):
        build_edges([axis(0)], [], GraphConfig())


# ---------------------------------------------------------------------------
# Metrics
# ---------------------------------------------------------------------------


def test_perfect_labelling_scores_one() -> None:
    score = score_labels([0, 0, 1, 1], ["A", "A", "B", "B"], algorithm="graph-v2", floor=0.4, top_k=16)

    assert (score.b3_precision, score.b3_recall, score.b3_f1) == pytest.approx((1.0, 1.0, 1.0))
    assert score.pairwise_f1 == pytest.approx(1.0)
    assert score.merge_rate == 0.0
    assert score.identity_merges == 0
    assert score.identity_integrity == 1.0
    assert score.unassigned == 0.0


def test_split_identity_costs_recall_not_precision() -> None:
    score = score_labels([0, 1, 2, 3], ["A", "A", "B", "B"], algorithm="graph-v1", floor=0.4, top_k=16)

    assert score.b3_precision == pytest.approx(1.0)
    assert score.b3_recall == pytest.approx(0.5)
    assert score.identity_integrity == 0.0
    assert score.identity_merges == 0
    assert score.clusters == 4
    assert score.singleton_clusters == 4
    assert score.merge_rate == 0.0


def test_merged_identities_cost_precision_and_are_counted() -> None:
    score = score_labels([0, 0, 0, 0], ["A", "A", "B", "B"], algorithm="graph-v2", floor=0.4, top_k=16)

    assert score.b3_recall == pytest.approx(1.0)
    assert score.b3_precision == pytest.approx(0.5)
    assert score.merge_rate == pytest.approx(1.0)
    assert score.merged_identity_share == pytest.approx(1.0)
    assert score.identity_merges == 1
    assert score.identity_integrity == 0.0


def test_unassigned_faces_are_singletons_not_merges() -> None:
    score = score_labels([-1, 0, 1, 1], ["A", "A", "B", "B"], algorithm="graph-v2", floor=0.4, top_k=16)

    assert score.unassigned == pytest.approx(0.25)
    assert score.b3_precision == pytest.approx(1.0)
    # Both A photos are split (one unassigned, one alone); B stays whole.
    assert score.b3_recall == pytest.approx(0.75)
    assert score.merge_rate == 0.0
    assert score.identity_merges == 0


def test_b_cubed_and_pairwise_handle_a_single_identity() -> None:
    assert b_cubed([0, 0], ["A", "A"]) == pytest.approx((1.0, 1.0, 1.0))
    assert pairwise([0, 0], ["A", "A"]) == pytest.approx((1.0, 1.0, 1.0))
    assert pairwise([0, 1], ["A", "A"])[2] == pytest.approx(0.0)


def test_score_rejects_misaligned_labels() -> None:
    with pytest.raises(bench.CalibrationError, match="aligned"):
        score_labels([0], [], algorithm="graph-v1", floor=0.4, top_k=16)


# ---------------------------------------------------------------------------
# Labelling through the service
# ---------------------------------------------------------------------------


def _complete(faces: list[int], similarity: float) -> list[tuple[int, int, float]]:
    edges: list[tuple[int, int, float]] = []
    for left in faces:
        for right in faces:
            if left != right:
                edges.append((left, right, similarity))
    return edges


@pytest.mark.parametrize("algorithm", ["graph-v1", "graph-v2"])
def test_label_faces_separates_two_identities(algorithm: str) -> None:
    images = [
        LabelledImage("A", Path(f"A{i}.jpg"), f"A{i}.jpg", f"A{i}.jpg") for i in range(4)
    ] + [LabelledImage("B", Path(f"B{i}.jpg"), f"B{i}.jpg", f"B{i}.jpg") for i in range(4)]

    edges = _complete([0, 1, 2, 3], 0.9) + _complete([4, 5, 6, 7], 0.9)
    labels = label_faces(images, edges, ClusterConfig(algorithm=algorithm))
    score = score_labels(
        labels, [image.identity for image in images], algorithm=algorithm, floor=0.4, top_k=16
    )

    assert score.merge_rate == 0.0
    assert score.identity_integrity == 1.0
    assert score.b3_f1 == pytest.approx(1.0)


def test_evaluate_runs_the_whole_pipeline(tmp_path: Path) -> None:
    paths = write_tree(tmp_path, {"Alice": 4, "Bob": 4})
    vectors: dict[bytes, list[float]] = {}
    for number, path in enumerate(paths["Alice"]):
        vectors[path.read_bytes()] = blend(0, 1, 0.95 - 0.01 * number)
    for number, path in enumerate(paths["Bob"]):
        vectors[path.read_bytes()] = blend(2, 3, 0.95 - 0.01 * number)

    images = load_labelled_images(tmp_path)
    engine = FakeEngine(vectors)
    from tools.clustering_benchmark import embed_labelled_images
    from tools.lfw_calibration import EmbeddingCache, LazyEngine

    cache = EmbeddingCache(tmp_path / "embeddings.json", "buffalo_l", "test")
    embeddings = embed_labelled_images(images, cache, LazyEngine(lambda: engine, "buffalo_l"))

    score = evaluate(images, embeddings, graph=GraphConfig(floor=0.4), cluster=ClusterConfig(algorithm="graph-v1"))

    assert score.faces == 8
    assert score.identity_merges == 0
    assert score.identity_integrity == 1.0


def test_evaluate_detects_a_lookalike_merge(tmp_path: Path) -> None:
    paths = write_tree(tmp_path, {"Alice": 4, "Bob": 4})
    vectors: dict[bytes, list[float]] = {}
    for path in paths["Alice"] + paths["Bob"]:
        vectors[path.read_bytes()] = blend(0, 1, 0.98)

    images = load_labelled_images(tmp_path)
    engine = FakeEngine(vectors)
    from tools.clustering_benchmark import embed_labelled_images
    from tools.lfw_calibration import EmbeddingCache, LazyEngine

    cache = EmbeddingCache(tmp_path / "embeddings.json", "buffalo_l", "test")
    embeddings = embed_labelled_images(images, cache, LazyEngine(lambda: engine, "buffalo_l"))

    score = evaluate(images, embeddings, graph=GraphConfig(floor=0.4), cluster=ClusterConfig(algorithm="graph-v1"))

    assert score.identity_merges == 1
    assert score.merge_rate > 0.0
    assert score.b3_precision < 1.0


# ---------------------------------------------------------------------------
# Hard negatives and ranking
# ---------------------------------------------------------------------------


def test_hard_negatives_reports_the_cross_identity_ceiling() -> None:
    vectors = [axis(0), blend(0, 1, 0.9), axis(2)]
    identities = ["A", "B", "C"]

    stats = hard_negatives(vectors, identities, floors=(0.5, 0.95))

    assert stats.best == pytest.approx(0.9)
    assert stats.over[0.5] == 2
    assert stats.over[0.95] == 0


def test_hard_negatives_ignores_same_identity_neighbours() -> None:
    vectors = [axis(0), axis(0), blend(0, 2, 0.4)]
    stats = hard_negatives(vectors, ["A", "A", "B"], floors=(0.3,))
    assert stats.best == pytest.approx(0.4)


def test_rank_prefers_merge_free_configurations() -> None:
    scores = [
        score_labels([0, 0, 0, 0], ["A", "A", "B", "B"], algorithm="graph-v2", floor=0.4, top_k=16),
        score_labels([0, 0, 1, 1], ["A", "A", "B", "B"], algorithm="graph-v2", floor=0.4, top_k=16),
    ]

    ranked = rank(scores)

    assert ranked[0].identity_merges == 0


def test_embedding_cache_survives_a_second_run(tmp_path: Path) -> None:
    write_tree(tmp_path, {"Alice": 2})
    images = load_labelled_images(tmp_path)
    first = FakeEngine({image.path.read_bytes(): axis(0) for image in images})
    engine = FakeEngine({})
    from tools.clustering_benchmark import embed_labelled_images
    from tools.lfw_calibration import EmbeddingCache, LazyEngine

    cache_path = tmp_path / "embeddings.json"
    embed_labelled_images(
        images,
        EmbeddingCache(cache_path, "buffalo_l", "test"),
        LazyEngine(lambda: first, "buffalo_l"),
    )
    assert len(first.analyzed) == 2

    vectors = embed_labelled_images(
        images,
        EmbeddingCache(cache_path, "buffalo_l", "test"),
        LazyEngine(lambda: engine, "buffalo_l"),
    )
    assert engine.analyzed == [], "a warm cache must not call the engine"
    assert all(vector is not None for vector in vectors)


# ---------------------------------------------------------------------------
# Synthetic bursts and the dataset-agnostic CLI
# ---------------------------------------------------------------------------


def test_synthesize_bursts_adds_distinct_photos_of_one_identity(tmp_path: Path) -> None:
    write_tree(tmp_path, {"Alice": 1})
    images = load_labelled_images(tmp_path, minimum_images=1)

    bursts = bench.synthesize_bursts(images, tmp_path / "bursts", 2)

    assert [burst.identity for burst in bursts] == ["Alice", "Alice"]
    assert {burst.photo for burst in bursts} == {
        "Alice/Alice_0001.jpg#burst1",
        "Alice/Alice_0001.jpg#burst2",
    }
    assert all(burst.path.is_file() for burst in bursts)


def test_benchmark_cli_scores_a_tree_and_cleans_up(tmp_path: Path) -> None:
    paths = write_tree(tmp_path / "images", {"Alice": 4, "Bob": 4})
    vectors = {path.read_bytes(): blend(0, 1, 0.95) for path in paths["Alice"]}
    vectors.update({path.read_bytes(): blend(2, 3, 0.95) for path in paths["Bob"]})

    code = bench.main(
        [
            "score",
            "--images",
            str(tmp_path / "images"),
            "--work-dir",
            str(tmp_path / "work"),
            "--floor",
            "0.4",
            "--algorithm",
            "graph-v1",
            "--json-out",
            str(tmp_path / "score.json"),
        ],
        engine_factory=lambda: FakeEngine(vectors),
    )

    assert code == 0
    payload = json.loads((tmp_path / "score.json").read_text(encoding="utf-8"))
    assert payload["plain"]["identityMerges"] == 0
    assert not (tmp_path / "work").exists()


def test_benchmark_cli_writes_hard_negatives(tmp_path: Path) -> None:
    paths = write_tree(tmp_path / "images", {"Alice": 2, "Bob": 2})
    vectors = {path.read_bytes(): axis(0) for path in paths["Alice"]}
    vectors.update({path.read_bytes(): blend(0, 1, 0.8) for path in paths["Bob"]})

    code = bench.main(
        [
            "hard-negatives",
            "--images",
            str(tmp_path / "images"),
            "--work-dir",
            str(tmp_path / "work"),
            "--floors",
            "0.5,0.9",
        ],
        engine_factory=lambda: FakeEngine(vectors),
    )

    assert code == 0
