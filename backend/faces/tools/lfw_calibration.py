"""Offline 1:1 face-verification calibration on the LFW benchmark.

The faces service ships similarity thresholds that the core recognition job
consumes from ``GET /faces/health``. Nothing in the repository used to measure
whether those numbers still describe reality, so this harness scores the
canonical Labelled Faces in the Wild (LFW) verification protocol against the
service's own engine:

* the official ``pairs.txt`` (6,000 pairs in 10 folds of 300 same-person and
  300 different-person pairs) is scored fold by fold, and the reported number
  is the unweighted mean of the ten fold accuracies;
* each distinct image is embedded once through :class:`app.engine.FaceEngine`
  (detection + alignment + ArcFace, including its small-face handling), so the
  measurement covers the production pipeline rather than pre-aligned crops;
* the cosine decision threshold is swept from 0.05 to 0.95 in 0.005 steps, and
  the accuracy-maximising value wins with ties broken toward the stricter
  (higher) threshold;
* a run is accepted only when the mean accuracy at the currently shipped
  threshold is at least :data:`ACCEPTANCE_FLOOR`; otherwise the command exits
  non-zero, reports the measured accuracy and the best candidate threshold and
  leaves ``app/config.py`` untouched.

LFW is fetched on demand from Kaggle into a temporary work directory and removed
again after a successful run, so no dataset is committed, needed on the host or
left behind. Only the Kaggle dataset ``atulanandjha/lfwpeople`` is used: it
carries both the official ``pairs.txt`` and ``lfw-funneled.tgz``, the funneled
archive that keeps the canonical ``lfw_funneled/<Name>/<Name>_NNNN.jpg`` layout
(13,233 images of 5,749 people), which the download normalises to ``lfw/``.

Kaggle serves that archive wrapped in a ZIP whose only member is the requested
file, so the transport is unwrapped by signature before extraction. Requests are
authenticated with a Kaggle access token taken from ``--kaggle-token-file``,
then ``KAGGLE_API_TOKEN``/``KAGGLE_ACCESS_TOKEN``, then the repository's
``.kaggle/access_token``. The token is resolved lazily, so a run against an
already prepared work directory needs no credentials at all.

The pytest suite never downloads anything: the downloader opener and the engine
factory are injected, and the tests serve a synthetic archive.

Usage
-----

.. code-block:: text

    python -m tools.lfw_calibration download   [--work-dir PATH] [--refresh]
                                               [--kaggle-token-file PATH]
    python -m tools.lfw_calibration verify     [--work-dir PATH]
    python -m tools.lfw_calibration calibrate  [--work-dir PATH] [--keep]
                                               [--apply-config PATH]
                                               [--shipped-threshold TAU]
                                               [--model-id ID]
                                               [--kaggle-token-file PATH]
    python -m tools.lfw_calibration clean      [--work-dir PATH]

``--work-dir`` falls back to ``LFW_WORK_DIR`` and then to a fresh
``tempfile.mkdtemp(prefix="flashbacks-lfw-")`` directory outside the repository.
"""

from __future__ import annotations

import argparse
import base64
import hashlib
import json
import re
import shutil
import sys
import tarfile
import tempfile
import urllib.request
import zipfile
from dataclasses import dataclass
from pathlib import Path, PurePosixPath
from typing import Any, Callable, ContextManager, Iterable, Mapping, Optional, Sequence

import numpy as np

from app.config import RECOGNITION_MIN_SIMILARITY

#: Kaggle dataset that mirrors the official LFW corpus (images and pairs.txt).
KAGGLE_DATASET = "atulanandjha/lfwpeople"
#: Kaggle per-file download endpoint: ``.../<owner>/<dataset>/<file>``.
KAGGLE_DOWNLOAD_URL = "https://www.kaggle.com/api/v1/datasets/download"
#: Archive inside the dataset; extracts to ``lfw_funneled/<Name>/<Name>_NNNN.jpg``.
ARCHIVE_NAME = "lfw-funneled.tgz"
PAIRS_NAME = "pairs.txt"
#: Top-level directory the Kaggle archive extracts to.
KAGGLE_IMAGES_DIR = "lfw_funneled"
#: Canonical directory the extracted tree is normalised to, relative to the work dir.
IMAGES_DIR = "lfw"
#: Embedding cache file inside the work directory.
CACHE_NAME = "embeddings.json"

#: Repository root (``backend/faces/tools`` is three levels below it).
REPO_ROOT = Path(__file__).resolve().parents[3]
#: Token file used when neither a flag nor an environment variable provides one.
DEFAULT_TOKEN_FILE = REPO_ROOT / ".kaggle" / "access_token"
#: Environment variables holding the Kaggle access token directly.
TOKEN_ENV_VARS = ("KAGGLE_API_TOKEN", "KAGGLE_ACCESS_TOKEN")
#: Sent with every Kaggle request so the traffic is attributable.
USER_AGENT = "flashbacks-lfw-calibration"

#: The official protocol: 10 folds of 600 pairs (300 same, 300 different).
FOLD_COUNT = 10
PAIRS_PER_FOLD = 600
SAME_PER_FOLD = 300
DIFFERENT_PER_FOLD = 300
TOTAL_PAIRS = FOLD_COUNT * PAIRS_PER_FOLD
#: Images in the canonical archive, reported so mirror drift is visible.
EXPECTED_IMAGES = 13233

#: Threshold sweep of D3: 0.05 .. 0.95 in 0.005 steps.
SWEEP_START = 0.05
SWEEP_STOP = 0.95
SWEEP_STEP = 0.005
#: Mean accuracy a candidate must reach to be adopted.
ACCEPTANCE_FLOOR = 0.90

#: Model pack the service loads; part of the embedding cache identity.
DEFAULT_MODEL_ID = "buffalo_l"
#: Bump when the embedding pipeline changes, so stale vectors are not reused.
PREPROCESSING_VERSION = "analyze-largest-face-v1"
#: Similarity recorded for a pair whose image yielded no face. It is below any
#: swept threshold, so such a pair counts as a non-match for both pair kinds.
UNDETECTED_SIMILARITY = -1.0

_NAME_RE = re.compile(r"^[A-Za-z0-9_'\-]+$")
# Horizontal whitespace only: ``\s`` would swallow the newline the constant ends
# on and rewrite the module without it.
_THRESHOLD_RE = re.compile(
    r"^RECOGNITION_MIN_SIMILARITY[ \t]*=[ \t]*[0-9]+(?:\.[0-9]+)?[ \t]*$",
    re.MULTILINE,
)


class CalibrationError(RuntimeError):
    """A calibration run cannot continue; the message explains why."""


class BelowFloor(CalibrationError):
    """The shipped threshold misses the acceptance floor."""

    def __init__(
        self,
        accuracy: float,
        threshold: float,
        best_threshold: float,
        best_accuracy: float,
    ) -> None:
        super().__init__(
            f"mean accuracy {accuracy:.4f} at the shipped threshold {threshold:g} "
            f"is below the {ACCEPTANCE_FLOOR:.2f} floor; the best candidate "
            f"threshold {best_threshold:g} reaches {best_accuracy:.4f}; "
            "configuration left unchanged"
        )
        self.accuracy = accuracy
        self.threshold = threshold
        self.best_threshold = best_threshold
        self.best_accuracy = best_accuracy


# ---------------------------------------------------------------------------
# Pair protocol
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class Pair:
    """One LFW verification pair, with both images as archive-relative paths."""

    fold: int
    same: bool
    left: str
    right: str


@dataclass(frozen=True)
class Dataset:
    """A validated, extracted LFW copy inside a work directory."""

    work_dir: Path
    pairs: tuple[Pair, ...]
    images: int


def image_path(name: str, number: int) -> str:
    """Return the archive-relative path of one LFW image.

    ``Adel_Abdel_Mahmoud`` + ``1`` becomes
    ``lfw/Adel_Abdel_Mahmoud/Adel_Abdel_Mahmoud_0001.jpg``.
    """
    return f"{IMAGES_DIR}/{name}/{name}_{number:04d}.jpg"


def _image_number(value: str, line: str) -> int:
    if not value.isdigit():
        raise CalibrationError(f"image number {value!r} is not a number: {line!r}")
    number = int(value)
    if number < 1:
        raise CalibrationError(f"image number {value!r} is below 1: {line!r}")
    return number


def _image_name(value: str, line: str) -> str:
    if not _NAME_RE.match(value):
        raise CalibrationError(f"invalid LFW identity name {value!r}: {line!r}")
    return value


def parse_pairs(text: str) -> tuple[Pair, ...]:
    """Parse ``pairs.txt`` into the official ten-fold pair protocol.

    The header declares ``<folds> <same-per-fold>``; each following line is
    either ``name num num`` (same person) or ``name num name num`` (different
    people). Folds are derived from position, six hundred pairs at a time, and
    the resulting structure is validated.
    """
    lines = [line.strip() for line in text.splitlines() if line.strip()]
    if not lines:
        raise CalibrationError("pairs file is empty")

    header = lines[0].split()
    if len(header) != 2 or not all(field.isdigit() for field in header):
        raise CalibrationError(f"unexpected pairs header: {lines[0]!r}")
    declared_folds, declared_same = int(header[0]), int(header[1])
    if (declared_folds, declared_same) != (FOLD_COUNT, SAME_PER_FOLD):
        raise CalibrationError(
            f"pairs header declares {declared_folds} folds of {declared_same} "
            f"same-person pairs; expected {FOLD_COUNT} folds of {SAME_PER_FOLD}"
        )

    pairs: list[Pair] = []
    for line in lines[1:]:
        fields = line.split()
        fold = len(pairs) // PAIRS_PER_FOLD + 1
        if len(fields) == 3:
            name = _image_name(fields[0], line)
            pairs.append(
                Pair(
                    fold=fold,
                    same=True,
                    left=image_path(name, _image_number(fields[1], line)),
                    right=image_path(name, _image_number(fields[2], line)),
                )
            )
        elif len(fields) == 4:
            left = _image_name(fields[0], line)
            right = _image_name(fields[2], line)
            pairs.append(
                Pair(
                    fold=fold,
                    same=False,
                    left=image_path(left, _image_number(fields[1], line)),
                    right=image_path(right, _image_number(fields[3], line)),
                )
            )
        else:
            raise CalibrationError(f"unexpected pairs line: {line!r}")

    validate_pairs(pairs)
    return tuple(pairs)


def validate_pairs(pairs: Sequence[Pair]) -> None:
    """Assert the ten-fold structure: 600 pairs per fold, 300 + 300."""
    if len(pairs) != TOTAL_PAIRS:
        raise CalibrationError(
            f"pairs file holds {len(pairs)} pairs; expected {TOTAL_PAIRS}"
        )
    folds = {pair.fold for pair in pairs}
    if folds != set(range(1, FOLD_COUNT + 1)):
        raise CalibrationError(
            f"pairs file spans folds {sorted(folds)}; expected 1..{FOLD_COUNT}"
        )
    for fold in range(1, FOLD_COUNT + 1):
        same = sum(1 for pair in pairs if pair.fold == fold and pair.same)
        different = sum(1 for pair in pairs if pair.fold == fold and not pair.same)
        if different != DIFFERENT_PER_FOLD or same != SAME_PER_FOLD:
            raise CalibrationError(
                f"fold {fold} holds {same} same-person and {different} "
                f"different-person pairs; expected {SAME_PER_FOLD} and "
                f"{DIFFERENT_PER_FOLD}"
            )


# ---------------------------------------------------------------------------
# Dataset lifecycle
# ---------------------------------------------------------------------------


def resolve_work_dir(
    explicit: Optional[str], environ: Optional[Mapping[str, str]], *, create: bool
) -> Path:
    """Resolve ``--work-dir`` → ``LFW_WORK_DIR`` → a fresh temporary directory."""
    if explicit:
        return Path(explicit).expanduser()
    from_env = (environ or {}).get("LFW_WORK_DIR")
    if from_env:
        return Path(from_env).expanduser()
    if not create:
        raise CalibrationError(
            "no work directory given; pass --work-dir or set LFW_WORK_DIR"
        )
    return Path(tempfile.mkdtemp(prefix="flashbacks-lfw-"))


def resolve_token(
    explicit_file: Optional[str], environ: Optional[Mapping[str, str]]
) -> str:
    """Resolve the Kaggle access token: environment → flag → repository file."""
    for name in TOKEN_ENV_VARS:
        from_env = (environ or {}).get(name)
        if from_env and from_env.strip():
            return from_env.strip()

    path = Path(explicit_file).expanduser() if explicit_file else DEFAULT_TOKEN_FILE
    try:
        token = path.read_text(encoding="utf-8").strip()
    except OSError as exc:
        raise CalibrationError(
            f"no Kaggle access token: cannot read {path} ({exc}); create the token "
            "on kaggle.com/settings/api or set KAGGLE_API_TOKEN"
        ) from exc
    if not token:
        raise CalibrationError(f"Kaggle access token file {path} is empty")
    return token


def kaggle_opener(
    token: str, timeout: float = 60.0
) -> Callable[[str], ContextManager[Any]]:
    """Opener that authenticates every Kaggle request with the bearer *token*."""
    headers = {"Authorization": f"Bearer {token}", "User-Agent": USER_AGENT}

    def open_url(url: str) -> ContextManager[Any]:
        request = urllib.request.Request(url, headers=headers)
        return urllib.request.urlopen(request, timeout=timeout)  # noqa: S310 - known host

    return open_url


class LazyKaggleOpener:
    """Build the authenticated opener on first use, so offline runs need no token.

    Commands that score a prepared work directory never touch the network and
    therefore never demand credentials; only an actual download resolves the
    token, and a missing one surfaces as a :class:`CalibrationError`.
    """

    def __init__(
        self,
        token_file: Optional[str] = None,
        environ: Optional[Mapping[str, str]] = None,
    ) -> None:
        self._token_file = token_file
        self._environ = environ
        self._opener: Optional[Callable[[str], ContextManager[Any]]] = None

    def __call__(self, url: str) -> ContextManager[Any]:
        if self._opener is None:
            self._opener = kaggle_opener(resolve_token(self._token_file, self._environ))
        return self._opener(url)


def dataset_url(file_name: str) -> str:
    """Kaggle API URL of one file inside :data:`KAGGLE_DATASET`."""
    return f"{KAGGLE_DOWNLOAD_URL}/{KAGGLE_DATASET}/{file_name}"


def download_file(
    url: str, destination: Path, opener: Callable[[str], ContextManager[Any]]
) -> int:
    """Stream *url* into *destination* atomically; return the byte count."""
    destination.parent.mkdir(parents=True, exist_ok=True)
    partial = destination.with_name(destination.name + ".part")
    written = 0
    try:
        with opener(url) as response, partial.open("wb") as handle:
            while True:
                chunk = response.read(1 << 20)
                if not chunk:
                    break
                handle.write(chunk)
                written += len(chunk)
    except OSError as exc:
        partial.unlink(missing_ok=True)
        raise CalibrationError(f"cannot download {url}: {exc}") from exc
    if written == 0:
        partial.unlink(missing_ok=True)
        raise CalibrationError(f"download of {url} returned no data")
    partial.replace(destination)
    return written


def _looks_like_zip(path: Path) -> bool:
    """True when *path* starts with the ZIP local-file signature."""
    with path.open("rb") as handle:
        return handle.read(4) == b"PK\x03\x04"


def unwrap_kaggle_zip(path: Path, expected_name: str) -> None:
    """Replace a ZIP download with the single dataset file it carries.

    The Kaggle per-file endpoint serves binary payloads wrapped in a ZIP whose
    only member is the requested file, while text payloads arrive raw; the
    signature decides, so both transports work unchanged.
    """
    if not _looks_like_zip(path):
        return

    extracted = path.with_name(path.name + ".unwrapped")
    try:
        with zipfile.ZipFile(path) as archive:
            members = [
                info
                for info in archive.infolist()
                if PurePosixPath(info.filename).name == expected_name
            ]
            if len(members) != 1:
                raise CalibrationError(
                    f"{path.name} carries {len(members)} copies of {expected_name}; "
                    "expected exactly 1"
                )
            with archive.open(members[0]) as source, extracted.open("wb") as sink:
                shutil.copyfileobj(source, sink, 1 << 20)
    except (zipfile.BadZipFile, OSError) as exc:
        extracted.unlink(missing_ok=True)
        raise CalibrationError(f"cannot unwrap {path.name}: {exc}") from exc
    extracted.replace(path)


def download_dataset_file(
    file_name: str,
    work_dir: Path,
    opener: Callable[[str], ContextManager[Any]],
) -> Path:
    """Fetch one file of :data:`KAGGLE_DATASET` into *work_dir*."""
    destination = work_dir / file_name
    if destination.is_file():
        return destination
    download_file(dataset_url(file_name), destination, opener)
    try:
        unwrap_kaggle_zip(destination, file_name)
    except CalibrationError:
        # A half-processed download would fail extraction on the next attempt.
        destination.unlink(missing_ok=True)
        raise
    return destination


def extract_archive(archive: Path, destination: Path) -> None:
    """Extract the LFW archive into *destination*."""
    filter_kwargs = (
        {"filter": "data"} if getattr(tarfile, "data_filter", None) is not None else {}
    )
    try:
        with tarfile.open(archive, "r:gz") as tar:
            tar.extractall(destination, **filter_kwargs)  # noqa: S202 - trusted host
    except (tarfile.TarError, OSError) as exc:
        raise CalibrationError(f"cannot extract {archive.name}: {exc}") from exc


def normalise_images_tree(work_dir: Path) -> None:
    """Move the extracted layout (``lfw_funneled/``) to the canonical ``lfw/``.

    The Kaggle archive keeps identity folders under ``lfw_funneled``; the pair
    protocol and the image cache address images as ``lfw/<Name>/<Name>_NNNN.jpg``,
    so the tree is renamed once instead of threading a second layout through the
    scoring code.
    """
    root = work_dir / IMAGES_DIR
    if root.is_dir():
        return
    for candidate in (KAGGLE_IMAGES_DIR, KAGGLE_IMAGES_DIR.replace("_", "-")):
        source = work_dir / candidate
        if source.is_dir():
            source.replace(root)
            return
    raise CalibrationError(
        f"{ARCHIVE_NAME} contains neither a {IMAGES_DIR}/ nor a "
        f"{KAGGLE_IMAGES_DIR}/ tree"
    )


def verify_dataset(work_dir: Path, pairs: Sequence[Pair]) -> Dataset:
    """Check that every paired image exists and count the extracted images."""
    missing = [
        path
        for pair in pairs
        for path in (pair.left, pair.right)
        if not (work_dir / path).is_file()
    ]
    if missing:
        raise CalibrationError(
            f"{len(missing)} images referenced by the pairs file are missing "
            f"(first: {missing[0]})"
        )
    root = work_dir / IMAGES_DIR
    if not root.is_dir():
        raise CalibrationError(f"{root} does not exist")
    images = sum(1 for _ in root.rglob("*.jpg"))
    return Dataset(work_dir=work_dir, pairs=tuple(pairs), images=images)


def prepare_dataset(
    work_dir: Path,
    opener: Callable[[str], ContextManager[Any]],
    *,
    refresh: bool = False,
) -> Dataset:
    """Download (when absent), extract, normalise and validate LFW in *work_dir*."""
    work_dir.mkdir(parents=True, exist_ok=True)
    root = work_dir / IMAGES_DIR
    archive = work_dir / ARCHIVE_NAME
    pairs_file = work_dir / PAIRS_NAME

    if refresh:
        shutil.rmtree(root, ignore_errors=True)
        shutil.rmtree(work_dir / KAGGLE_IMAGES_DIR, ignore_errors=True)
        archive.unlink(missing_ok=True)
        pairs_file.unlink(missing_ok=True)

    if not pairs_file.is_file():
        download_dataset_file(PAIRS_NAME, work_dir, opener)

    pairs = parse_pairs(pairs_file.read_text(encoding="utf-8", errors="replace"))

    if not root.is_dir():
        if not archive.is_file():
            download_dataset_file(ARCHIVE_NAME, work_dir, opener)
        extract_archive(archive, work_dir)
        normalise_images_tree(work_dir)

    return verify_dataset(work_dir, pairs)


# ---------------------------------------------------------------------------
# Embeddings
# ---------------------------------------------------------------------------


class EmbeddingCache:
    """Content-addressed embedding cache: image sha256 + model + preprocessing."""

    def __init__(self, path: Path, model_id: str, preprocessing: str) -> None:
        self.path = path
        self.model_id = model_id
        self.preprocessing = preprocessing
        self._vectors: dict[str, Optional[list[float]]] = {}
        self._load()

    def _load(self) -> None:
        if not self.path.is_file():
            return
        try:
            payload = json.loads(self.path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            return
        stored = payload.get("vectors") if isinstance(payload, dict) else None
        if not isinstance(stored, dict):
            return
        self._vectors = {
            key: value
            for key, value in stored.items()
            if isinstance(key, str) and (value is None or isinstance(value, list))
        }

    def key(self, digest: str) -> str:
        """Cache key of one image digest under this model and preprocessing."""
        return f"{digest}|{self.model_id}|{self.preprocessing}"

    def has(self, digest: str) -> bool:
        return self.key(digest) in self._vectors

    def get(self, digest: str) -> Optional[list[float]]:
        return self._vectors[self.key(digest)]

    def put(self, digest: str, embedding: Optional[Sequence[float]]) -> None:
        self._vectors[self.key(digest)] = (
            None if embedding is None else [float(value) for value in embedding]
        )

    def save(self) -> None:
        """Persist the cache; a crash mid-run keeps the finished images."""
        self.path.parent.mkdir(parents=True, exist_ok=True)
        payload = {
            "modelId": self.model_id,
            "preprocessing": self.preprocessing,
            "vectors": self._vectors,
        }
        self.path.write_text(json.dumps(payload), encoding="utf-8")


class LazyEngine:
    """Construct the engine only when an image misses the cache.

    A warm cache therefore measures nothing but the arithmetic, and a second
    A/B candidate never reloads the model.
    """

    def __init__(
        self,
        factory: Callable[[], Any],
        model_id: str = DEFAULT_MODEL_ID,
    ) -> None:
        self._factory = factory
        self._model_id = model_id
        self._engine: Optional[Any] = None

    @property
    def loaded(self) -> bool:
        return self._engine is not None

    @property
    def model_id(self) -> str:
        """Model pack id the cache is keyed for."""
        return self._model_id

    def get(self) -> Any:
        if self._engine is not None:
            return self._engine
        engine = self._factory()
        name = getattr(engine, "name", None)
        if isinstance(name, str) and name != self._model_id:
            raise CalibrationError(
                f"the engine loads model {name!r} but the cache is keyed for "
                f"{self._model_id!r}; re-run with --model-id {name}"
            )
        self._engine = engine
        return engine


def default_engine_factory() -> Any:
    """Build the production engine; the heavy imports stay lazy."""
    from app.engine import FaceEngine

    return FaceEngine()


def largest_face_embedding(faces: Sequence[Mapping[str, Any]]) -> Optional[np.ndarray]:
    """Return the embedding of the largest detected face (LFW shows one person)."""
    best: Optional[Mapping[str, Any]] = None
    best_area = -1.0
    for face in faces:
        area = float(face.get("width", 0.0)) * float(face.get("height", 0.0))
        if area > best_area:
            best, best_area = face, area
    if best is None:
        return None
    embedding = best.get("embedding") or []
    if not embedding:
        return None
    return np.asarray(embedding, dtype=float)


def distinct_pair_images(pairs: Iterable[Pair]) -> list[str]:
    """Every image path the protocol needs, once, in first-appearance order."""
    seen: dict[str, None] = {}
    for pair in pairs:
        seen.setdefault(pair.left, None)
        seen.setdefault(pair.right, None)
    return list(seen)


def embed_images(
    dataset: Dataset,
    cache: EmbeddingCache,
    engine: LazyEngine,
    *,
    log: Callable[[str], None] = lambda _message: None,
) -> dict[str, Optional[np.ndarray]]:
    """Embed every image the protocol mentions once, reusing cached vectors."""
    vectors: dict[str, Optional[np.ndarray]] = {}
    embeddingd = 0
    try:
        for relative in distinct_pair_images(dataset.pairs):
            raw = (dataset.work_dir / relative).read_bytes()
            digest = hashlib.sha256(raw).hexdigest()
            if cache.has(digest):
                cached = cache.get(digest)
                vectors[relative] = (
                    None if cached is None else np.asarray(cached, dtype=float)
                )
                continue
            faces = engine.get().analyze(base64.b64encode(raw).decode("ascii"))
            embedding = largest_face_embedding(faces)
            cache.put(digest, None if embedding is None else embedding.tolist())
            vectors[relative] = embedding
            embeddingd += 1
            if embeddingd % 500 == 0:
                log(f"embedded {embeddingd} images")
    finally:
        cache.save()
    undetected = sum(1 for vector in vectors.values() if vector is None)
    log(
        f"images: {len(vectors)} distinct, {embeddingd} freshly embedded, "
        f"{undetected} without a detected face"
    )
    return vectors


# ---------------------------------------------------------------------------
# Scoring
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class FoldScores:
    """Per-fold similarities, split by the expected decision."""

    fold: int
    same: np.ndarray
    different: np.ndarray

    @property
    def pairs(self) -> int:
        return int(self.same.size + self.different.size)


@dataclass(frozen=True)
class Evaluation:
    """Accuracies of one candidate threshold."""

    threshold: float
    per_fold: tuple[float, ...]
    mean: float
    pairs: int
    folds: int


@dataclass(frozen=True)
class CandidateScore:
    """One row of the A/B table."""

    label: str
    threshold: float
    per_fold: tuple[float, ...]
    mean: float
    pairs: int
    folds: int


def cosine_similarity(left: np.ndarray, right: np.ndarray) -> float:
    """Cosine similarity of two embeddings; a zero vector matches nobody."""
    denominator = float(np.linalg.norm(left) * np.linalg.norm(right))
    if denominator <= 0.0:
        return UNDETECTED_SIMILARITY
    return float(np.dot(left, right) / denominator)


def pair_similarity(
    pair: Pair, embeddings: Mapping[str, Optional[np.ndarray]]
) -> float:
    """Similarity of one pair, or :data:`UNDETECTED_SIMILARITY` when a face is missing.

    An image that yields no detectable face cannot be matched in production
    either, so the pair is scored as a non-match rather than dropped.
    """
    left = embeddings.get(pair.left)
    right = embeddings.get(pair.right)
    if left is None or right is None:
        return UNDETECTED_SIMILARITY
    return cosine_similarity(left, right)


def fold_scores(
    pairs: Sequence[Pair], embeddings: Mapping[str, Optional[np.ndarray]]
) -> list[FoldScores]:
    """Group pair similarities by fold, keeping the expected decision apart."""
    same: dict[int, list[float]] = {fold: [] for fold in range(1, FOLD_COUNT + 1)}
    different: dict[int, list[float]] = {
        fold: [] for fold in range(1, FOLD_COUNT + 1)
    }
    for pair in pairs:
        similarity = pair_similarity(pair, embeddings)
        (same if pair.same else different)[pair.fold].append(similarity)
    return [
        FoldScores(
            fold=fold,
            same=np.asarray(same[fold], dtype=float),
            different=np.asarray(different[fold], dtype=float),
        )
        for fold in sorted(same)
    ]


def evaluate(scores: Sequence[FoldScores], threshold: float) -> Evaluation:
    """Score every fold at *threshold* and average the folds unweighted."""
    per_fold: list[float] = []
    pairs = 0
    for folded in scores:
        correct = int(np.count_nonzero(folded.same >= threshold)) + int(
            np.count_nonzero(folded.different < threshold)
        )
        total = folded.pairs
        pairs += total
        per_fold.append(correct / total if total else 0.0)
    mean = sum(per_fold) / len(per_fold) if per_fold else 0.0
    return Evaluation(
        threshold=threshold,
        per_fold=tuple(per_fold),
        mean=mean,
        pairs=pairs,
        folds=len(per_fold),
    )


def threshold_grid(
    start: float = SWEEP_START,
    stop: float = SWEEP_STOP,
    step: float = SWEEP_STEP,
) -> list[float]:
    """Ascending candidate thresholds, inclusive of *stop*, free of float noise."""
    count = int(round((stop - start) / step)) + 1
    return [round(start + index * step, 3) for index in range(count)]


def sweep(scores: Sequence[FoldScores]) -> list[tuple[float, float]]:
    """Mean accuracy for every swept threshold, ascending by threshold."""
    return [(tau, evaluate(scores, tau).mean) for tau in threshold_grid()]


def select_threshold(rows: Sequence[tuple[float, float]]) -> tuple[float, float]:
    """Return the accuracy-maximising threshold, ties broken toward the stricter one."""
    if not rows:
        raise CalibrationError("the threshold sweep produced no rows")
    best_threshold, best_accuracy = rows[0]
    for threshold, accuracy in rows[1:]:
        if accuracy > best_accuracy or (
            accuracy == best_accuracy and threshold > best_threshold
        ):
            best_threshold, best_accuracy = threshold, accuracy
    return best_threshold, best_accuracy


def candidate_rows(
    shipped: Evaluation, selected: Evaluation
) -> list[CandidateScore]:
    """A/B rows: the shipped threshold and the swept candidate, same pairs."""
    return [
        CandidateScore(
            label="shipped",
            threshold=shipped.threshold,
            per_fold=shipped.per_fold,
            mean=shipped.mean,
            pairs=shipped.pairs,
            folds=shipped.folds,
        ),
        CandidateScore(
            label="sweep",
            threshold=selected.threshold,
            per_fold=selected.per_fold,
            mean=selected.mean,
            pairs=selected.pairs,
            folds=selected.folds,
        ),
    ]


def apply_threshold(config_path: Path, threshold: float) -> None:
    """Rewrite ``RECOGNITION_MIN_SIMILARITY`` in *config_path*."""
    try:
        text = config_path.read_text(encoding="utf-8")
    except OSError as exc:
        raise CalibrationError(f"cannot read {config_path}: {exc}") from exc
    replacement = f"RECOGNITION_MIN_SIMILARITY = {threshold:g}"
    updated, count = _THRESHOLD_RE.subn(replacement, text)
    if count != 1:
        raise CalibrationError(
            f"{config_path} must declare RECOGNITION_MIN_SIMILARITY exactly once "
            f"(found {count})"
        )
    try:
        config_path.write_text(updated, encoding="utf-8")
    except OSError as exc:
        raise CalibrationError(f"cannot write {config_path}: {exc}") from exc


# ---------------------------------------------------------------------------
# Reporting
# ---------------------------------------------------------------------------


def format_evaluation(evaluation: Evaluation) -> str:
    """Per-fold table plus the unweighted mean."""
    lines = ["fold  pairs  accuracy"]
    for index, accuracy in enumerate(evaluation.per_fold, start=1):
        lines.append(f"{index:>4}  {PAIRS_PER_FOLD:>5}  {accuracy:.4f}")
    lines.append(
        f"mean  {evaluation.pairs:>5}  {evaluation.mean:.4f} "
        f"(threshold {evaluation.threshold:g})"
    )
    return "\n".join(lines)


def format_candidates(candidates: Sequence[CandidateScore]) -> str:
    """A/B table: one row per candidate over the identical pair set."""
    lines = ["candidate  threshold  pairs  folds  accuracy"]
    for candidate in candidates:
        lines.append(
            f"{candidate.label:<9}  {candidate.threshold:>9.3f}  "
            f"{candidate.pairs:>5}  {candidate.folds:>5}  {candidate.mean:.4f}"
        )
    return "\n".join(lines)


def format_sweep(rows: Sequence[tuple[float, float]], selected_threshold: float) -> str:
    """Best swept thresholds, highest accuracy first."""
    ranked = sorted(rows, key=lambda row: (-row[1], -row[0]))[:5]
    lines = ["best thresholds (accuracy, strictest first on ties)"]
    for threshold, accuracy in ranked:
        marker = "*" if threshold == selected_threshold else " "
        lines.append(f"{marker} {threshold:.3f}  {accuracy:.4f}")
    return "\n".join(lines)


# ---------------------------------------------------------------------------
# Commands
# ---------------------------------------------------------------------------


def run_download(
    work_dir: Path,
    opener: Callable[[str], ContextManager[Any]],
    *,
    refresh: bool = False,
) -> int:
    """Fetch, extract and validate LFW; the dataset is kept until cleaned."""
    try:
        dataset = prepare_dataset(work_dir, opener, refresh=refresh)
    except CalibrationError as exc:
        print(f"download failed: {exc}", file=sys.stderr)
        return 1
    print(
        f"dataset ready: {dataset.images} images, {len(dataset.pairs)} pairs in "
        f"{FOLD_COUNT} folds\nsource: kaggle {KAGGLE_DATASET}\n"
        f"work directory: {work_dir}"
    )
    if dataset.images != EXPECTED_IMAGES:
        print(
            f"warning: {dataset.images} images instead of the canonical "
            f"{EXPECTED_IMAGES}; the mirror may have drifted",
            file=sys.stderr,
        )
    return 0


def run_verify(work_dir: Path) -> int:
    """Validate a prepared work directory without scoring anything."""
    pairs_file = work_dir / PAIRS_NAME
    if not pairs_file.is_file():
        print(f"verify failed: {pairs_file} is missing", file=sys.stderr)
        return 1
    try:
        pairs = parse_pairs(pairs_file.read_text(encoding="utf-8", errors="replace"))
        dataset = verify_dataset(work_dir, pairs)
    except CalibrationError as exc:
        print(f"verify failed: {exc}", file=sys.stderr)
        return 1
    same = sum(1 for pair in pairs if pair.same)
    print(
        f"pairs: {len(pairs)} ({same} same-person, {len(pairs) - same} "
        f"different-person) in {FOLD_COUNT} folds of {PAIRS_PER_FOLD}\n"
        f"images: {dataset.images}"
    )
    return 0


def run_clean(work_dir: Path) -> int:
    """Remove a retained work directory."""
    if not work_dir.exists():
        print(f"nothing to clean: {work_dir} does not exist")
        return 0
    shutil.rmtree(work_dir, ignore_errors=True)
    if work_dir.exists():
        print(f"clean failed: {work_dir} is still present", file=sys.stderr)
        return 1
    print(f"removed: {work_dir}")
    return 0


def run_calibrate(
    work_dir: Path,
    opener: Callable[[str], ContextManager[Any]],
    engine: LazyEngine,
    *,
    shipped_threshold: float,
    keep: bool,
    apply_config: Optional[Path],
    log: Callable[[str], None] = print,
) -> int:
    """Score the protocol, sweep the threshold and enforce the acceptance floor."""
    try:
        dataset = prepare_dataset(work_dir, opener)
        log(
            f"dataset: {dataset.images} images, {len(dataset.pairs)} pairs in "
            f"{FOLD_COUNT} folds"
        )
        cache = EmbeddingCache(
            work_dir / CACHE_NAME, engine.model_id, PREPROCESSING_VERSION
        )
        embeddings = embed_images(dataset, cache, engine, log=log)
        scores = fold_scores(dataset.pairs, embeddings)

        baseline = evaluate(scores, shipped_threshold)
        log(format_evaluation(baseline))

        rows = sweep(scores)
        selected_threshold, selected_accuracy = select_threshold(rows)
        selected = evaluate(scores, selected_threshold)
        log(format_candidates(candidate_rows(baseline, selected)))
        log(format_sweep(rows, selected_threshold))

        if round(baseline.mean, 6) < ACCEPTANCE_FLOOR:
            raise BelowFloor(
                accuracy=baseline.mean,
                threshold=shipped_threshold,
                best_threshold=selected_threshold,
                best_accuracy=selected_accuracy,
            )

        if apply_config is not None:
            apply_threshold(apply_config, selected_threshold)
            log(f"recognition threshold written to {apply_config}: {selected_threshold:g}")

        log(
            f"accepted: mean accuracy {baseline.mean:.4f} at the shipped "
            f"threshold {shipped_threshold:g}; selected {selected_threshold:g} "
            f"({selected.mean:.4f})"
        )
    except CalibrationError as exc:
        print(f"calibration failed: {exc}", file=sys.stderr)
        if work_dir.exists():
            print(f"work directory retained: {work_dir}", file=sys.stderr)
        return 1

    if keep:
        log(f"work directory kept: {work_dir}")
    else:
        shutil.rmtree(work_dir, ignore_errors=True)
        log(f"work directory removed: {work_dir}")
    return 0


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------


def _add_token_argument(parser: argparse.ArgumentParser) -> None:
    """Register the Kaggle credential flag on a downloading subcommand."""
    parser.add_argument(
        "--kaggle-token-file",
        metavar="PATH",
        help=(
            "Kaggle access token file (default: $KAGGLE_API_TOKEN, then "
            f"{DEFAULT_TOKEN_FILE})"
        ),
    )


def build_parser() -> argparse.ArgumentParser:
    """Build the ``download``/``verify``/``calibrate``/``clean`` CLI."""
    parser = argparse.ArgumentParser(
        prog="tools.lfw_calibration",
        description=(
            "Measure 1:1 face verification on LFW with the service's own engine "
            "and calibrate the recognition threshold."
        ),
    )
    subcommands = parser.add_subparsers(dest="command", required=True)

    download = subcommands.add_parser(
        "download", help=f"fetch, extract and validate LFW from kaggle {KAGGLE_DATASET}"
    )
    download.add_argument("--work-dir", help="dataset work directory (or LFW_WORK_DIR)")
    download.add_argument(
        "--refresh", action="store_true", help="re-download even when a copy exists"
    )
    _add_token_argument(download)

    verify = subcommands.add_parser(
        "verify", help="validate a prepared work directory without scoring"
    )
    verify.add_argument("--work-dir", help="dataset work directory (or LFW_WORK_DIR)")

    calibrate = subcommands.add_parser(
        "calibrate",
        help="score the LFW protocol, sweep the threshold and enforce the floor",
    )
    calibrate.add_argument("--work-dir", help="dataset work directory (or LFW_WORK_DIR)")
    calibrate.add_argument(
        "--keep", action="store_true", help="keep the work directory after success"
    )
    calibrate.add_argument(
        "--apply-config",
        metavar="PATH",
        help="write the selected RECOGNITION_MIN_SIMILARITY into this config file",
    )
    calibrate.add_argument(
        "--shipped-threshold",
        type=float,
        help="threshold to score as the shipped baseline (default: app config value)",
    )
    _add_token_argument(calibrate)
    calibrate.add_argument(
        "--model-id",
        default=DEFAULT_MODEL_ID,
        help=f"model pack id used in the cache key (default: {DEFAULT_MODEL_ID})",
    )
    calibrate.add_argument(
        "--quiet", action="store_true", help="only print failures"
    )

    clean = subcommands.add_parser("clean", help="remove a retained work directory")
    clean.add_argument("--work-dir", help="dataset work directory (or LFW_WORK_DIR)")

    return parser


def main(
    argv: Optional[Sequence[str]] = None,
    *,
    opener: Optional[Callable[[str], ContextManager[Any]]] = None,
    engine_factory: Optional[Callable[[], Any]] = None,
    environ: Optional[Mapping[str, str]] = None,
) -> int:
    """Run one harness command; return its exit status."""
    arguments = build_parser().parse_args(argv)
    downloader = opener or LazyKaggleOpener(
        getattr(arguments, "kaggle_token_file", None), environ
    )

    if arguments.command == "clean":
        try:
            work_dir = resolve_work_dir(arguments.work_dir, environ, create=False)
        except CalibrationError as exc:
            print(f"clean failed: {exc}", file=sys.stderr)
            return 1
        return run_clean(work_dir)

    work_dir = resolve_work_dir(arguments.work_dir, environ, create=True)
    if arguments.command == "download":
        return run_download(work_dir, downloader, refresh=arguments.refresh)
    if arguments.command == "verify":
        return run_verify(work_dir)
    if arguments.command == "calibrate":
        engine = LazyEngine(
            engine_factory or default_engine_factory, arguments.model_id
        )
        log = (lambda _message: None) if arguments.quiet else print
        return run_calibrate(
            work_dir,
            downloader,
            engine,
            shipped_threshold=(
                arguments.shipped_threshold
                if arguments.shipped_threshold is not None
                else float(RECOGNITION_MIN_SIMILARITY)
            ),
            keep=arguments.keep,
            apply_config=(
                Path(arguments.apply_config).expanduser()
                if arguments.apply_config
                else None
            ),
            log=log,
        )
    raise CalibrationError(f"unknown command {arguments.command!r}")


if __name__ == "__main__":
    sys.exit(main())
