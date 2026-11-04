"""Support code shared by the faces HTTP integration suite.

The suite talks to a live ``faces`` service (see ``backend/faces/README.md``)
and drives it with the photo dataset stored in ``tests/dataset/faces``.

Every dataset path used here is relative to ``tests/dataset`` so that the face
photos and the "no face" control image share a single root.
"""

from __future__ import annotations

import base64
import math
from pathlib import Path
from typing import Mapping, Optional, Sequence

import requests

#: ArcFace (w600k_r50) embedding length — must match ``app.engine.EMBEDDING_DIM``.
EMBEDDING_DIMENSION = 512

FACES_DIR = "faces"
POLITICIANS = f"{FACES_DIR}/politicians"
SCIENTISTS = f"{FACES_DIR}/scientists"

#: Control image from the shared dataset that must not contain any face.
NO_FACE_IMAGE = "image/light-bulb.jpg"

# Leaned-on defaults. They are deliberately conservative: the suite must catch
# real regressions (a person no longer recognised) without failing on JPEG
# re-encoding noise. Every value can be overridden from the command line or the
# environment — see ``tests/faces/README.md``.
DEFAULT_MIN_DETECTION_CONFIDENCE = 0.5
DEFAULT_SAME_PERSON_COSINE = 0.40
DEFAULT_IMPOSTOR_COSINE = 0.35
DEFAULT_GROUP_COSINE = 0.30
DEFAULT_REQUEST_TIMEOUT = 120.0
DEFAULT_CLUSTER_MIN_SIZE = 2

# ─── Dataset ground truth ────────────────────────────────────────────────────
# Files and identities are documented in tests/dataset/faces/README.md and the
# per-subfolder READMEs. Keep this table in sync with the dataset.

OBAMA_FRONT = f"{POLITICIANS}/obama_front.jpg"
OBAMA_PROFILE = f"{POLITICIANS}/obama_profile.jpg"
OBAMA_BIDEN = f"{POLITICIANS}/obama_biden.jpg"
OBAMA_CABINET = f"{POLITICIANS}/obama_cabinet.jpg"
MERKEL_PORTRAIT = f"{POLITICIANS}/merkel_portrait.jpg"
MACRON_PORTRAIT = f"{POLITICIANS}/macron_portrait.jpg"
G7_FAMILY_PHOTO = f"{POLITICIANS}/g7_family_photo.jpg"

EINSTEIN_FRONTAL = f"{SCIENTISTS}/einstein_frontal.jpg"
EINSTEIN_PROFILE = f"{SCIENTISTS}/einstein_profile.jpg"
EINSTEIN_1947 = f"{SCIENTISTS}/einstein_1947.jpg"
CURIE_PORTRAIT = f"{SCIENTISTS}/curie_portrait.jpg"
BOHR_PORTRAIT = f"{SCIENTISTS}/bohr_portrait.jpg"
SOLVAY_1927 = f"{SCIENTISTS}/solvay_1927.jpg"
SOLVAY_1911 = f"{SCIENTISTS}/solvay_1911.jpg"

#: Identity -> photos that show exactly that one person.
IDENTITY_PORTRAITS: Mapping[str, tuple[str, ...]] = {
    "obama": (OBAMA_FRONT, OBAMA_PROFILE),
    "merkel": (MERKEL_PORTRAIT,),
    "macron": (MACRON_PORTRAIT,),
    "einstein": (EINSTEIN_FRONTAL, EINSTEIN_PROFILE, EINSTEIN_1947),
    "curie": (CURIE_PORTRAIT,),
    "bohr": (BOHR_PORTRAIT,),
}

PORTRAIT_FILES: tuple[str, ...] = tuple(
    photo for photos in IDENTITY_PORTRAITS.values() for photo in photos
)

#: Group photos -> conservative lower bound of detectable faces.
GROUP_PHOTOS: Mapping[str, int] = {
    OBAMA_BIDEN: 2,
    OBAMA_CABINET: 3,
    G7_FAMILY_PHOTO: 5,
    SOLVAY_1927: 5,
    SOLVAY_1911: 5,
}

#: (group photo, identity documented as present in it, minimum cosine).
#: The bound is per pair because face size differs wildly inside a group photo:
#: in the downscaled ``g7_family_photo.jpg`` Merkel's face is only ~20x28 px, so
#: its ArcFace vector carries less identity signal than a portrait does
#: (measured ~0.22) while the closest other face of that photo stays near 0.10.
#: ``--group-cosine`` can relax a bound further but never tighten it.
GROUP_IDENTITY_PAIRS: tuple[tuple[str, str, float], ...] = (
    (OBAMA_BIDEN, "obama", DEFAULT_GROUP_COSINE),
    (OBAMA_CABINET, "obama", DEFAULT_GROUP_COSINE),
    (G7_FAMILY_PHOTO, "merkel", 0.20),
    (G7_FAMILY_PHOTO, "macron", DEFAULT_GROUP_COSINE),
    (SOLVAY_1927, "einstein", DEFAULT_GROUP_COSINE),
    (SOLVAY_1927, "curie", DEFAULT_GROUP_COSINE),
    (SOLVAY_1927, "bohr", DEFAULT_GROUP_COSINE),
)

#: Portrait pairs of the same person with a per-pair minimum cosine similarity.
#: The older/newer Einstein pair gets the loosest bound: 26 years of age and a
#: different pose separate the two photos.
SAME_PERSON_PAIRS: tuple[tuple[str, str, float], ...] = (
    (OBAMA_FRONT, OBAMA_PROFILE, DEFAULT_SAME_PERSON_COSINE),
    (EINSTEIN_FRONTAL, EINSTEIN_PROFILE, DEFAULT_SAME_PERSON_COSINE),
    (EINSTEIN_FRONTAL, EINSTEIN_1947, 0.35),
    (EINSTEIN_PROFILE, EINSTEIN_1947, 0.35),
)

#: Portrait pairs of different people that must stay below the impostor bound.
IMPOSTOR_PAIRS: tuple[tuple[str, str], ...] = (
    (OBAMA_FRONT, MERKEL_PORTRAIT),
    (OBAMA_FRONT, MACRON_PORTRAIT),
    (OBAMA_FRONT, EINSTEIN_FRONTAL),
    (MERKEL_PORTRAIT, MACRON_PORTRAIT),
    (CURIE_PORTRAIT, BOHR_PORTRAIT),
    (EINSTEIN_FRONTAL, CURIE_PORTRAIT),
)


# ─── Filesystem helpers ──────────────────────────────────────────────────────


def repository_dataset_dir() -> Path:
    """Return ``tests/dataset`` resolved from this file (usable at import time)."""
    # tests/faces/integration/support.py -> tests/dataset
    return Path(__file__).resolve().parents[2] / "dataset"


def all_face_photos(dataset_dir: Path | str) -> tuple[str, ...]:
    """Return every face photo of the dataset as dataset-relative POSIX paths."""
    root = Path(dataset_dir) / FACES_DIR
    return tuple(
        sorted(path.relative_to(Path(dataset_dir)).as_posix() for path in root.rglob("*.jpg"))
    )


def image_b64(dataset_dir: Path | str, rel_path: str) -> str:
    """Read a dataset image and return it as a base64 string."""
    path = Path(dataset_dir) / rel_path
    return base64.b64encode(path.read_bytes()).decode()


def image_dimensions(dataset_dir: Path | str, rel_path: str) -> tuple[int, int]:
    """Return ``(width, height)`` of a dataset image (requires Pillow)."""
    from PIL import Image

    with Image.open(Path(dataset_dir) / rel_path) as image:
        width, height = image.size
    return int(width), int(height)


# ─── Math helpers ────────────────────────────────────────────────────────────


def cosine(a: Sequence[float], b: Sequence[float]) -> float:
    """Return the cosine similarity of two vectors (``0.0`` for degenerate input)."""
    if not a or not b:
        return 0.0
    dot = math.fsum(x * y for x, y in zip(a, b))
    norm_a = math.sqrt(math.fsum(x * x for x in a))
    norm_b = math.sqrt(math.fsum(y * y for y in b))
    if norm_a == 0.0 or norm_b == 0.0:
        return 0.0
    return dot / (norm_a * norm_b)


def best_match(embedding: Sequence[float], references: Sequence[Sequence[float]]) -> float:
    """Return the highest cosine similarity between *embedding* and *references*."""
    return max((cosine(embedding, reference) for reference in references), default=0.0)


def box_area(box: Mapping[str, float]) -> float:
    """Return the pixel area of a detection box."""
    return float(box["width"]) * float(box["height"])


def largest_box(boxes: Sequence[Mapping[str, float]]) -> Optional[Mapping[str, float]]:
    """Return the biggest detected box — the portrait subject in a single-face photo."""
    if not boxes:
        return None
    return max(boxes, key=box_area)


def describe_boxes(boxes: Sequence[Mapping[str, float]]) -> str:
    """Format detections for readable assertion messages."""
    return (
        ", ".join(
            f"({box['x']:.0f},{box['y']:.0f} {box['width']:.0f}x{box['height']:.0f} "
            f"conf={box['confidence']:.2f})"
            for box in boxes
        )
        or "none"
    )


# ─── HTTP client ─────────────────────────────────────────────────────────────


class FacesServiceError(RuntimeError):
    """Raised when the faces service is unreachable or answers with an error."""

    def __init__(self, url: str, status: Optional[int], body: str) -> None:
        status_label = "no response" if status is None else f"HTTP {status}"
        super().__init__(f"{url} -> {status_label}: {body[:500]}")
        self.url = url
        self.status = status
        self.body = body


class FacesService:
    """HTTP client for the faces service with per-run inference caching.

    Detection and embedding inference dominate the suite runtime, so both are
    memoised: every photo is sent to the service once per pytest run and all
    tests then compare the returned vectors locally.
    """

    def __init__(
        self,
        api_url: str,
        dataset_dir: Path | str,
        timeout: float = DEFAULT_REQUEST_TIMEOUT,
    ) -> None:
        self.api_url = api_url.rstrip("/")
        self.dataset_dir = Path(dataset_dir)
        self.timeout = timeout
        self._detections: dict[str, list[dict[str, float]]] = {}
        self._embeddings: dict[tuple[str, Optional[tuple[float, ...]]], list[float]] = {}

    # ── raw transport ────────────────────────────────────────────────────────

    def _call(self, method: str, path: str, payload: Optional[dict] = None) -> requests.Response:
        url = f"{self.api_url}{path}"
        try:
            response = requests.request(method, url, json=payload, timeout=self.timeout)
        except requests.RequestException as exc:
            raise FacesServiceError(url, None, str(exc)) from exc
        if response.status_code >= 400:
            raise FacesServiceError(url, response.status_code, response.text)
        return response

    def health(self) -> dict:
        """``GET /faces/health``."""
        return self._call("GET", "/health").json()

    def detect_raw(self, image: str) -> list[dict[str, float]]:
        """``POST /faces/detect`` with a base64 payload (no caching)."""
        return list(self._call("POST", "/detect", {"image": image}).json().get("boxes", []))

    def embed_raw(
        self, image: str, box: Optional[dict[str, float]] = None
    ) -> list[float]:
        """``POST /faces/embed`` with a base64 payload (no caching)."""
        payload: dict = {"image": image}
        if box is not None:
            payload["box"] = box
        return list(self._call("POST", "/embed", payload).json().get("embedding", []))

    def cluster(
        self, embeddings: Sequence[Sequence[float]], min_cluster_size: int = DEFAULT_CLUSTER_MIN_SIZE
    ) -> list[int]:
        """``POST /faces/cluster`` — returns one label per embedding (``-1`` is noise)."""
        payload = {
            "embeddings": [list(embedding) for embedding in embeddings],
            "minClusterSize": min_cluster_size,
        }
        return list(self._call("POST", "/cluster", payload).json().get("labels", []))

    # ── dataset-aware helpers (cached) ───────────────────────────────────────

    def image(self, rel_path: str) -> str:
        """Return a dataset photo as a base64 string."""
        return image_b64(self.dataset_dir, rel_path)

    def detect_file(self, rel_path: str) -> list[dict[str, float]]:
        """Detection boxes for a dataset photo (cached)."""
        if rel_path not in self._detections:
            self._detections[rel_path] = self.detect_raw(self.image(rel_path))
        return self._detections[rel_path]

    def embed_file(self, rel_path: str, box: Optional[dict[str, float]] = None) -> list[float]:
        """Embedding of a dataset photo, optionally restricted to *box* (cached)."""
        key = (rel_path, _box_key(box))
        if key not in self._embeddings:
            self._embeddings[key] = self.embed_raw(self.image(rel_path), box)
        return self._embeddings[key]

    def portrait_embedding(self, rel_path: str) -> list[float]:
        """Embedding of the biggest face in a single-face portrait (``[]`` if none)."""
        box = largest_box(self.detect_file(rel_path))
        if box is None:
            return []
        return self.embed_file(rel_path, dict(box))

    def face_embeddings(self, rel_path: str) -> list[tuple[dict[str, float], list[float]]]:
        """``[(box, embedding), ...]`` for every detected face in a photo."""
        return [(dict(box), self.embed_file(rel_path, dict(box))) for box in self.detect_file(rel_path)]

    def identity_references(self, identity: str) -> list[list[float]]:
        """Portrait embeddings that serve as the gallery for *identity*."""
        references = []
        for rel_path in IDENTITY_PORTRAITS[identity]:
            embedding = self.portrait_embedding(rel_path)
            if embedding:
                references.append(embedding)
        return references


def _box_key(box: Optional[Mapping[str, float]]) -> Optional[tuple[float, ...]]:
    """Build a cache key from a detection box (values rounded to 0.1 px)."""
    if box is None:
        return None
    return tuple(round(float(box[name]), 1) for name in ("x", "y", "width", "height"))
