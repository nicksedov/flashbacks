"""Runtime settings and ONNX Runtime execution-provider selection.

The provider decision is made in one place so the model session, the
``ctx_id`` InsightFace receives and the value ``/faces/health`` reports can
never disagree:

* ``FACES_PROVIDERS`` is an ordered, comma-separated list accepting shorthand
  (``cpu``, ``cuda``) and full ONNX Runtime names.
* The requested list is intersected with the providers the installed runtime
  actually offers; requesting CUDA on a CPU-only host degrades to CPU with a
  warning instead of failing the request.
* ``ctx_id`` is derived from the resolved list: a CUDA provider first means the
  GPU identified by ``FACES_GPU_ID`` (default ``0``), anything else means
  ``-1`` (CPU).
"""

from __future__ import annotations

import logging
import os
from dataclasses import dataclass
from typing import Mapping, Optional, Sequence

logger = logging.getLogger("faces.config")

#: ArcFace (w600k_r50) produces 512-dimensional embeddings.
EMBEDDING_DIM = 512

#: Minimum cosine similarity for two faces to be treated as the same person.
#: Online photos of one person score well above it, two different people well
#: below it; ``cluster`` turns it into a DBSCAN cosine-distance radius.
CLUSTER_MIN_SIMILARITY = 0.4

#: A face narrower than this many pixels embeds poorly: the recognizer aligns it
#: onto a 112x112 canvas from very few real pixels, so its vector drifts away
#: from the same person's portrait. Such a face is re-embedded from an upscaled
#: padded crop (measured on the G7 group photo: a 21 px wide face scores 0.29
#: against its portrait this way, 0.22 when embedded from the full frame).
SMALL_FACE_WIDTH = 64

#: Padded crop (a multiple of the face box) and target face width used when
#: re-embedding a small face. Two box widths of context keeps the detector from
#: clipping the chin and forehead the recognizer aligns on.
UPSCALE_PADDING = 2.0
UPSCALE_TARGET_WIDTH = 160

#: How much the face detected in the upscaled crop must overlap the expected
#: position of the original detection. A dense group photo puts other faces
#: inside the padded crop, and without this guard the neighbouring face's
#: embedding was used, merging different people into one.
UPSCALE_MIN_IOU = 0.5

#: Minimum cosine similarity for linking a leftover face to an existing group.
#: Below ``CLUSTER_MIN_SIMILARITY`` because a small face never reaches it, above
#: the highest cosine measured between two different people in the calibration
#: set (0.15).
RECOGNITION_MIN_SIMILARITY = 0.25

#: How much the best match must beat the runner-up before a leftover face is
#: linked: an ambiguous match stays unassigned instead of guessing.
RECOGNITION_MARGIN = 0.05

#: Largest share of the gallery one person may hold before the group is treated
#: as chained. Single-linkage follows a chain of lookalikes and merges the whole
#: library into one person as soon as a few faces bridge two identities, so a
#: group above this share is re-split at a stricter similarity floor.
#:
#: Measured on a 39,024-face production gallery: the strict pass chains
#: everything into one group of 32,442, while a 10% cap re-splits it at
#: similarity 0.70 into groups whose largest holds 3,167 faces and leaves 61.5%
#: of the faces in a person. A tighter cap only buys smaller groups at the price
#: of a much stricter floor and far more unassigned faces.
CLUSTER_MAX_SHARE = 0.10

#: Floor of the share above: a small gallery must not split a legitimate person,
#: so the cap never drops below this many faces.
CLUSTER_MAX_FACES_MIN = 250

#: Similarity step used when a chained group is re-split, and the floor at which
#: the split stops. At this similarity two faces are the same person by any
#: reasonable calibration, so a group that survives it is kept as one person.
CLUSTER_SPLIT_STEP = 0.05
CLUSTER_SPLIT_MAX_SIMILARITY = 0.9

#: Labelling algorithm revision. ``graph-v1`` keeps today's single-linkage
#: behaviour and is the default until the calibrated ``graph-v2`` variant is
#: enabled with ``FACES_CLUSTER_ALGO=graph-v2`` (see ``docs/faces/clustering-calibration.md``).
CLUSTER_ALGO = "graph-v1"
CLUSTER_ALGOS = ("graph-v1", "graph-v2")

#: ``graph-v2``: floor of the hub-corrected (CSLS) similarity between two
#: *mutual* candidates for the edge to join a component. The scale differs from
#: a raw cosine: a pair at its own neighbourhood mean scores 0, so the floor sits
#: on that mean rather than near the raw 0.40 floor. Recalibrated by
#: ``backend/faces/tools/calibrate_clustering.py sweep``.
CLUSTER_CORE_SIMILARITY = 0.0

#: ``graph-v2``: mutual neighbours at ``CLUSTER_CORE_SIMILARITY`` a face needs
#: before it may form (or join) a component. A face below it cannot bridge two
#: identities; it is assigned to a component later or stays unassigned.
CLUSTER_CORE_NEIGHBOURS = 3

#: ``graph-v2``: similarity floor for assigning a non-core face to an already
#: formed component, and the floor core uses to match a face against an existing
#: person's reference set. The runner-up margin (``RECOGNITION_MARGIN``) applies
#: on top, so an ambiguous match stays unassigned.
CLUSTER_ASSIGN_SIMILARITY = 0.30

#: ``graph-v2``: medoid similarity two components need before the merge stage
#: joins them.
CLUSTER_MERGE_SIMILARITY = 0.50

#: ``graph-v2``: number of mutual, photo-distinct cross-component pairs that must
#: clear ``CLUSTER_MERGE_SUPPORT_SIMILARITY`` before two components merge.
CLUSTER_MERGE_SUPPORT = 2

#: ``graph-v2``: similarity floor of one supporting cross-component pair.
CLUSTER_MERGE_SUPPORT_SIMILARITY = CLUSTER_MIN_SIMILARITY

#: ``graph-v2``: candidate faces kept per source photo in one neighbour list, so
#: a burst sequence cannot fill the candidate budget.
CLUSTER_CANDIDATES_PER_PHOTO = 2

#: ``graph-v2``: distinct photos a person must span. A cluster made of one
#: photo's burst is not a person.
CLUSTER_MIN_PHOTOS = 2

#: The always-available provider every host runs on.
CPU_PROVIDER = "CPUExecutionProvider"

#: Shorthand -> canonical ONNX Runtime provider name.
PROVIDER_SHORTHANDS: dict[str, str] = {
    "cpu": CPU_PROVIDER,
    "cuda": "CUDAExecutionProvider",
    "tensorrt": "TensorrtExecutionProvider",
    "openvino": "OpenVINOExecutionProvider",
    "directml": "DmlExecutionProvider",
    "coreml": "CoreMLExecutionProvider",
}

#: Canonical provider name -> shorthand used in health and log lines.
SHORTHAND_BY_PROVIDER: dict[str, str] = {
    provider: shorthand for shorthand, provider in PROVIDER_SHORTHANDS.items()
}

#: Providers that execute on a CUDA device and therefore need a ``ctx_id``.
CUDA_PROVIDERS = frozenset({"CUDAExecutionProvider", "TensorrtExecutionProvider"})


def expand_provider(name: str) -> str:
    """Return the canonical provider name for a shorthand or full name."""
    trimmed = name.strip()
    return PROVIDER_SHORTHANDS.get(trimmed.lower(), trimmed)


def provider_shorthand(name: str) -> str:
    """Return the short, human-readable form of a canonical provider name."""
    return SHORTHAND_BY_PROVIDER.get(name, name)


@dataclass(frozen=True)
class FaceSettings:
    """Configuration read from the environment at service start."""

    #: Requested providers as canonical ONNX Runtime names, in priority order.
    providers: tuple[str, ...] = (CPU_PROVIDER,)
    #: CUDA devices the engine keeps a model session on, in preference order.
    #: ``FACES_GPU_IDS=0,1`` spreads one session over both cards.
    gpu_ids: tuple[int, ...] = (0,)
    #: Maximum number of images one ``/faces/analyze-batch`` request may carry.
    max_batch: int = 16
    #: Number of aligned face crops sent through the recognizer per forward pass.
    recognition_batch: int = 32
    #: Concurrent analyses allowed per device (each device holds its own session).
    max_concurrency: int = 1
    #: Labelling algorithm revision: ``graph-v1`` or ``graph-v2``.
    cluster_algo: str = CLUSTER_ALGO

    @classmethod
    def from_env(cls, environ: Optional[Mapping[str, str]] = None) -> "FaceSettings":
        """Build settings from *environ* (defaults to ``os.environ``)."""
        source: Mapping[str, str] = os.environ if environ is None else environ
        providers = tuple(
            expand_provider(name)
            for name in source.get("FACES_PROVIDERS", "cpu").split(",")
            if name.strip()
        )
        gpu_ids = _env_int_list(source, "FACES_GPU_IDS")
        if not gpu_ids:
            gpu_ids = (_env_int(source, "FACES_GPU_ID", 0),)
        return cls(
            providers=providers or (CPU_PROVIDER,),
            gpu_ids=gpu_ids,
            max_batch=_env_int(source, "FACES_MAX_BATCH", 16, minimum=1),
            recognition_batch=_env_int(source, "FACES_RECOGNITION_BATCH", 32, minimum=1),
            max_concurrency=_env_int(source, "FACES_MAX_CONCURRENCY", 1, minimum=1),
            cluster_algo=_env_choice(
                source, "FACES_CLUSTER_ALGO", CLUSTER_ALGO, CLUSTER_ALGOS
            ),
        )


def _env_int_list(environ: Mapping[str, str], name: str) -> tuple[int, ...]:
    """Parse a comma-separated int list, ignoring blank or invalid entries."""
    raw = environ.get(name, "").strip()
    if not raw:
        return ()
    values: list[int] = []
    for part in raw.split(","):
        part = part.strip()
        if not part:
            continue
        try:
            values.append(int(part))
        except ValueError:
            logger.warning("invalid %s entry %r; ignoring", name, part)
    return tuple(values)


def _env_choice(
    environ: Mapping[str, str], name: str, default: str, choices: Sequence[str]
) -> str:
    """Return the configured choice, falling back to *default* when invalid."""
    raw = environ.get(name, "").strip()
    if not raw:
        return default
    if raw not in choices:
        logger.warning("invalid %s=%r; using %s", name, raw, default)
        return default
    return raw


def _env_int(
    environ: Mapping[str, str], name: str, default: int, minimum: Optional[int] = None
) -> int:
    raw = environ.get(name, "").strip()
    if not raw:
        return default
    try:
        value = int(raw)
    except ValueError:
        logger.warning("invalid %s=%r; using %d", name, raw, default)
        return default
    if minimum is not None and value < minimum:
        logger.warning("%s=%d below minimum %d; using %d", name, value, minimum, minimum)
        return minimum
    return value


@dataclass(frozen=True)
class ProviderSelection:
    """The outcome of intersecting the request with the runtime's offers."""

    #: Canonical names requested by configuration, in priority order.
    requested: tuple[str, ...]
    #: Providers the installed runtime reports as available.
    available: tuple[str, ...]
    #: Providers actually handed to InsightFace/onnxruntime, in priority order.
    active: tuple[str, ...]
    #: ``ctx_id`` for InsightFace: the GPU device id, or ``-1`` for CPU.
    device_id: int

    @property
    def active_provider(self) -> str:
        """Short name of the provider that will run inference."""
        return provider_shorthand(self.active[0])

    @property
    def requested_providers(self) -> list[str]:
        """Short names of the configured providers, in priority order."""
        return [provider_shorthand(name) for name in self.requested]


def resolve_providers(
    requested: Sequence[str],
    available: Sequence[str],
    gpu_id: int = 0,
) -> ProviderSelection:
    """Intersect *requested* providers with *available*, preserving order.

    An empty intersection falls back to the CPU provider with a warning, so a
    GPU request on a CPU-only host degrades instead of failing. ``ctx_id`` is
    derived from the resolved list rather than configured separately.
    """
    expanded = tuple(expand_provider(name) for name in requested if name.strip())
    requested_names = expanded or (CPU_PROVIDER,)
    available_names = tuple(available)

    resolved: list[str] = []
    for provider in requested_names:
        if provider in available_names and provider not in resolved:
            resolved.append(provider)

    if not resolved:
        logger.warning(
            "requested execution providers %s are unavailable (runtime offers %s); "
            "falling back to %s",
            list(requested_names),
            list(available_names),
            CPU_PROVIDER,
        )
        resolved = [CPU_PROVIDER]
    elif resolved[0] != requested_names[0]:
        logger.warning(
            "requested execution provider %s is unavailable (runtime offers %s); using %s",
            requested_names[0],
            list(available_names),
            resolved[0],
        )

    device_id = gpu_id if resolved[0] in CUDA_PROVIDERS else -1
    return ProviderSelection(
        requested=requested_names,
        available=available_names,
        active=tuple(resolved),
        device_id=device_id,
    )


def available_providers() -> tuple[str, ...]:
    """Return the providers the installed ONNX Runtime reports.

    Discovery is best-effort: a runtime that fails to import or to answer must
    not take the service down, so the CPU provider is assumed.
    """
    try:
        import onnxruntime

        return tuple(onnxruntime.get_available_providers())
    except Exception as exc:  # noqa: BLE001 - discovery must never fail the service
        logger.warning(
            "onnxruntime provider discovery failed (%s); assuming %s", exc, CPU_PROVIDER
        )
        return (CPU_PROVIDER,)
