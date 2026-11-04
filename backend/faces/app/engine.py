"""ML engine wrapping InsightFace detection/embedding and HDBSCAN clustering."""

from __future__ import annotations

import base64
import io
import logging
from typing import Any, Optional

import numpy as np
from PIL import Image

logger = logging.getLogger("faces.engine")

# ArcFace (w600k_r50) produces 512-dimensional embeddings.
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

#: A new person is only created from leftover faces when one of them is the sole
#: face of its photo, i.e. a portrait. Two faces cropped out of group shots never
#: become a person on their own: without a portrait anchor a loose match is far
#: more likely to be a lookalike than the same person.

#: How much the best match must beat the runner-up before a leftover face is
#: linked: an ambiguous match stays unassigned instead of guessing.
RECOGNITION_MARGIN = 0.05


def _box_bounds(box: dict[str, float]) -> tuple[float, float, float, float]:
    """Convert an ``{x, y, width, height}`` box to ``(x1, y1, x2, y2)``."""
    x1 = float(box.get("x", 0.0))
    y1 = float(box.get("y", 0.0))
    return x1, y1, x1 + float(box.get("width", 0.0)), y1 + float(box.get("height", 0.0))


def _bbox_area(bbox: Any) -> float:
    """Area of an insightface ``bbox`` (``[x1, y1, x2, y2]``)."""
    x1, y1, x2, y2 = (float(v) for v in bbox)
    return max(0.0, x2 - x1) * max(0.0, y2 - y1)


def _iou(a: tuple[float, float, float, float], b: tuple[float, float, float, float]) -> float:
    """Intersection-over-union of two ``(x1, y1, x2, y2)`` rectangles."""
    ix1, iy1 = max(a[0], b[0]), max(a[1], b[1])
    ix2, iy2 = min(a[2], b[2]), min(a[3], b[3])
    if ix2 <= ix1 or iy2 <= iy1:
        return 0.0
    intersection = (ix2 - ix1) * (iy2 - iy1)
    union = (a[2] - a[0]) * (a[3] - a[1]) + (b[2] - b[0]) * (b[3] - b[1]) - intersection
    if union <= 0.0:
        return 0.0
    return intersection / union


class FaceEngine:
    """Loads and runs the face recognition models.

    Model loading is lazy and the class keeps no state beyond the loaded
    analysis object, so endpoints stay stateless and tests can substitute a
    deterministic fake engine.
    """

    def __init__(self, name: str = "buffalo_l", providers: Optional[list[str]] = None) -> None:
        self._name = name
        self._providers = providers or ["CPUExecutionProvider"]
        self._analysis: Any = None

    def ensure_loaded(self) -> Any:
        if self._analysis is None:
            # Imported lazily so the service (and its tests) do not require the
            # heavy ML dependencies until a real inference runs.
            from insightface.app import FaceAnalysis

            app = FaceAnalysis(name=self._name, providers=self._providers)
            # ctx_id=0 selects CPU; GPU remains a documented future option.
            app.prepare(ctx_id=0, det_size=(640, 640))
            self._analysis = app
            # Format the model name into the message: an ``extra={"name": ...}``
            # key collides with LogRecord.name and raises on Python 3.12.
            logger.info("face models loaded: %s", self._name)
        return self._analysis

    @staticmethod
    def decode_image(image_b64: str) -> np.ndarray:
        try:
            raw = base64.b64decode(image_b64)
            return np.asarray(Image.open(io.BytesIO(raw)).convert("RGB"))
        except Exception as exc:  # noqa: BLE001 - surface a uniform error
            raise ValueError(f"invalid image payload: {exc}") from exc

    def detect(self, image_b64: str) -> list[dict[str, float]]:
        img = self.decode_image(image_b64)
        faces = self.ensure_loaded().get(img)
        boxes: list[dict[str, float]] = []
        for face in faces:
            x1, y1, x2, y2 = (float(v) for v in face.bbox)
            boxes.append(
                {
                    "x": x1,
                    "y": y1,
                    "width": x2 - x1,
                    "height": y2 - y1,
                    "confidence": float(face.det_score),
                }
            )
        return boxes

    def analyze(self, image_b64: str) -> list[dict[str, Any]]:
        """Detect every face of an image and embed all of them in one pass.

        Detecting once and embedding every detection from that single pass is
        what keeps group photos complete: re-running detection per face costs a
        full model pass each and a 27-face photo exceeded the caller's request
        budget, so most faces were never stored.
        """
        img = self.decode_image(image_b64)
        faces = self.ensure_loaded().get(img)
        return [self._describe_face(img, face) for face in faces]

    def _describe_face(self, img: np.ndarray, face: Any) -> dict[str, Any]:
        """Return the box, confidence and (refined) embedding of one detection."""
        x1, y1, x2, y2 = (float(v) for v in face.bbox)
        embedding = np.asarray(face.normed_embedding, dtype=float)
        if x2 - x1 < SMALL_FACE_WIDTH:
            refined = self._embed_upscaled(img, (x1, y1, x2, y2))
            if refined is not None:
                embedding = refined
        return {
            "x": x1,
            "y": y1,
            "width": x2 - x1,
            "height": y2 - y1,
            "confidence": float(face.det_score),
            "embedding": embedding.tolist(),
        }

    def _embed_upscaled(
        self, img: np.ndarray, box: tuple[float, float, float, float]
    ) -> Optional[np.ndarray]:
        """Embed one small face from an upscaled padded crop of the image.

        The crop keeps ``UPSCALE_PADDING`` box widths of context — a tight crop
        starves the detector — and is upscaled so the face is about
        ``UPSCALE_TARGET_WIDTH`` pixels wide before detection runs on it.
        Returns ``None`` when the crop yields no face, so the caller keeps the
        embedding from the full frame.
        """
        x1, y1, x2, y2 = box
        width, height = x2 - x1, y2 - y1
        center_x, center_y = (x1 + x2) / 2, (y1 + y2) / 2
        half = max(width, height) * UPSCALE_PADDING / 2
        left = int(max(0, center_x - half))
        top = int(max(0, center_y - half))
        right = int(min(img.shape[1], center_x + half))
        bottom = int(min(img.shape[0], center_y + half))
        if right <= left or bottom <= top:
            return None

        crop = Image.fromarray(img[top:bottom, left:right])
        scale = UPSCALE_TARGET_WIDTH / max(1.0, width)
        crop = crop.resize(
            (max(1, int(crop.width * scale)), max(1, int(crop.height * scale))),
            Image.LANCZOS,
        )
        faces = self.ensure_loaded().get(np.asarray(crop))
        if not faces:
            return None

        expected = (
            (x1 - left) * scale,
            (y1 - top) * scale,
            (x2 - left) * scale,
            (y2 - top) * scale,
        )
        best: Optional[Any] = None
        best_iou = 0.0
        for candidate in faces:
            cx1, cy1, cx2, cy2 = (float(v) for v in candidate.bbox)
            iou = _iou((cx1, cy1, cx2, cy2), expected)
            if iou > best_iou:
                best, best_iou = candidate, iou
        if best is None or best_iou < UPSCALE_MIN_IOU:
            return None
        return np.asarray(best.normed_embedding, dtype=float)

    def embed(self, image_b64: str, box: Optional[dict[str, float]] = None) -> list[float]:
        """Return the ArcFace embedding of one face in the image.

        Detection always runs over the whole image: SCRFD needs the context
        around a face and finds nothing in a tight crop of a detection box, so
        ``box`` *selects* one of the detected faces (the one overlapping it the
        most) instead of being used as the crop region. Without a box the
        largest detected face is embedded. Small faces are embedded from an
        upscaled crop, exactly like ``analyze`` does.
        """
        img = self.decode_image(image_b64)
        faces = self.ensure_loaded().get(img)
        if not faces:
            return []
        face = self._select_face(faces, box) if box is not None else self._largest_face(faces)
        if face is None:
            return []
        return self._describe_face(img, face)["embedding"]

    @classmethod
    def _select_face(cls, faces: list[Any], box: dict[str, float]) -> Optional[Any]:
        """Return the detected face overlapping *box* the most (``None`` if none does)."""
        target = _box_bounds(box)
        best: Optional[Any] = None
        best_iou = 0.0
        for face in faces:
            x1, y1, x2, y2 = (float(v) for v in face.bbox)
            iou = _iou((x1, y1, x2, y2), target)
            if iou > best_iou:
                best, best_iou = face, iou
        return best

    @staticmethod
    def _largest_face(faces: list[Any]) -> Optional[Any]:
        """Return the face with the biggest bounding box (the portrait subject)."""
        if not faces:
            return None
        return max(faces, key=lambda face: _bbox_area(face.bbox))

    @staticmethod
    def cluster(
        embeddings: list[list[float]],
        min_cluster_size: int = 2,
        solo_faces: Optional[list[bool]] = None,
    ) -> list[int]:
        """Group embeddings into people with a cosine-distance threshold.

        DBSCAN runs over cosine distance: two faces closer than
        ``1 - CLUSTER_MIN_SIMILARITY`` are the same person, and a person needs
        at least ``min_cluster_size`` faces to form a cluster. The default of
        two keeps a person whose photos are few (a small gallery or a family
        archive) clusterable; a larger minimum turns every face into noise.
        Embeddings that belong to no cluster keep the label ``-1`` (noise), so
        a face seen only once does not become a person of its own.

        Faces left over by the strict pass are then recognised: a small face in
        a group photo stays below ``CLUSTER_MIN_SIMILARITY`` even for the same
        person, so leftover faces that agree with an existing group above
        ``RECOGNITION_MIN_SIMILARITY`` (and beat the runner-up by
        ``RECOGNITION_MARGIN``) are linked instead of dropped.
        ``solo_faces`` marks the embeddings that are the sole face of their
        photo; only such a portrait may anchor a new person together with the
        leftover faces matching it.

        A density hierarchy (HDBSCAN) was rejected here: on a small, well
        separated gallery it labels every face as noise, and on a tiny gallery
        it can merge unrelated people — an absolute similarity threshold is what
        ArcFace embeddings are calibrated for.
        """
        from sklearn.cluster import DBSCAN

        if not embeddings:
            return []
        arr = np.asarray(embeddings, dtype=float)
        norms = np.linalg.norm(arr, axis=1, keepdims=True)
        norms[norms == 0] = 1.0
        normalized = arr / norms
        clusterer = DBSCAN(
            eps=1.0 - CLUSTER_MIN_SIMILARITY,
            min_samples=max(1, int(min_cluster_size)),
            metric="cosine",
        )
        labels = [int(label) for label in clusterer.fit_predict(normalized)]
        return FaceEngine._recognise_leftovers(
            normalized, labels, max(2, int(min_cluster_size)), solo_faces or []
        )

    @staticmethod
    def _recognise_leftovers(
        normalized: np.ndarray,
        labels: list[int],
        min_cluster_size: int,
        solo_faces: list[bool],
    ) -> list[int]:
        """Link faces the strict pass left as noise to the group they belong to.

        Every leftover face is judged on its own — never through a chain of other
        leftovers, which would merge a whole group photo into one person:

        1. it joins a group the strict pass already formed when its best match
           against that group's *portraits* clears ``RECOGNITION_MIN_SIMILARITY``
           and beats the runner-up group by ``RECOGNITION_MARGIN``;
        2. otherwise a portrait (the sole face of its photo) anchors a new person
           and the leftovers matching *that portrait* unambiguously join it.

        A face without a portrait never creates a person, and a portrait nobody
        matches keeps ``-1``, so no one-photo person appears.
        """

        def is_solo(index: int) -> bool:
            return 0 <= index < len(solo_faces) and bool(solo_faces[index])

        leftovers = [index for index, label in enumerate(labels) if label == -1]
        if not leftovers:
            return labels

        def best_match(index: int, targets: dict[int, list[int]]) -> tuple[Optional[int], float, float]:
            """Best target label, its score, and the runner-up score."""
            best_label: Optional[int] = None
            best_score = runner_up = -1.0
            for label, members in targets.items():
                score = max(float(normalized[index] @ normalized[member]) for member in members)
                if score > best_score:
                    best_score, runner_up, best_label = score, best_score, label
                elif score > runner_up:
                    runner_up = score
            return best_label, best_score, runner_up

        def decisive(index: int, targets: dict[int, list[int]]) -> Optional[int]:
            label, score, runner_up = best_match(index, targets)
            if (
                label is not None
                and score >= RECOGNITION_MIN_SIMILARITY
                and score - runner_up >= RECOGNITION_MARGIN
            ):
                return label
            return None

        # Only portraits are trustworthy recognition anchors: a face cropped from
        # a group shot is similar to the other people standing next to it, so
        # matching against such members pulled whole group photos into one person.
        groups: dict[int, list[int]] = {}
        for index, label in enumerate(labels):
            if label != -1 and is_solo(index):
                groups.setdefault(label, []).append(index)

        resolved = list(labels)
        for index in leftovers:
            label = decisive(index, groups)
            if label is not None:
                resolved[index] = label

        anchors = [index for index in leftovers if is_solo(index) and resolved[index] == -1]
        if not anchors:
            return resolved

        # Compare against the portraits only: the members collected here must not
        # become targets themselves, or a lookalike chain would merge strangers.
        anchor_targets = {anchor: [anchor] for anchor in anchors}
        anchored_members = {anchor: [anchor] for anchor in anchors}
        for index in leftovers:
            if resolved[index] != -1 or index in anchored_members:
                continue
            anchor = decisive(index, anchor_targets)
            if anchor is not None:
                anchored_members[anchor].append(index)

        next_label = max(labels) + 1
        for members in anchored_members.values():
            if len(members) < min_cluster_size:
                continue
            for member in members:
                resolved[member] = next_label
            next_label += 1
        return resolved
