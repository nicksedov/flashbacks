"""1:N clustering calibration on the LFW identities.

``tools.lfw_calibration`` answers "is 0.25 the right similarity for a 1:1
decision?". This harness answers the next question: given the candidate graph
core builds and the labelling the service applies, does the gallery come back as
the right people? LFW is reused because it is the only labelled, freely
available corpus already wired into the repository — but it is scored as a
closed-set clustering problem, not with the official pairs protocol.

Scope, and why the numbers here are directional rather than final:

* usable ground truth is the 1,680 LFW identities with two or more images
  (9,164 faces); the 4,069 single-photo identities cannot be clustered at all
  and are skipped;
* every identity is one face per image, so ``CLUSTER_MIN_PHOTOS`` and the
  burst candidate cap are not exercised — ``tools.clustering_benchmark``
  synthesises bursts and runs against any labelled tree for that;
* LFW is closed-set and small next to the production gallery, so a threshold
  chosen here still needs gallery evidence before it ships.

Commands
--------

.. code-block:: text

    python -m tools.lfw_clustering score  [--work-dir PATH] [--algorithm ID]
                                          [--floor F] [--top-k K]
                                          [--identities N] [--json-out PATH]
    python -m tools.lfw_clustering sweep  [--work-dir PATH] [--identities N]
    python -m tools.lfw_clustering hard-negatives [--work-dir PATH] [--identities N]

The dataset comes from Kaggle through ``tools.lfw_calibration`` (access token
resolved lazily), and embeddings share its content-addressed cache, so a second
command on the same work directory embeds nothing.
"""

from __future__ import annotations

import argparse
import json
import shutil
import sys
import tempfile
from pathlib import Path
from typing import Any, Callable, Mapping, Optional, Sequence

import numpy as np

from app.config import CLUSTER_ALGO, CLUSTER_MIN_SIMILARITY
from tools.clustering_benchmark import (
    DEFAULT_TOP_K,
    ClusterConfig,
    ClusteringScore,
    GraphConfig,
    LabelledImage,
    build_edges,
    candidate_grid,
    embed_labelled_images,
    evaluate,
    format_score,
    hard_negatives,
    label_faces,
    load_labelled_images,
    rank,
    score_labels,
)
from tools.lfw_calibration import (
    DEFAULT_MODEL_ID,
    IMAGES_DIR,
    PREPROCESSING_VERSION,
    CalibrationError,
    EmbeddingCache,
    LazyEngine,
    LazyKaggleOpener,
    default_engine_factory,
    prepare_dataset,
    resolve_work_dir,
)

#: Default sweep grid: candidate floor x core neighbours x assign x merge floor.
DEFAULT_FLOORS = (0.30, 0.35, 0.40, 0.45, 0.50)
DEFAULT_CORE_NEIGHBOURS = (2, 3, 4)
DEFAULT_ASSIGN = (0.25, 0.30, 0.35)
DEFAULT_MERGE = (0.45, 0.50, 0.55)
#: Hard-negative floors reported by ``hard-negatives``.
DEFAULT_NEGATIVE_FLOORS = (0.30, 0.35, 0.40, 0.45, 0.50)


def lfw_subset(
    work_dir: Path,
    *,
    minimum_images: int = 2,
    limit_identities: Optional[int] = None,
) -> list[LabelledImage]:
    """Images of the LFW identities that clustering can be scored on."""
    return load_labelled_images(
        work_dir / IMAGES_DIR,
        minimum_images=minimum_images,
        limit_identities=limit_identities,
    )


def embed_dataset(
    images: Sequence[LabelledImage],
    work_dir: Path,
    engine: LazyEngine,
    *,
    log: Callable[[str], None] = print,
) -> list[Optional[np.ndarray]]:
    """Embed the subset once, through the shared cache inside *work_dir*."""
    cache = EmbeddingCache(work_dir / "embeddings.json", engine.model_id, PREPROCESSING_VERSION)
    return embed_labelled_images(images, cache, engine, log=log)


def format_sweep(scores: Sequence[ClusteringScore]) -> str:
    """Ranked sweep table; the first row is the recommended configuration."""
    lines = [
        "floor  coreN  assign  merge  clusters  unassigned    b3_f1   pairwise_f1  "
        "merges  integrity"
    ]
    for score in scores:
        core = "-" if score.core_neighbours is None else str(score.core_neighbours)
        assign = (
            "-" if score.assign_similarity is None else f"{score.assign_similarity:.2f}"
        )
        merge = (
            "-" if score.merge_similarity is None else f"{score.merge_similarity:.2f}"
        )
        lines.append(
            f"{score.floor:.3f}  {core:>5}  {assign:>6}  {merge:>5}  "
            f"{score.clusters:>8}  {score.unassigned:>10.4f}  {score.b3_f1:.4f}  "
            f"{score.pairwise_f1:>11.4f}  {score.identity_merges:>6}  "
            f"{score.identity_integrity:.4f}"
        )
    return "\n".join(lines)


def run_score(
    work_dir: Path,
    engine: LazyEngine,
    *,
    graph: GraphConfig,
    cluster: ClusterConfig,
    minimum_images: int,
    limit_identities: Optional[int],
    json_out: Optional[Path],
    max_merge_rate: Optional[float],
    min_b3: Optional[float],
    min_integrity: Optional[float],
    log: Callable[[str], None] = print,
) -> int:
    """Score one configuration and enforce the optional gates."""
    images = lfw_subset(
        work_dir, minimum_images=minimum_images, limit_identities=limit_identities
    )
    log(f"identities: {len({image.identity for image in images})}, faces: {len(images)}")
    vectors = embed_dataset(images, work_dir, engine, log=log)
    score = evaluate(images, vectors, graph=graph, cluster=cluster)
    log("")
    log(format_score(score))

    if json_out is not None:
        json_out.write_text(json.dumps(score.as_dict(), indent=2), encoding="utf-8")

    failures: list[str] = []
    if max_merge_rate is not None and score.merge_rate > max_merge_rate:
        failures.append(
            f"merge rate {score.merge_rate:.4f} exceeds {max_merge_rate:.4f}"
        )
    if min_b3 is not None and score.b3_f1 < min_b3:
        failures.append(f"B-cubed F1 {score.b3_f1:.4f} is below {min_b3:.4f}")
    if min_integrity is not None and score.identity_integrity < min_integrity:
        failures.append(
            f"identity integrity {score.identity_integrity:.4f} is below {min_integrity:.4f}"
        )
    for failure in failures:
        print(f"gate failed: {failure}", file=sys.stderr)
    return 1 if failures else 0


def run_sweep(
    work_dir: Path,
    engine: LazyEngine,
    *,
    minimum_images: int,
    limit_identities: Optional[int],
    json_out: Optional[Path],
    max_merges: Optional[int],
    log: Callable[[str], None] = print,
) -> int:
    """Score the default grid on one embedding set and rank by merge safety.

    LFW contains genuine lookalikes, so a configuration with **zero** identity
    merges does not exist there; the gate is therefore explicit
    (``--max-merges``) and compares against a recorded baseline, not against
    zero.
    """
    images = lfw_subset(
        work_dir, minimum_images=minimum_images, limit_identities=limit_identities
    )
    log(f"identities: {len({image.identity for image in images})}, faces: {len(images)}")
    vectors = embed_dataset(images, work_dir, engine, log=log)
    photos = [image.photo for image in images]
    identities = [image.identity for image in images]

    scores: list[ClusteringScore] = []
    edges_cache: dict[float, list[tuple[int, int, float]]] = {}
    for graph, cluster in candidate_grid(
        DEFAULT_FLOORS, DEFAULT_CORE_NEIGHBOURS, DEFAULT_ASSIGN, DEFAULT_MERGE
    ):
        edges = edges_cache.get(graph.floor)
        if edges is None:
            edges = build_edges(vectors, photos, graph)
            edges_cache[graph.floor] = edges
        labels = label_faces(images, edges, cluster)
        scores.append(
            score_labels(
                labels,
                identities,
                algorithm=cluster.algorithm,
                floor=graph.floor,
                top_k=graph.top_k,
                core_neighbours=cluster.core_neighbours,
                assign_similarity=cluster.assign_similarity,
                merge_similarity=cluster.merge_similarity,
            )
        )
        log(
            f"scored floor={graph.floor:.3f} coreN={cluster.core_neighbours} "
            f"assign={cluster.assign_similarity} merge={cluster.merge_similarity}: "
            f"merges={scores[-1].identity_merges} b3={scores[-1].b3_f1:.4f}"
        )

    ranked = rank(scores)
    log("")
    log(format_sweep(ranked[:15]))
    best = ranked[0]
    log("")
    log(
        f"recommended: floor={best.floor:.3f} b3_f1={best.b3_f1:.4f} "
        f"merges={best.identity_merges} integrity={best.identity_integrity:.4f}"
    )
    if json_out is not None:
        json_out.write_text(
            json.dumps([score.as_dict() for score in ranked], indent=2), encoding="utf-8"
        )

    if max_merges is not None and best.identity_merges > max_merges:
        print(
            f"gate failed: best configuration merges {best.identity_merges} "
            f"identity pairs, above the allowed {max_merges}",
            file=sys.stderr,
        )
        return 1
    return 0


def run_hard_negatives(
    work_dir: Path,
    engine: LazyEngine,
    *,
    minimum_images: int,
    limit_identities: Optional[int],
    top_k: int,
    floors: Sequence[float],
    json_out: Optional[Path],
    log: Callable[[str], None] = print,
) -> int:
    """Report how similar different people get: the ceiling no floor can beat."""
    images = lfw_subset(
        work_dir, minimum_images=minimum_images, limit_identities=limit_identities
    )
    log(f"identities: {len({image.identity for image in images})}, faces: {len(images)}")
    vectors = embed_dataset(images, work_dir, engine, log=log)
    stats = hard_negatives(
        vectors,
        [image.identity for image in images],
        top_k=top_k,
        floors=floors,
    )
    log("")
    log(f"faces: {stats.faces}")
    log(f"best cross-identity similarity : {stats.best:.4f}")
    log(f"p99 cross-identity similarity  : {stats.p99:.4f}")
    log(f"median cross-identity similarity: {stats.median:.4f}")
    for floor, count in sorted(stats.over.items()):
        share = count / stats.faces
        log(f"faces whose nearest foreign neighbour >= {floor:.3f}: {count} ({share:.4f})")
    if json_out is not None:
        json_out.write_text(json.dumps(stats.as_dict(), indent=2), encoding="utf-8")
    return 0


def _floors(raw: str) -> tuple[float, ...]:
    try:
        values = tuple(float(part) for part in raw.split(",") if part.strip())
    except ValueError as exc:
        raise CalibrationError(f"invalid floor list {raw!r}") from exc
    if not values:
        raise CalibrationError("no floors given")
    return values


def build_parser() -> argparse.ArgumentParser:
    """Build the ``score``/``sweep``/``hard-negatives`` CLI."""
    parser = argparse.ArgumentParser(
        prog="tools.lfw_clustering",
        description=(
            "Score 1:N face clustering on the LFW identities with the service's "
            "own engine and labeller, and sweep the graph configuration."
        ),
    )
    subcommands = parser.add_subparsers(dest="command", required=True)

    def add_common(sub: argparse.ArgumentParser) -> None:
        sub.add_argument("--work-dir", help="dataset work directory (or LFW_WORK_DIR)")
        sub.add_argument(
            "--kaggle-token-file",
            metavar="PATH",
            help="Kaggle access token file (default: $KAGGLE_API_TOKEN, then .kaggle/access_token)",
        )
        sub.add_argument("--minimum-images", type=int, default=2)
        sub.add_argument("--identities", type=int, default=None, help="limit a smoke run")
        sub.add_argument(
            "--keep", action="store_true", help="keep the work directory after success"
        )
        sub.add_argument("--json-out", help="write the score as JSON")
        sub.add_argument("--quiet", action="store_true", help="only print failures")

    score = subcommands.add_parser("score", help="score one configuration")
    add_common(score)
    score.add_argument("--floor", type=float, default=CLUSTER_MIN_SIMILARITY)
    score.add_argument("--top-k", type=int, default=DEFAULT_TOP_K)
    score.add_argument("--algorithm", default=CLUSTER_ALGO)
    score.add_argument("--max-merge-rate", type=float, default=None)
    score.add_argument("--min-b3", type=float, default=None)
    score.add_argument("--min-integrity", type=float, default=None)

    sweep = subcommands.add_parser("sweep", help="score the default grid and rank it")
    add_common(sweep)
    sweep.add_argument(
        "--max-merges",
        type=int,
        default=None,
        help="fail when the best configuration merges more identity pairs than this",
    )

    negatives = subcommands.add_parser(
        "hard-negatives", help="measure the cross-identity similarity ceiling"
    )
    add_common(negatives)
    negatives.add_argument("--top-k", type=int, default=DEFAULT_TOP_K)
    negatives.add_argument(
        "--floors", default=",".join(f"{floor:.2f}" for floor in DEFAULT_NEGATIVE_FLOORS)
    )
    return parser


def main(
    argv: Optional[Sequence[str]] = None,
    *,
    opener: Optional[Callable[[str], Any]] = None,
    engine_factory: Optional[Callable[[], Any]] = None,
    environ: Optional[Mapping[str, str]] = None,
) -> int:
    """Run one harness command; return its exit status."""
    arguments = build_parser().parse_args(argv)
    work_dir = resolve_work_dir(arguments.work_dir, environ, create=True)
    downloader = opener or LazyKaggleOpener(
        getattr(arguments, "kaggle_token_file", None), environ
    )
    engine = LazyEngine(engine_factory or default_engine_factory, DEFAULT_MODEL_ID)
    log = (lambda _message: None) if arguments.quiet else print
    results = 1

    try:
        dataset = prepare_dataset(work_dir, downloader)
        log(
            f"dataset: {dataset.images} images; "
            f"scoring identities with >= {arguments.minimum_images} photos"
        )
        json_out = Path(arguments.json_out).expanduser() if arguments.json_out else None
        common: dict[str, Any] = {
            "minimum_images": arguments.minimum_images,
            "limit_identities": arguments.identities,
            "json_out": json_out,
            "log": log,
        }
        if arguments.command == "score":
            results = run_score(
                work_dir,
                engine,
                graph=GraphConfig(floor=arguments.floor, top_k=arguments.top_k),
                cluster=ClusterConfig(algorithm=arguments.algorithm),
                max_merge_rate=arguments.max_merge_rate,
                min_b3=arguments.min_b3,
                min_integrity=arguments.min_integrity,
                **common,
            )
        elif arguments.command == "sweep":
            results = run_sweep(
                work_dir,
                engine,
                max_merges=arguments.max_merges,
                **common,
            )
        elif arguments.command == "hard-negatives":
            results = run_hard_negatives(
                work_dir,
                engine,
                top_k=arguments.top_k,
                floors=_floors(arguments.floors),
                **common,
            )
        else:  # pragma: no cover - argparse restricts the choices
            raise CalibrationError(f"unknown command {arguments.command!r}")
    except CalibrationError as exc:
        print(f"clustering calibration failed: {exc}", file=sys.stderr)
        if work_dir.exists():
            print(f"work directory retained: {work_dir}", file=sys.stderr)
        return 1

    if arguments.keep:
        log(f"work directory kept: {work_dir}")
    elif results == 0:
        shutil.rmtree(work_dir, ignore_errors=True)
        log(f"work directory removed: {work_dir}")
    else:
        print(f"work directory retained: {work_dir}", file=sys.stderr)
    return results


if __name__ == "__main__":
    sys.exit(main())
