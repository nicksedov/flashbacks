"""Calibration harness for the face-clustering variants.

The harness answers one question with numbers instead of anecdotes: does a
clustering change put one identity into one person? It

1. reads a labelled fixture (``manifest.csv`` or a known photo set),
2. embeds every photo exactly once through :class:`~app.engine.FaceEngine`
   and caches the vectors in ``embeddings.json`` next to the manifest, keyed by
   the manifest ``sha256`` column, so re-runs cost nothing and never load the
   model again,
3. rebuilds the candidate graph the core service would send (k nearest
   neighbours above the recognition floor), and
4. scores a clustering variant — fragments per identity, pairwise
   precision/recall, unassigned share and person count.

Run it from the repository root or from ``backend/faces``::

    backend/faces/.venv/bin/python backend/faces/tools/calibrate_clustering.py embed
    backend/faces/.venv/bin/python backend/faces/tools/calibrate_clustering.py baseline

Inside the ``flashbacks-faces`` container the same module is available as
``python -m tools.calibrate_clustering``.
"""

from __future__ import annotations

import argparse
import base64
import csv
import hashlib
import json
import sys
from dataclasses import dataclass, fields
from pathlib import Path
from typing import Any, Callable, Iterable, Mapping, Optional, Sequence

# ``python backend/faces/tools/calibrate_clustering.py`` puts only the tools
# directory on sys.path; the app package lives one level up.
if __package__ in (None, ""):
    sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import numpy as np

from app.config import (
    CLUSTER_ASSIGN_SIMILARITY,
    CLUSTER_CANDIDATES_PER_PHOTO,
    CLUSTER_CORE_NEIGHBOURS,
    CLUSTER_CORE_SIMILARITY,
    CLUSTER_MERGE_SIMILARITY,
    CLUSTER_MERGE_SUPPORT,
    CLUSTER_MIN_PHOTOS,
    RECOGNITION_MARGIN,
    RECOGNITION_MIN_SIMILARITY,
)
from app.engine import FaceEngine

#: Repository root (``backend/faces/tools`` -> ``backend/faces`` -> ``backend``).
REPO_ROOT = Path(__file__).resolve().parents[3]

#: The private labelled fixture used for the A/B report.
DEFAULT_DATASET = REPO_ROOT / "tests" / "dataset" / "faces" / "dima"

#: The impostor sets that make false merges visible (portraits only).
IMPOSTOR_DATASETS = (
    REPO_ROOT / "tests" / "dataset" / "faces" / "politicians",
    REPO_ROOT / "tests" / "dataset" / "faces" / "scientists",
)

#: Name of the embedding cache written next to a fixture.
CACHE_NAME = "embeddings.json"
CACHE_VERSION = 1

#: Candidate budget per face, mirroring ``Options.ClusterNeighbours`` in core.
CANDIDATE_LIMIT = 16

#: At most this many candidates of one source photo enter a neighbour list
#: (design D5); only applied by the v2 graph builder.
CANDIDATES_PER_PHOTO = 2

#: Portrait-only photo sets used to measure the impostor ceiling. Group photos
#: are deliberately absent: the faces on them cannot be attributed to one
#: identity without labels, and the precision check only needs portraits.
PHOTO_SET_IDENTITIES: dict[str, dict[str, str]] = {
    "politicians": {
        "obama_front.jpg": "obama",
        "obama_profile.jpg": "obama",
        "merkel_portrait.jpg": "merkel",
        "macron_portrait.jpg": "macron",
    },
    "scientists": {
        "einstein_frontal.jpg": "einstein",
        "einstein_profile.jpg": "einstein",
        "einstein_1947.jpg": "einstein",
        "curie_portrait.jpg": "curie",
        "bohr_portrait.jpg": "bohr",
    },
}


@dataclass(frozen=True)
class FixtureFace:
    """One labelled face of a fixture.

    ``identity`` is the ground truth (one value per real person), while
    ``label`` is the production ``people.id`` the face was stored under. The
    two differ on the ``dima`` fixture: six production labels, one identity.
    """

    identity: str
    label: str
    file: str
    image_file_id: int
    face_id: int
    confidence: float
    box: dict[str, float]
    faces_on_photo: int
    sha256: str

    @property
    def solo(self) -> bool:
        """Whether the face is the only face of its photo."""
        return self.faces_on_photo <= 1


@dataclass(frozen=True)
class ClusteringScore:
    """Aggregate quality of one clustering over a labelled fixture."""

    fragments: dict[str, int]
    precision: float
    recall: float
    unassigned_share: float
    people: int
    faces: int


def _float(row: Mapping[str, str], key: str, default: float = 0.0) -> float:
    raw = (row.get(key) or "").strip()
    if not raw:
        return default
    try:
        return float(raw)
    except ValueError:
        return default


def _int(row: Mapping[str, str], key: str, default: int = 0) -> int:
    raw = (row.get(key) or "").strip()
    if not raw:
        return default
    try:
        return int(raw)
    except ValueError:
        return default


def file_sha256(path: Path) -> str:
    """Return the hex SHA-256 of *path*."""
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def load_manifest(manifest_path: Path) -> list[FixtureFace]:
    """Read a fixture manifest, one entry per labelled face.

    The manifest may carry an explicit ``identity`` column. Without it the
    fixture folder name is the identity — that is what makes the ``dima`` set
    meaningful, because its ``person_id`` column holds the six *fragments* the
    old clustering produced, not six different people.
    """
    with manifest_path.open(newline="", encoding="utf-8") as handle:
        rows = list(csv.DictReader(handle))
    if not rows:
        return []

    dataset = manifest_path.resolve().parent.name
    has_identity = "identity" in rows[0]
    faces: list[FixtureFace] = []
    for row in rows:
        identity = (row.get("identity") or "").strip() if has_identity else ""
        faces.append(
            FixtureFace(
                identity=identity or dataset,
                label=(row.get("person_id") or dataset).strip(),
                file=row["file"],
                image_file_id=_int(row, "image_file_id"),
                face_id=_int(row, "face_id"),
                confidence=_float(row, "confidence"),
                box={
                    "x": _float(row, "x"),
                    "y": _float(row, "y"),
                    "width": _float(row, "width"),
                    "height": _float(row, "height"),
                },
                faces_on_photo=_int(row, "faces_on_photo", default=1),
                sha256=(row.get("sha256") or "").strip(),
            )
        )
    return faces


def load_photo_set(dataset_dir: Path) -> list[FixtureFace]:
    """Read a portrait-only photo set described by :data:`PHOTO_SET_IDENTITIES`.

    A file absent from the mapping (a group photo) is skipped: its faces span
    several identities and cannot serve as ground truth. Identities are
    prefixed with the folder name so two sets can be scored together.
    """
    mapping = PHOTO_SET_IDENTITIES.get(dataset_dir.name)
    if mapping is None:
        raise ValueError(
            f"{dataset_dir} has no manifest.csv and is not a known photo set; "
            f"expected one of {sorted(PHOTO_SET_IDENTITIES)}"
        )

    faces: list[FixtureFace] = []
    for index, filename in enumerate(sorted(mapping)):
        path = dataset_dir / filename
        if not path.is_file():
            continue
        prefix = dataset_dir.name
        faces.append(
            FixtureFace(
                identity=f"{prefix}:{mapping[filename]}",
                label=mapping[filename],
                file=filename,
                image_file_id=index + 1,
                face_id=index + 1,
                confidence=0.0,
                box={},
                faces_on_photo=1,
                sha256=file_sha256(path),
            )
        )
    return faces


def load_fixture(dataset_dir: Path) -> list[FixtureFace]:
    """Load a fixture from *dataset_dir* (manifest first, then known sets)."""
    manifest = dataset_dir / "manifest.csv"
    if manifest.is_file():
        return load_manifest(manifest)
    return load_photo_set(dataset_dir)


def _read_cache(cache_path: Path) -> dict[str, Any]:
    if not cache_path.is_file():
        return {"version": CACHE_VERSION, "embeddings": {}}
    try:
        payload = json.loads(cache_path.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return {"version": CACHE_VERSION, "embeddings": {}}
    if not isinstance(payload, dict):
        return {"version": CACHE_VERSION, "embeddings": {}}
    embeddings = payload.get("embeddings")
    if not isinstance(embeddings, dict):
        payload["embeddings"] = {}
    return payload


def _manifest_digest(manifest_path: Path) -> str:
    if not manifest_path.is_file():
        return ""
    return file_sha256(manifest_path)


def _write_cache(
    cache_path: Path, cache: dict[str, Any], dataset_dir: Path, manifest_path: Path
) -> None:
    payload = {
        "version": CACHE_VERSION,
        "manifestSha256": _manifest_digest(manifest_path),
        "embeddings": cache.get("embeddings", {}),
    }
    cache_path.parent.mkdir(parents=True, exist_ok=True)
    cache_path.write_text(json.dumps(payload, indent=1), encoding="utf-8")


def _iou(a: tuple[float, float, float, float], b: tuple[float, float, float, float]) -> float:
    ix1, iy1 = max(a[0], b[0]), max(a[1], b[1])
    ix2, iy2 = min(a[2], b[2]), min(a[3], b[3])
    if ix2 <= ix1 or iy2 <= iy1:
        return 0.0
    intersection = (ix2 - ix1) * (iy2 - iy1)
    union = (a[2] - a[0]) * (a[3] - a[1]) + (b[2] - b[0]) * (b[3] - b[1]) - intersection
    if union <= 0.0:
        return 0.0
    return intersection / union


def _match_description(
    descriptions: Sequence[Mapping[str, Any]], box: Mapping[str, float]
) -> Optional[Mapping[str, Any]]:
    """Return the analysed face that overlaps *box* the most.

    An empty box (portrait photo sets) selects the largest detected face, which
    is the subject of a portrait.
    """
    if not descriptions:
        return None
    if not box or not float(box.get("width", 0.0)) or not float(box.get("height", 0.0)):
        return max(
            descriptions,
            key=lambda face: float(face.get("width", 0.0)) * float(face.get("height", 0.0)),
        )

    target = (
        float(box.get("x", 0.0)),
        float(box.get("y", 0.0)),
        float(box.get("x", 0.0)) + float(box.get("width", 0.0)),
        float(box.get("y", 0.0)) + float(box.get("height", 0.0)),
    )
    best: Optional[Mapping[str, Any]] = None
    best_iou = 0.0
    for face in descriptions:
        candidate = (
            float(face.get("x", 0.0)),
            float(face.get("y", 0.0)),
            float(face.get("x", 0.0)) + float(face.get("width", 0.0)),
            float(face.get("y", 0.0)) + float(face.get("height", 0.0)),
        )
        score = _iou(candidate, target)
        if score > best_iou:
            best, best_iou = face, score
    return best


def ensure_embeddings(
    faces: Sequence[FixtureFace],
    dataset_dir: Path,
    cache_path: Path,
    *,
    engine_factory: Optional[Callable[[], Any]] = None,
    force: bool = False,
    out: Callable[[str], None] = print,
) -> dict[str, list[float]]:
    """Return one embedding per face, embedding only the cache misses.

    ``engine_factory`` is only called when at least one photo must be analysed,
    so a cache hit proves that no model was loaded. The cache maps each photo's
    manifest ``sha256`` to its embedding vector.
    """
    cache = _read_cache(cache_path)
    embeddings: dict[str, list[float]] = cache.setdefault("embeddings", {})
    missing = [face for face in faces if force or face.sha256 not in embeddings]

    if not missing:
        out("cache hit")
        return embeddings

    engine = (engine_factory or FaceEngine)()
    manifest_path = dataset_dir / "manifest.csv"
    embedded = 0
    for face in missing:
        path = dataset_dir / face.file
        image_b64 = base64.b64encode(path.read_bytes()).decode()
        match = _match_description(engine.analyze(image_b64), face.box)
        if match is None:
            raise ValueError(f"no analysed face overlapped the box of {face.file}")
        embeddings[face.sha256] = [float(value) for value in match["embedding"]]
        embedded += 1

    _write_cache(cache_path, cache, dataset_dir, manifest_path)
    out(f"{embedded} faces embedded, cache written")
    return embeddings


def _normalised(vectors: Iterable[Sequence[float]]) -> np.ndarray:
    array = np.asarray(list(vectors), dtype=float)
    norms = np.linalg.norm(array, axis=1, keepdims=True)
    norms[norms == 0] = 1.0
    return array / norms


def candidate_edges(
    embeddings: Sequence[Sequence[float]],
    face_ids: Sequence[int],
    *,
    limit: int = CANDIDATE_LIMIT,
    floor: float = RECOGNITION_MIN_SIMILARITY,
    photo_ids: Optional[Sequence[int]] = None,
    max_per_photo: Optional[int] = None,
) -> list[tuple[int, int, float]]:
    """Rebuild the candidate links the core service would query.

    Core asks pgvector for each face's nearest faces inside the recognition
    radius, capped at ``ClusterNeighbours`` (16). The harness reproduces that
    query from the cached vectors. ``photo_ids``/``max_per_photo`` additionally
    reproduce the v2 photo-diversity rule (at most two candidates per source
    photo).
    """
    count = len(embeddings)
    if count == 0:
        return []
    matrix = _normalised(embeddings)
    similarities = matrix @ matrix.T

    edges: list[tuple[int, int, float]] = []
    for source in range(count):
        order = np.argsort(-similarities[source])
        per_photo: dict[int, int] = {}
        taken = 0
        for candidate in order:
            if int(candidate) == source:
                continue
            value = float(similarities[source][int(candidate)])
            if value < floor:
                break
            if photo_ids is not None and max_per_photo is not None:
                photo = int(photo_ids[int(candidate)])
                if per_photo.get(photo, 0) >= max_per_photo:
                    continue
                per_photo[photo] = per_photo.get(photo, 0) + 1
            edges.append((int(face_ids[source]), int(face_ids[int(candidate)]), value))
            taken += 1
            if taken >= limit:
                break
    return edges


def fragments_per_identity(
    labels: Sequence[int], identities: Sequence[str]
) -> dict[str, int]:
    """Number of distinct people each ground-truth identity is split into.

    Unassigned faces (label ``-1``) do not create a fragment: they are counted
    by the unassigned share instead.
    """
    grouped: dict[str, set[int]] = {}
    for label, identity in zip(labels, identities):
        if label < 0:
            continue
        grouped.setdefault(identity, set()).add(int(label))
    return {identity: len(clusters) for identity, clusters in grouped.items()}


def pairwise_scores(
    labels: Sequence[int], identities: Sequence[str]
) -> tuple[float, float]:
    """Pairwise precision and recall over the assigned faces.

    Precision is the share of same-person pairs that share an identity (false
    merges lower it); recall is the share of same-identity pairs placed in one
    person (fragments lower it). When a fixture has no same-person pair or no
    same-identity pair the corresponding metric is reported as ``1.0``.
    """
    true_positive = false_positive = false_negative = predicted_positive = 0
    count = len(labels)
    for left in range(count):
        if labels[left] < 0:
            continue
        for right in range(left + 1, count):
            if labels[right] < 0:
                continue
            same_identity = identities[left] == identities[right]
            same_person = labels[left] == labels[right]
            if same_person:
                predicted_positive += 1
            if same_person and same_identity:
                true_positive += 1
            elif same_person:
                false_positive += 1
            elif same_identity:
                false_negative += 1

    precision = true_positive / predicted_positive if predicted_positive else 1.0
    positives = true_positive + false_negative
    recall = true_positive / positives if positives else 1.0
    return precision, recall


def unassigned_share(labels: Sequence[int]) -> float:
    """Share of faces the variant left without a person."""
    if not labels:
        return 0.0
    return sum(1 for label in labels if label < 0) / len(labels)


def person_count(labels: Sequence[int]) -> int:
    """Number of distinct people the variant produced."""
    return len({label for label in labels if label >= 0})


def score_labels(
    labels: Sequence[int], faces: Sequence[FixtureFace]
) -> ClusteringScore:
    """Score one label assignment against the fixture labels."""
    identities = [face.identity for face in faces]
    precision, recall = pairwise_scores(labels, identities)
    return ClusteringScore(
        fragments=fragments_per_identity(labels, identities),
        precision=precision,
        recall=recall,
        unassigned_share=unassigned_share(labels),
        people=person_count(labels),
        faces=len(labels),
    )


def production_label_table(
    labels: Sequence[int], faces: Sequence[FixtureFace]
) -> list[dict[str, Any]]:
    """Per production label: faces, clusters, largest cluster, unassigned."""
    rows: dict[str, dict[str, Any]] = {}
    for label, face in zip(labels, faces):
        row = rows.setdefault(
            face.label,
            {"label": face.label, "faces": 0, "clusters": set(), "largest": 0, "unassigned": 0},
        )
        row["faces"] += 1
        if label < 0:
            row["unassigned"] += 1
            continue
        row["clusters"].add(int(label))

    counts: dict[tuple[str, int], int] = {}
    for label, face in zip(labels, faces):
        if label >= 0:
            key = (face.label, int(label))
            counts[key] = counts.get(key, 0) + 1

    table: list[dict[str, Any]] = []
    for label in sorted(rows):
        row = rows[label]
        largest = max(
            (count for (owner, _), count in counts.items() if owner == label), default=0
        )
        table.append(
            {
                "label": label,
                "faces": row["faces"],
                "clusters": len(row["clusters"]),
                "largest": largest,
                "unassigned": row["unassigned"],
            }
        )
    return table


@dataclass(frozen=True)
class VariantParams:
    """One point of the threshold grid."""

    candidates: int = CANDIDATE_LIMIT
    core_similarity: float = CLUSTER_CORE_SIMILARITY
    core_neighbours: int = CLUSTER_CORE_NEIGHBOURS
    assign_similarity: float = CLUSTER_ASSIGN_SIMILARITY
    merge_similarity: float = CLUSTER_MERGE_SIMILARITY
    merge_support: int = CLUSTER_MERGE_SUPPORT
    margin: float = RECOGNITION_MARGIN
    min_photos: int = CLUSTER_MIN_PHOTOS
    candidates_per_photo: int = CLUSTER_CANDIDATES_PER_PHOTO


def baseline_labels(
    faces: Sequence[FixtureFace], embeddings: Mapping[str, Sequence[float]]
) -> list[int]:
    """Label the fixture with the current (v1) algorithm and thresholds."""
    face_ids = [face.face_id for face in faces]
    vectors = [embeddings[face.sha256] for face in faces]
    solo = [face.solo for face in faces]
    edges = candidate_edges(vectors, face_ids)
    return FaceEngine.cluster_graph(face_ids, edges, solo, 2)


def variant_labels(
    faces: Sequence[FixtureFace],
    embeddings: Mapping[str, Sequence[float]],
    params: VariantParams,
) -> list[int]:
    """Label the fixture with the density-aware variant and *params*."""
    face_ids = [face.face_id for face in faces]
    photo_ids = [face.image_file_id for face in faces]
    vectors = [embeddings[face.sha256] for face in faces]
    edges = candidate_edges(
        vectors,
        face_ids,
        limit=params.candidates,
        photo_ids=photo_ids,
        max_per_photo=params.candidates_per_photo,
    )
    return FaceEngine.cluster_graph(
        face_ids,
        edges,
        [face.solo for face in faces],
        2,
        photo_ids=photo_ids,
        algorithm="graph-v2",
        core_similarity=params.core_similarity,
        core_neighbours=params.core_neighbours,
        assign_similarity=params.assign_similarity,
        merge_similarity=params.merge_similarity,
        merge_support=params.merge_support,
        margin=params.margin,
        min_photos=params.min_photos,
        candidates_per_photo=params.candidates_per_photo,
    )


def run_variant(
    dataset_dirs: Sequence[Path], params: VariantParams, *, force: bool = False
) -> None:
    """Print the graph-v2 score of every supplied fixture."""
    for dataset_dir in dataset_dirs:
        faces = load_fixture(dataset_dir)
        embeddings = ensure_embeddings(
            faces, dataset_dir, dataset_dir / CACHE_NAME, force=force
        )
        score = score_labels(variant_labels(faces, embeddings, params), faces)
        print(f"\ngraph-v2 — {dataset_dir.name}")
        print(_score_line(dataset_dir.name, score))
        _print_label_table(
            production_label_table(variant_labels(faces, embeddings, params), faces)
        )


def _dominates(left: ClusteringScore, right: ClusteringScore) -> bool:
    """Whether *left* is at least as good on every axis, and better on one."""
    left_fragments = sum(left.fragments.values())
    right_fragments = sum(right.fragments.values())
    better_or_equal = (
        left_fragments <= right_fragments
        and left.precision >= right.precision
        and left.unassigned_share <= right.unassigned_share
    )
    strictly_better = (
        left_fragments < right_fragments
        or left.precision > right.precision
        or left.unassigned_share < right.unassigned_share
    )
    return better_or_equal and strictly_better


def pareto_front(
    results: Sequence[tuple[VariantParams, ClusteringScore]]
) -> list[tuple[VariantParams, ClusteringScore]]:
    """Points no other point beats on fragments, precision and unassigned share."""
    front: list[tuple[VariantParams, ClusteringScore]] = []
    for params, score in results:
        if any(
            _dominates(other_score, score)
            for other_params, other_score in results
            if other_params != params
        ):
            continue
        front.append((params, score))
    front.sort(
        key=lambda item: (
            sum(item[1].fragments.values()),
            -item[1].precision,
            item[1].unassigned_share,
        )
    )
    return front


def sweep(dataset_dirs: Sequence[Path]) -> None:
    """Sweep the graph-v2 thresholds and print the Pareto front.

    The labelled fixture is scored first; the impostor sets constrain the
    front to variants that never merge two different people.
    """
    fixtures = [(dataset_dir, load_fixture(dataset_dir)) for dataset_dir in dataset_dirs]
    embeddings = {
        dataset_dir: ensure_embeddings(faces, dataset_dir, dataset_dir / CACHE_NAME)
        for dataset_dir, faces in fixtures
    }

    results: list[tuple[VariantParams, ClusteringScore]] = []
    for candidates in (8, 16, 24):
        for core_similarity in (-0.05, 0.0, 0.05, 0.1):
            for core_neighbours in (2, 3, 4):
                for assign_similarity in (0.25, 0.30, 0.35):
                    for merge_similarity in (0.45, 0.50, 0.55):
                        for margin in (0.03, 0.05, 0.08):
                            for merge_support in (2, 3):
                                params = VariantParams(
                                    candidates=candidates,
                                    core_similarity=core_similarity,
                                    core_neighbours=core_neighbours,
                                    assign_similarity=assign_similarity,
                                    merge_similarity=merge_similarity,
                                    merge_support=merge_support,
                                    margin=margin,
                                )
                                faces = fixtures[0][1]
                                score = score_labels(
                                    variant_labels(faces, embeddings[fixtures[0][0]], params),
                                    faces,
                                )
                                results.append((params, score))

    def impostors_clean(params: VariantParams) -> bool:
        for dataset_dir, faces in fixtures[1:]:
            score = score_labels(
                variant_labels(faces, embeddings[dataset_dir], params), faces
            )
            if score.precision < 1.0:
                return False
        return True

    clean = [
        (params, score) for params, score in results if impostors_clean(params)
    ]
    front = pareto_front(clean)

    # Many grid points are equally good; show one representative per distinct
    # outcome, preferring the parameters closest to the configured defaults.
    defaults = VariantParams()

    def distance(params: VariantParams) -> float:
        return sum(
            abs(getattr(params, field.name) - getattr(defaults, field.name))
            for field in fields(params)
        )

    representatives: dict[tuple, tuple[VariantParams, ClusteringScore]] = {}
    for params, score in front:
        key = (
            sum(score.fragments.values()),
            round(score.precision, 3),
            round(score.unassigned_share, 3),
            score.people,
        )
        current = representatives.get(key)
        if current is None or distance(params) < distance(current[0]):
            representatives[key] = (params, score)

    ordered = sorted(
        representatives.values(),
        key=lambda item: (
            sum(item[1].fragments.values()),
            -item[1].precision,
            item[1].unassigned_share,
            distance(item[0]),
        ),
    )
    print(
        f"\nPareto front ({len(front)} of {len(results)} combinations, "
        f"impostor precision 1.0 on {len(fixtures) - 1} sets; "
        f"{len(ordered)} distinct outcomes)"
    )
    header = (
        f"{'k':>3}{'core':>7}{'kcore':>7}{'assign':>8}{'merge':>7}{'m':>3}"
        f"{'margin':>8}{'frag':>6}{'prec':>7}{'recall':>8}{'unassigned':>12}{'people':>8}"
    )
    print(header)
    for params, score in ordered[:20]:
        fragments = sum(score.fragments.values())
        print(
            f"{params.candidates:>3}{params.core_similarity:>7.2f}"
            f"{params.core_neighbours:>7}{params.assign_similarity:>8.2f}"
            f"{params.merge_similarity:>7.2f}{params.merge_support:>3}"
            f"{params.margin:>8.2f}{fragments:>6}{score.precision:>7.3f}"
            f"{score.recall:>8.3f}{score.unassigned_share:>12.1%}{score.people:>8}"
        )


def _score_line(name: str, score: ClusteringScore) -> str:
    fragments = ", ".join(f"{identity}={count}" for identity, count in sorted(score.fragments.items()))
    return (
        f"{name}: faces={score.faces} people={score.people} "
        f"fragments[{fragments}] precision={score.precision:.3f} "
        f"recall={score.recall:.3f} unassigned={score.unassigned_share:.1%}"
    )


def _print_label_table(rows: Sequence[Mapping[str, Any]]) -> None:
    print(f"{'label':<10}{'faces':>7}{'clusters':>10}{'largest':>9}{'unassigned':>12}")
    for row in rows:
        print(
            f"{str(row['label']):<10}{row['faces']:>7}{row['clusters']:>10}"
            f"{row['largest']:>9}{row['unassigned']:>12}"
        )


def stored_labels(faces: Sequence[FixtureFace]) -> list[int]:
    """Label every face with the production partition recorded in the manifest.

    This is the true "before" of the calibration: the stored gallery already
    holds one person per production label, so ``dima`` starts as six people.
    A single-batch re-run of the old algorithm cannot reproduce that number —
    the fragments came from repeated passes and the cap-driven split of a
    2 900-face group, neither of which exists in a 64-face sample — so the
    stored partition is reported alongside the re-run.
    """
    by_label: dict[str, int] = {}
    labels: list[int] = []
    for face in faces:
        labels.append(by_label.setdefault(face.label, len(by_label)))
    return labels


def run_baseline(dataset_dirs: Sequence[Path], *, force: bool = False) -> None:
    """Print the stored and re-run baselines over every supplied fixture."""
    for dataset_dir in dataset_dirs:
        faces = load_fixture(dataset_dir)
        embeddings = ensure_embeddings(faces, dataset_dir, dataset_dir / CACHE_NAME, force=force)
        labels = baseline_labels(faces, embeddings)
        score = score_labels(labels, faces)
        stored = score_labels(stored_labels(faces), faces)

        print(f"\nBaseline — {dataset_dir.name}")
        print(_score_line("stored production partition", stored))
        print(_score_line("graph-v1 single pass", score))
        _print_label_table(production_label_table(labels, faces))


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "stage",
        nargs="?",
        default="baseline",
        choices=("embed", "baseline", "v2", "sweep"),
        help=(
            "embed (cache only), baseline (stored + graph-v1), v2 (density-aware "
            "variant at the configured thresholds) or sweep (Pareto front)"
        ),
    )
    parser.add_argument(
        "--with-impostors",
        action="store_true",
        help="also score the politicians and scientists portrait sets",
    )
    parser.add_argument(
        "--dataset",
        action="append",
        type=Path,
        dest="datasets",
        help="fixture directory; repeat to score several sets (default: the dima fixture)",
    )
    parser.add_argument(
        "--force",
        action="store_true",
        help="re-embed every photo even when the cache already holds it",
    )
    return parser


def main(argv: Optional[Sequence[str]] = None) -> int:
    args = build_parser().parse_args(argv)
    datasets = list(args.datasets or [DEFAULT_DATASET])

    if args.stage == "embed":
        for dataset_dir in datasets:
            faces = load_fixture(dataset_dir)
            ensure_embeddings(
                faces, dataset_dir, dataset_dir / CACHE_NAME, force=args.force
            )
        return 0

    if args.stage == "v2":
        run_variant(datasets, VariantParams(), force=args.force)
        return 0

    if args.stage == "sweep":
        sweep(datasets if args.datasets else [DEFAULT_DATASET, *IMPOSTOR_DATASETS])
        return 0

    run_baseline(datasets, force=args.force)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
