"""Offline 1:N face-clustering evaluation on any labelled image tree.

LFW is a 1:1 verification benchmark, so the verification floor is calibrated by
``tools.lfw_calibration``. Clustering, however, is a different question: does the
candidate graph and the labelling in :class:`app.engine.FaceEngine` turn a folder
of faces into the right people? This module answers that on any tree of
``<root>/<identity>/<file>`` images — the LFW multi-image identities in
``tools.lfw_clustering``, the labelled fixtures under ``tests/dataset/faces`` or
a labelled export of the production gallery — and scores the result.

The graph is built the way core builds it: cosine similarity, a candidate floor,
a per-face ``top_k``, and a per-photo candidate cap. The labels come from the
service's own ``FaceEngine.cluster_graph``, so an algorithm or threshold change
is measured through production code rather than a reimplementation.

Metrics are chosen for LFW's skewed distribution (1,680 identities with two or
more images, one identity holding hundreds of photos):

* B-cubed precision/recall/F1 averaged per face — stable under singleton noise;
* pairwise precision/recall/F1 from contingency counts — exact, no O(n²) matrix;
* ``merge_rate`` — assigned clusters that span more than one identity, the
  failure single-linkage is famous for;
* ``identity_integrity`` — identities that ended up as exactly one person of
  their own;
* ``identity_merges`` — distinct identity pairs glued together, the number that
  must stay at zero;
* ``unassigned`` — faces no person claimed — and hard-negative statistics, which
  say whether any floor can separate lookalikes at all.

An unassigned face (``-1``) is scored as its own singleton cluster: a missed face
is a split, never a merge, which is how the open-set product behaves.
"""

from __future__ import annotations

import argparse
import base64
import hashlib
import json
import shutil
import sys
import tempfile
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable, Iterable, Mapping, Optional, Sequence

import numpy as np

from app.config import (
    CLUSTER_ALGO,
    CLUSTER_ASSIGN_SIMILARITY,
    CLUSTER_CANDIDATES_PER_PHOTO,
    CLUSTER_CORE_NEIGHBOURS,
    CLUSTER_CORE_SIMILARITY,
    CLUSTER_MERGE_SIMILARITY,
    CLUSTER_MERGE_SUPPORT,
    CLUSTER_MERGE_SUPPORT_SIMILARITY,
    CLUSTER_MIN_PHOTOS,
    CLUSTER_MIN_SIMILARITY,
    EMBEDDING_DIM,
    RECOGNITION_MARGIN,
)
from tools.lfw_calibration import (
    DEFAULT_MODEL_ID,
    PREPROCESSING_VERSION,
    CalibrationError,
    EmbeddingCache,
    LazyEngine,
    default_engine_factory,
    largest_face_embedding,
)

#: Image suffixes the indexer accepts.
IMAGE_SUFFIXES = frozenset({".jpg", ".jpeg", ".png"})
#: Candidate neighbours per face, mirroring ``FACES_CLUSTER_NEIGHBOURS`` in core.
DEFAULT_TOP_K = 16
#: Faces scored per cosine block, bounding the temporary similarity matrix.
BLOCK = 512


@dataclass(frozen=True)
class LabelledImage:
    """One image with its ground-truth identity and its photo group."""

    identity: str
    path: Path
    relative: str
    #: Photo identity: frames of one shot share it, distinct images do not.
    photo: str


@dataclass(frozen=True)
class GraphConfig:
    """How the candidate graph is built (core's kNN step)."""

    floor: float = CLUSTER_MIN_SIMILARITY
    top_k: int = DEFAULT_TOP_K
    per_photo: int = CLUSTER_CANDIDATES_PER_PHOTO


@dataclass(frozen=True)
class ClusterConfig:
    """How the service labels the graph (its own thresholds by default)."""

    algorithm: str = CLUSTER_ALGO
    min_cluster_size: int = 2
    core_similarity: Optional[float] = CLUSTER_CORE_SIMILARITY
    core_neighbours: Optional[int] = CLUSTER_CORE_NEIGHBOURS
    assign_similarity: Optional[float] = CLUSTER_ASSIGN_SIMILARITY
    merge_similarity: Optional[float] = CLUSTER_MERGE_SIMILARITY
    merge_support: Optional[int] = CLUSTER_MERGE_SUPPORT
    support_similarity: Optional[float] = CLUSTER_MERGE_SUPPORT_SIMILARITY
    margin: Optional[float] = RECOGNITION_MARGIN
    min_photos: Optional[int] = CLUSTER_MIN_PHOTOS
    candidates_per_photo: Optional[int] = CLUSTER_CANDIDATES_PER_PHOTO


@dataclass(frozen=True)
class ClusteringScore:
    """How well one configuration grouped one labelled set."""

    algorithm: str
    floor: float
    top_k: int
    faces: int
    identities: int
    clusters: int
    singleton_clusters: int
    unassigned: float
    b3_precision: float
    b3_recall: float
    b3_f1: float
    pairwise_precision: float
    pairwise_recall: float
    pairwise_f1: float
    merge_rate: float
    merged_identity_share: float
    identity_merges: int
    identity_integrity: float
    core_neighbours: Optional[int] = None
    assign_similarity: Optional[float] = None
    merge_similarity: Optional[float] = None

    def as_dict(self) -> dict[str, Any]:
        return {
            "algorithm": self.algorithm,
            "floor": self.floor,
            "topK": self.top_k,
            "coreNeighbours": self.core_neighbours,
            "assignSimilarity": self.assign_similarity,
            "mergeSimilarity": self.merge_similarity,
            "faces": self.faces,
            "identities": self.identities,
            "clusters": self.clusters,
            "singletonClusters": self.singleton_clusters,
            "unassigned": self.unassigned,
            "b3": {
                "precision": self.b3_precision,
                "recall": self.b3_recall,
                "f1": self.b3_f1,
            },
            "pairwise": {
                "precision": self.pairwise_precision,
                "recall": self.pairwise_recall,
                "f1": self.pairwise_f1,
            },
            "mergeRate": self.merge_rate,
            "mergedIdentityShare": self.merged_identity_share,
            "identityMerges": self.identity_merges,
            "identityIntegrity": self.identity_integrity,
        }


@dataclass(frozen=True)
class HardNegativeStats:
    """Best cross-identity similarity per face: the chaining risk at a floor."""

    faces: int
    best: float
    p99: float
    median: float
    over: Mapping[float, int]

    def as_dict(self) -> dict[str, Any]:
        return {
            "faces": self.faces,
            "best": self.best,
            "p99": self.p99,
            "median": self.median,
            "over": {f"{floor:.3f}": count for floor, count in self.over.items()},
        }


# ---------------------------------------------------------------------------
# Labelled index
# ---------------------------------------------------------------------------


def _identity_groups(
    root: Path, flat_names: bool
) -> list[tuple[str, list[Path]]]:
    """Photo files grouped by identity, either by folder or by filename prefix.

    ``flat_names`` reads ``<identity>_<rest>.jpg`` — the shape of the repository
    fixtures (``einstein_frontal.jpg``) — so a labelled set does not have to be
    reorganised into folders first.
    """
    if flat_names:
        grouped: dict[str, list[Path]] = {}
        for path in sorted(root.iterdir()):
            if path.is_file() and path.suffix.lower() in IMAGE_SUFFIXES:
                grouped.setdefault(path.stem.split("_")[0], []).append(path)
        return sorted(grouped.items())

    groups: list[tuple[str, list[Path]]] = []
    for identity_dir in sorted(
        entry for entry in root.iterdir() if entry.is_dir() and not entry.name.startswith(".")
    ):
        files = sorted(
            path
            for path in identity_dir.iterdir()
            if path.is_file() and path.suffix.lower() in IMAGE_SUFFIXES
        )
        groups.append((identity_dir.name, files))
    return groups


def load_labelled_images(
    root: Path,
    *,
    minimum_images: int = 2,
    limit_identities: Optional[int] = None,
    flat_names: bool = False,
) -> list[LabelledImage]:
    """Index a labelled tree, keeping identities with enough photos.

    ``minimum_images=2`` is the honest default for clustering: a person with one
    photo cannot be grouped, so counting those identities would only dilute the
    score. ``limit_identities`` bounds a smoke run.
    """
    if not root.is_dir():
        raise CalibrationError(f"{root} is not a directory")
    images: list[LabelledImage] = []
    for identity, files in _identity_groups(root, flat_names):
        if len(files) < minimum_images:
            continue
        for path in files:
            relative = path.relative_to(root).as_posix()
            images.append(
                LabelledImage(
                    identity=identity,
                    path=path,
                    relative=relative,
                    photo=relative,
                )
            )
        if limit_identities is not None and len(
            {image.identity for image in images}
        ) >= limit_identities:
            break
    if not images:
        raise CalibrationError(
            f"{root} holds no identity with at least {minimum_images} images"
        )
    return images


# ---------------------------------------------------------------------------
# Embeddings
# ---------------------------------------------------------------------------


def embed_labelled_images(
    images: Sequence[LabelledImage],
    cache: EmbeddingCache,
    engine: LazyEngine,
    *,
    log: Callable[[str], None] = lambda _message: None,
) -> list[Optional[np.ndarray]]:
    """Embed every image once, reusing the content-addressed cache."""
    vectors: list[Optional[np.ndarray]] = []
    embedded = 0
    try:
        for image in images:
            raw = image.path.read_bytes()
            digest = hashlib.sha256(raw).hexdigest()
            if cache.has(digest):
                cached = cache.get(digest)
                vectors.append(None if cached is None else np.asarray(cached, dtype=float))
                continue
            faces = engine.get().analyze(base64.b64encode(raw).decode("ascii"))
            embedding = largest_face_embedding(faces)
            cache.put(digest, None if embedding is None else embedding.tolist())
            vectors.append(embedding)
            embedded += 1
            if embedded % 500 == 0:
                # Persist as we go: a long CPU run must not lose hours of work
                # if it is interrupted.
                cache.save()
                log(f"embedded {embedded} images")
    finally:
        cache.save()
    undetected = sum(1 for vector in vectors if vector is None)
    log(
        f"images: {len(vectors)} labelled, {embedded} freshly embedded, "
        f"{undetected} without a detected face"
    )
    return vectors


def normalised_matrix(vectors: Sequence[Optional[np.ndarray]]) -> np.ndarray:
    """Row-normalised embeddings; a missing or zero vector stays all zeros."""
    dimension = next(
        (len(vector) for vector in vectors if vector is not None and len(vector)),
        EMBEDDING_DIM,
    )
    matrix = np.zeros((len(vectors), dimension), dtype=np.float32)
    for index, vector in enumerate(vectors):
        if vector is None:
            continue
        norm = float(np.linalg.norm(vector))
        if norm > 0.0:
            matrix[index] = np.asarray(vector, dtype=np.float32) / norm
    return matrix


# ---------------------------------------------------------------------------
# Candidate graph
# ---------------------------------------------------------------------------


def _neighbour_order(row: np.ndarray, candidates: np.ndarray, take: int) -> np.ndarray:
    """Candidate positions, highest similarity first, at most *take* of them."""
    if candidates.size <= take:
        picks = candidates
    else:
        part = np.argpartition(-row[candidates], take - 1)[:take]
        picks = candidates[part]
    return picks[np.argsort(-row[picks], kind="stable")]


def build_edges(
    vectors: Sequence[Optional[np.ndarray]],
    photos: Sequence[str],
    config: GraphConfig = GraphConfig(),
) -> list[tuple[int, int, float]]:
    """Directed candidate edges, as core's kNN query produces them.

    A face keeps its strongest neighbours at or above ``floor``, at most
    ``top_k`` of them, and at most ``per_photo`` from any one photo — the cap
    that stops a burst from filling the candidate budget.
    """
    if len(vectors) != len(photos):
        raise CalibrationError("every vector needs a photo id")
    matrix = normalised_matrix(vectors)
    count = matrix.shape[0]
    edges: list[tuple[int, int, float]] = []
    for start in range(0, count, BLOCK):
        block = matrix[start : start + BLOCK]
        similarities = block @ matrix.T
        for offset in range(block.shape[0]):
            index = start + offset
            if not np.any(block[offset]):
                continue
            row = similarities[offset]
            candidates = np.nonzero(row >= config.floor)[0]
            candidates = candidates[candidates != index]
            if candidates.size == 0:
                continue
            per_photo = max(1, config.per_photo)
            take = min(candidates.size, max(config.top_k * per_photo, config.top_k))
            picked: dict[str, int] = {}
            accepted = 0
            for neighbour in _neighbour_order(row, candidates, take):
                photo = photos[int(neighbour)]
                if picked.get(photo, 0) >= per_photo:
                    continue
                picked[photo] = picked.get(photo, 0) + 1
                edges.append((index, int(neighbour), float(row[neighbour])))
                accepted += 1
                if accepted >= config.top_k:
                    break
    return edges


def label_faces(
    images: Sequence[LabelledImage],
    edges: Sequence[tuple[int, int, float]],
    config: ClusterConfig = ClusterConfig(),
) -> list[int]:
    """Let the service's own labeller turn the graph into person labels."""
    from app.engine import FaceEngine

    count = len(images)
    return FaceEngine.cluster_graph(
        list(range(count)),
        list(edges),
        [True] * count,
        max(2, int(config.min_cluster_size)),
        photo_ids=list(range(count)),
        algorithm=config.algorithm,
        core_similarity=config.core_similarity,
        core_neighbours=config.core_neighbours,
        assign_similarity=config.assign_similarity,
        merge_similarity=config.merge_similarity,
        merge_support=config.merge_support,
        support_similarity=config.support_similarity,
        margin=config.margin,
        min_photos=config.min_photos,
        candidates_per_photo=config.candidates_per_photo,
    )


# ---------------------------------------------------------------------------
# Metrics
# ---------------------------------------------------------------------------


def _pairs(count: int) -> int:
    return count * (count - 1) // 2


def _contingency(
    predicted: Sequence[int], identities: Sequence[str]
) -> tuple[dict[tuple[int, str], int], dict[int, int], dict[str, int]]:
    """Cluster/identity intersections and both marginals."""
    joint: dict[tuple[int, str], int] = {}
    by_cluster: dict[int, int] = {}
    by_identity: dict[str, int] = {}
    for label, identity in zip(predicted, identities):
        joint[(label, identity)] = joint.get((label, identity), 0) + 1
        by_cluster[label] = by_cluster.get(label, 0) + 1
        by_identity[identity] = by_identity.get(identity, 0) + 1
    return joint, by_cluster, by_identity


def b_cubed(
    predicted: Sequence[int], identities: Sequence[str]
) -> tuple[float, float, float]:
    """B-cubed precision, recall and F1, averaged per face."""
    joint, by_cluster, by_identity = _contingency(predicted, identities)
    count = len(predicted)
    precision = recall = 0.0
    for (label, identity), shared in joint.items():
        precision += shared / by_cluster[label] * shared
        recall += shared / by_identity[identity] * shared
    precision /= count
    recall /= count
    f1 = 2 * precision * recall / (precision + recall) if precision + recall else 0.0
    return precision, recall, f1


def pairwise(
    predicted: Sequence[int], identities: Sequence[str]
) -> tuple[float, float, float]:
    """Pairwise precision, recall and F1 from the contingency counts."""
    joint, by_cluster, by_identity = _contingency(predicted, identities)
    true_positives = float(sum(_pairs(shared) for shared in joint.values()))
    predicted_pairs = float(sum(_pairs(size) for size in by_cluster.values()))
    identity_pairs = float(sum(_pairs(size) for size in by_identity.values()))
    precision = true_positives / predicted_pairs if predicted_pairs else 0.0
    recall = true_positives / identity_pairs if identity_pairs else 0.0
    f1 = 2 * precision * recall / (precision + recall) if precision + recall else 0.0
    return precision, recall, f1


def score_labels(
    predicted: Sequence[int],
    identities: Sequence[str],
    *,
    algorithm: str,
    floor: float,
    top_k: int,
    core_neighbours: Optional[int] = None,
    assign_similarity: Optional[float] = None,
    merge_similarity: Optional[float] = None,
) -> ClusteringScore:
    """Score one labelling; an unassigned face counts as its own singleton."""
    if len(predicted) != len(identities):
        raise CalibrationError("labels and identities must be aligned")
    if not predicted:
        raise CalibrationError("nothing to score")

    effective = [
        label if label >= 0 else -(index + 1) for index, label in enumerate(predicted)
    ]
    b3_precision, b3_recall, b3_f1 = b_cubed(effective, identities)
    pairwise_precision, pairwise_recall, pairwise_f1 = pairwise(effective, identities)

    members: dict[int, list[int]] = {}
    for index, label in enumerate(predicted):
        if label >= 0:
            members.setdefault(label, []).append(index)
    assigned = sum(len(indices) for indices in members.values())
    mixed = [
        indices
        for indices in members.values()
        if len({identities[index] for index in indices}) > 1
    ]
    merges: set[frozenset[str]] = set()
    for indices in mixed:
        present = sorted({identities[index] for index in indices})
        for left in range(len(present)):
            for right in range(left + 1, len(present)):
                merges.add(frozenset((present[left], present[right])))

    intact = 0
    by_identity: dict[str, list[int]] = {}
    for index, identity in enumerate(identities):
        by_identity.setdefault(identity, []).append(index)
    for indices in by_identity.values():
        labels = {predicted[index] for index in indices}
        if len(labels) != 1:
            continue
        label = next(iter(labels))
        if label < 0:
            continue
        # Whole *and* alone: a person sharing its cluster with someone else is
        # not intact, however completely its own photos were grouped.
        if len({identities[index] for index in members[label]}) == 1:
            intact += 1

    return ClusteringScore(
        algorithm=algorithm,
        floor=floor,
        top_k=top_k,
        faces=len(predicted),
        identities=len(by_identity),
        clusters=len(members),
        singleton_clusters=sum(1 for indices in members.values() if len(indices) == 1),
        unassigned=(len(predicted) - assigned) / len(predicted),
        b3_precision=b3_precision,
        b3_recall=b3_recall,
        b3_f1=b3_f1,
        pairwise_precision=pairwise_precision,
        pairwise_recall=pairwise_recall,
        pairwise_f1=pairwise_f1,
        merge_rate=len(mixed) / len(members) if members else 0.0,
        merged_identity_share=(
            sum(len(indices) for indices in mixed) / assigned if assigned else 0.0
        ),
        identity_merges=len(merges),
        identity_integrity=intact / len(by_identity),
        core_neighbours=core_neighbours,
        assign_similarity=assign_similarity,
        merge_similarity=merge_similarity,
    )


def hard_negatives(
    vectors: Sequence[Optional[np.ndarray]],
    identities: Sequence[str],
    *,
    top_k: int = DEFAULT_TOP_K,
    floors: Sequence[float] = (0.30, 0.35, 0.40, 0.45, 0.50),
) -> HardNegativeStats:
    """Best cross-identity similarity per face: the floor a graph cannot beat.

    If many faces have a different person as their closest neighbour above
    ``CLUSTER_MIN_SIMILARITY``, single-linkage will glue those identities no
    matter how the labelling is tuned; the numbers here say how many.
    """
    if len(vectors) != len(identities):
        raise CalibrationError("every vector needs an identity")
    matrix = normalised_matrix(vectors)
    count = matrix.shape[0]
    best: list[float] = []
    for start in range(0, count, BLOCK):
        block = matrix[start : start + BLOCK]
        similarities = block @ matrix.T
        for offset in range(block.shape[0]):
            index = start + offset
            if not np.any(block[offset]):
                best.append(float("-inf"))
                continue
            row = similarities[offset]
            candidates = np.nonzero(row >= 0.0)[0]
            candidates = candidates[candidates != index]
            partner = identities[index]
            foreign = np.array(
                [identities[int(j)] != partner for j in candidates], dtype=bool
            )
            candidates = candidates[foreign]
            if candidates.size == 0:
                best.append(float("-inf"))
                continue
            take = min(candidates.size, top_k)
            top = np.sort(row[candidates])[-take:]
            best.append(float(top.max()))
    finite = [value for value in best if np.isfinite(value)]
    if not finite:
        raise CalibrationError("no cross-identity neighbours found")
    array = np.asarray(finite, dtype=float)
    return HardNegativeStats(
        faces=count,
        best=float(array.max()),
        p99=float(np.percentile(array, 99)),
        median=float(np.median(array)),
        over={float(floor): int(np.count_nonzero(array >= floor)) for floor in floors},
    )


# ---------------------------------------------------------------------------
# Evaluation
# ---------------------------------------------------------------------------


def evaluate(
    images: Sequence[LabelledImage],
    vectors: Sequence[Optional[np.ndarray]],
    *,
    graph: GraphConfig = GraphConfig(),
    cluster: ClusterConfig = ClusterConfig(),
) -> ClusteringScore:
    """Build the graph, label it with the service and score the result."""
    edges = build_edges(vectors, [image.photo for image in images], graph)
    labels = label_faces(images, edges, cluster)
    return score_labels(
        labels,
        [image.identity for image in images],
        algorithm=cluster.algorithm,
        floor=graph.floor,
        top_k=graph.top_k,
        core_neighbours=cluster.core_neighbours,
        assign_similarity=cluster.assign_similarity,
        merge_similarity=cluster.merge_similarity,
    )


def format_score(score: ClusteringScore) -> str:
    """One score as an aligned report table."""
    rows = [
        ("algorithm", score.algorithm),
        ("candidate floor", f"{score.floor:.3f}"),
        ("top-k", str(score.top_k)),
        ("faces / identities", f"{score.faces} / {score.identities}"),
        ("clusters", f"{score.clusters} (singletons {score.singleton_clusters})"),
        ("unassigned", f"{score.unassigned:.4f}"),
        ("B-cubed P / R / F1", f"{score.b3_precision:.4f} / {score.b3_recall:.4f} / {score.b3_f1:.4f}"),
        ("pairwise P / R / F1", f"{score.pairwise_precision:.4f} / {score.pairwise_recall:.4f} / {score.pairwise_f1:.4f}"),
        ("merge rate", f"{score.merge_rate:.4f}"),
        ("merged identity share", f"{score.merged_identity_share:.4f}"),
        ("identity merges", str(score.identity_merges)),
        ("identity integrity", f"{score.identity_integrity:.4f}"),
    ]
    width = max(len(name) for name, _ in rows)
    return "\n".join(f"{name:<{width}}  {value}" for name, value in rows)


def rank(scores: Iterable[ClusteringScore]) -> list[ClusteringScore]:
    """Sort by merge safety first, then by grouping quality."""
    return sorted(
        scores,
        key=lambda score: (
            score.merge_rate,
            score.identity_merges,
            -score.b3_f1,
            -score.identity_integrity,
        ),
    )


def candidate_grid(
    floors: Sequence[float] = (0.30, 0.35, 0.40, 0.45, 0.50),
    core_neighbours: Sequence[int] = (2, 3, 4),
    assign: Sequence[float] = (0.25, 0.30, 0.35),
    merge: Sequence[float] = (0.45, 0.50, 0.55),
) -> list[tuple[GraphConfig, ClusterConfig]]:
    """Graph/labelling combinations a sweep scores on the identical set."""
    combinations: list[tuple[GraphConfig, ClusterConfig]] = []
    for floor in floors:
        graph = GraphConfig(floor=floor)
        for neighbours in core_neighbours:
            for assign_similarity in assign:
                for merge_similarity in merge:
                    combinations.append(
                        (
                            graph,
                            ClusterConfig(
                                algorithm="graph-v2",
                                core_neighbours=neighbours,
                                assign_similarity=assign_similarity,
                                merge_similarity=merge_similarity,
                            ),
                        )
                    )
    return combinations


# ---------------------------------------------------------------------------
# Synthetic bursts (product-like series)
# ---------------------------------------------------------------------------


def synthesize_bursts(
    images: Sequence[LabelledImage],
    root: Path,
    copies: int,
) -> list[LabelledImage]:
    """Write *copies* deterministic variants of every image as separate photos.

    LFW contains one photo per shot, so it cannot exercise ``CLUSTER_MIN_PHOTOS``
    or the burst candidate cap. A burst is emulated by re-encoding the same face
    with a small exposure/crop/quality drift; each variant is its own photo, as
    frames of one event are in production.
    """
    from PIL import Image, ImageEnhance

    if copies < 1:
        raise CalibrationError("a burst needs at least one extra copy")
    bursts: list[LabelledImage] = []
    for image in images:
        with Image.open(image.path) as source:
            picture = source.convert("RGB")
        for copy in range(1, copies + 1):
            variant = ImageEnhance.Brightness(picture).enhance(1.0 + 0.06 * copy)
            variant = ImageEnhance.Contrast(variant).enhance(1.0 - 0.05 * copy)
            width, height = variant.size
            margin = int(min(width, height) * 0.03 * copy)
            if margin and width > 2 * margin and height > 2 * margin:
                variant = variant.crop((margin, margin, width - margin, height - margin))
            target = root / image.identity / f"{image.path.stem}__burst{copy}.jpg"
            target.parent.mkdir(parents=True, exist_ok=True)
            variant.save(target, format="JPEG", quality=max(55, 90 - 10 * copy))
            relative = target.relative_to(root).as_posix()
            bursts.append(
                LabelledImage(
                    identity=image.identity,
                    path=target,
                    relative=relative,
                    photo=f"{image.relative}#burst{copy}",
                )
            )
    return bursts


# ---------------------------------------------------------------------------
# Dataset-agnostic harness (stage 2)
# ---------------------------------------------------------------------------


def run_benchmark(
    root: Path,
    *,
    cache_path: Path,
    engine: LazyEngine,
    graph: GraphConfig = GraphConfig(),
    cluster: ClusterConfig = ClusterConfig(),
    minimum_images: int = 2,
    limit_identities: Optional[int] = None,
    flat_names: bool = False,
    bursts: int = 0,
    burst_root: Optional[Path] = None,
    log: Callable[[str], None] = lambda _message: None,
) -> tuple[ClusteringScore, Optional[ClusteringScore], HardNegativeStats]:
    """Score a labelled tree, optionally with synthetic bursts added.

    Returns the plain score, the burst-augmented score (``None`` without
    bursts) and the hard-negative statistics of the plain set.
    """
    images = load_labelled_images(
        root,
        minimum_images=minimum_images,
        limit_identities=limit_identities,
        flat_names=flat_names,
    )
    cache = EmbeddingCache(cache_path, engine.model_id, PREPROCESSING_VERSION)
    vectors = embed_labelled_images(images, cache, engine, log=log)
    plain = evaluate(images, vectors, graph=graph, cluster=cluster)
    negatives = hard_negatives(vectors, [image.identity for image in images])

    with_bursts: Optional[ClusteringScore] = None
    if bursts > 0:
        burst_dir = burst_root or Path(tempfile.mkdtemp(prefix="flashbacks-bursts-"))
        synthetic = synthesize_bursts(images, burst_dir, bursts)
        extra = embed_labelled_images(synthetic, cache, engine, log=log)
        combined = list(images) + synthetic
        with_bursts = evaluate(
            combined, list(vectors) + extra, graph=graph, cluster=cluster
        )
    return plain, with_bursts, negatives


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------


def _floors_list(raw: str) -> list[float]:
    try:
        return [float(part) for part in raw.split(",") if part.strip()]
    except ValueError as exc:
        raise CalibrationError(f"invalid floor list {raw!r}") from exc


def build_parser() -> argparse.ArgumentParser:
    """Build the ``score``/``hard-negatives`` CLI for any labelled tree."""
    parser = argparse.ArgumentParser(
        prog="tools.clustering_benchmark",
        description=(
            "Score face clustering on a labelled <root>/<identity>/<image> tree "
            "with the service's own engine and labeller."
        ),
    )
    subcommands = parser.add_subparsers(dest="command", required=True)

    score = subcommands.add_parser("score", help="group and score one labelled tree")
    score.add_argument("--images", required=True, help="labelled image tree")
    score.add_argument("--floor", type=float, default=CLUSTER_MIN_SIMILARITY)
    score.add_argument("--top-k", type=int, default=DEFAULT_TOP_K)
    score.add_argument("--algorithm", default=CLUSTER_ALGO)
    score.add_argument("--minimum-images", type=int, default=2)
    score.add_argument("--limit-identities", type=int, default=None)
    score.add_argument(
        "--flat-names",
        action="store_true",
        help="take the identity from the filename prefix (<identity>_<rest>.jpg)",
    )
    score.add_argument("--bursts", type=int, default=0, help="extra variants per image")
    score.add_argument("--work-dir", help="embedding cache directory")
    score.add_argument("--keep", action="store_true", help="keep the work directory")
    score.add_argument("--json-out", help="write the scores as JSON")

    negatives = subcommands.add_parser(
        "hard-negatives", help="measure the cross-identity similarity ceiling"
    )
    negatives.add_argument("--images", required=True, help="labelled image tree")
    negatives.add_argument("--top-k", type=int, default=DEFAULT_TOP_K)
    negatives.add_argument("--minimum-images", type=int, default=2)
    negatives.add_argument("--limit-identities", type=int, default=None)
    negatives.add_argument(
        "--flat-names",
        action="store_true",
        help="take the identity from the filename prefix (<identity>_<rest>.jpg)",
    )
    negatives.add_argument(
        "--floors",
        default="0.30,0.35,0.40,0.45,0.50",
        help="comma-separated floors to count above",
    )
    negatives.add_argument("--work-dir", help="embedding cache directory")
    negatives.add_argument("--keep", action="store_true", help="keep the work directory")
    return parser


def main(
    argv: Optional[Sequence[str]] = None,
    *,
    engine_factory: Optional[Callable[[], Any]] = None,
) -> int:
    """Run one benchmark command; return its exit status."""
    arguments = build_parser().parse_args(argv)
    images_root = Path(arguments.images).expanduser()
    work_dir = (
        Path(arguments.work_dir).expanduser()
        if arguments.work_dir
        else Path(tempfile.mkdtemp(prefix="flashbacks-clustering-"))
    )
    work_dir.mkdir(parents=True, exist_ok=True)
    engine = LazyEngine(engine_factory or default_engine_factory, DEFAULT_MODEL_ID)
    cache_path = work_dir / "embeddings.json"

    try:
        images = load_labelled_images(
            images_root,
            minimum_images=arguments.minimum_images,
            limit_identities=arguments.limit_identities,
            flat_names=getattr(arguments, "flat_names", False),
        )
        embeddings = embed_labelled_images(
            images, EmbeddingCache(cache_path, engine.model_id, PREPROCESSING_VERSION), engine
        )
        if arguments.command == "hard-negatives":
            stats = hard_negatives(
                embeddings,
                [image.identity for image in images],
                top_k=arguments.top_k,
                floors=_floors_list(arguments.floors),
            )
            print(f"faces / identities: {stats.faces} / {len({i.identity for i in images})}")
            print(f"best cross-identity similarity: {stats.best:.4f}")
            print(f"p99 / median: {stats.p99:.4f} / {stats.median:.4f}")
            for floor, count in sorted(stats.over.items()):
                print(f"faces with a foreign neighbour >= {floor:.3f}: {count}")
            _cleanup(work_dir, arguments.keep)
            return 0

        graph = GraphConfig(floor=arguments.floor, top_k=arguments.top_k)
        cluster = ClusterConfig(algorithm=arguments.algorithm)
        plain = evaluate(images, embeddings, graph=graph, cluster=cluster)
        print(format_score(plain))
        payload: dict[str, Any] = {"plain": plain.as_dict()}

        if arguments.bursts > 0:
            synthetic = synthesize_bursts(images, work_dir / "bursts", arguments.bursts)
            extra = embed_labelled_images(
                synthetic,
                EmbeddingCache(cache_path, engine.model_id, PREPROCESSING_VERSION),
                engine,
            )
            bursted = evaluate(
                list(images) + synthetic,
                list(embeddings) + extra,
                graph=graph,
                cluster=cluster,
            )
            print("\nwith synthetic bursts")
            print(format_score(bursted))
            payload["withBursts"] = bursted.as_dict()

        if arguments.json_out:
            Path(arguments.json_out).expanduser().write_text(
                json.dumps(payload, indent=2), encoding="utf-8"
            )
    except CalibrationError as exc:
        print(f"benchmark failed: {exc}", file=sys.stderr)
        if work_dir.exists():
            print(f"work directory retained: {work_dir}", file=sys.stderr)
        return 1

    _cleanup(work_dir, arguments.keep)
    return 0


def _cleanup(work_dir: Path, keep: bool) -> None:
    if keep:
        print(f"work directory kept: {work_dir}")
        return
    shutil.rmtree(work_dir, ignore_errors=True)


if __name__ == "__main__":
    sys.exit(main())
