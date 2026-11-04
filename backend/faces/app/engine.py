"""ML engine wrapping InsightFace detection/embedding and HDBSCAN clustering."""

from __future__ import annotations

import base64
import io
import logging
import queue
import threading
from contextlib import contextmanager
from dataclasses import dataclass, replace
from typing import Any, Iterator, Optional, Sequence

import numpy as np
from PIL import Image

from app.config import (
    CLUSTER_MIN_SIMILARITY,
    CUDA_PROVIDERS,
    EMBEDDING_DIM,
    RECOGNITION_MARGIN,
    RECOGNITION_MIN_SIMILARITY,
    SMALL_FACE_WIDTH,
    UPSCALE_MIN_IOU,
    UPSCALE_PADDING,
    UPSCALE_TARGET_WIDTH,
    FaceSettings,
    ProviderSelection,
    available_providers,
    expand_provider,
    resolve_providers,
)

logger = logging.getLogger("faces.engine")


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


def _l2_normalize(features: np.ndarray) -> np.ndarray:
    """Normalise each row to unit length, as ``get()`` does for one face."""
    norms = np.linalg.norm(features, axis=1, keepdims=True)
    norms[norms == 0] = 1.0
    return features / norms


@dataclass
class DeviceWorker:
    """One model session pinned to a device (or the CPU fallback)."""

    device_id: int
    selection: ProviderSelection
    analysis: Any = None


class FaceEngine:
    """Loads and runs the face recognition models.

    One InsightFace session is kept per configured CUDA device
    (``FACES_GPU_IDS``), so a request submitted while one card is busy runs on
    the free one. Model loading stays lazy and the endpoints stay stateless.
    """

    def __init__(
        self,
        name: str = "buffalo_l",
        providers: Optional[Sequence[str]] = None,
        settings: Optional[FaceSettings] = None,
        available: Optional[Sequence[str]] = None,
        analysis: Optional[Any] = None,
    ) -> None:
        self._name = name
        self._settings = settings or FaceSettings.from_env()
        if providers is not None:
            expanded = tuple(expand_provider(provider) for provider in providers)
            self._settings = replace(self._settings, providers=expanded or self._settings.providers)
        # ``available`` lets tests describe a host's runtime without monkeypatching
        # onnxruntime; production resolves it lazily on first use.
        self._available: Optional[tuple[str, ...]] = (
            tuple(available) if available is not None else None
        )
        # ``analysis`` injects a ready fake session (tests); the primary device
        # uses it instead of loading real weights.
        self._analysis: Any = analysis
        self._workers: Optional[list[DeviceWorker]] = None
        self._free_workers: Optional[queue.Queue[DeviceWorker]] = None
        self._workers_lock = threading.Lock()

    @property
    def settings(self) -> FaceSettings:
        """The effective service settings (provider list, batch knobs)."""
        return self._settings

    @property
    def device_count(self) -> int:
        """Number of devices the pool can use in parallel."""
        return len(self._ensure_workers())

    def device_slots(self) -> int:
        """Total concurrent analysis slots (devices x per-device concurrency)."""
        return self.device_count * max(1, self._settings.max_concurrency)

    def device_info(self) -> list[dict[str, Any]]:
        """Describe every session in the pool for ``/faces/health``."""
        return [
            {
                "deviceId": worker.device_id,
                "provider": worker.selection.active_provider,
            }
            for worker in self._ensure_workers()
        ]

    def _ensure_workers(self) -> list[DeviceWorker]:
        """Build the device pool once; provider resolution happens here."""
        with self._workers_lock:
            if self._workers is not None:
                return self._workers

            offered = self._available if self._available is not None else available_providers()
            selection = resolve_providers(self._settings.providers, offered, 0)
            if selection.device_id >= 0:
                gpu_ids = self._settings.gpu_ids or (selection.device_id,)
                workers = [
                    DeviceWorker(device_id, replace(selection, device_id=device_id))
                    for device_id in gpu_ids
                ]
            else:
                # CPU fallback: one worker is all a CPU host can use.
                workers = [DeviceWorker(-1, selection)]

            free: queue.Queue[DeviceWorker] = queue.Queue()
            for worker in workers:
                for _ in range(max(1, self._settings.max_concurrency)):
                    free.put(worker)

            self._workers = workers
            self._free_workers = free
            logger.info(
                "execution providers: requested=%s available=%s devices=%s",
                list(selection.requested),
                list(selection.available),
                [(worker.device_id, worker.selection.active_provider) for worker in workers],
            )
            return workers

    def provider_selection(self) -> ProviderSelection:
        """Selection of the first device, kept for the health payload."""
        return self._ensure_workers()[0].selection

    @contextmanager
    def _device(self) -> Iterator[DeviceWorker]:
        """Borrow a free session; waits when every device is busy."""
        self._ensure_workers()
        assert self._free_workers is not None
        worker = self._free_workers.get()
        try:
            yield worker
        finally:
            self._free_workers.put(worker)

    def _analysis_for(self, worker: DeviceWorker) -> Any:
        if worker is self._ensure_workers()[0]:
            return self.ensure_loaded()
        if worker.analysis is None:
            worker.analysis = self._load_analysis(worker.selection)
        return worker.analysis

    def ensure_loaded(self) -> Any:
        """Load (or return) the session of the first device."""
        primary = self._ensure_workers()[0]
        if primary.analysis is None:
            primary.analysis = (
                self._analysis
                if self._analysis is not None
                else self._load_analysis(primary.selection)
            )
        return primary.analysis

    def _load_analysis(self, selection: ProviderSelection) -> Any:
        # Imported lazily so the service (and its tests) do not require the
        # heavy ML dependencies until a real inference runs.
        from insightface.app import FaceAnalysis

        providers = list(selection.active)
        kwargs: dict[str, Any] = {"name": self._name, "providers": providers}
        if selection.device_id >= 0:
            # InsightFace's ``prepare(ctx_id=...)`` only switches CPU vs CUDA;
            # the card itself is chosen by ONNX Runtime's provider options, so
            # the device id must be passed when the sessions are created.
            kwargs["provider_options"] = [
                {"device_id": selection.device_id} if provider in CUDA_PROVIDERS else {}
                for provider in providers
            ]
        app = FaceAnalysis(**kwargs)
        # ``ctx_id >= 0`` keeps InsightFace on the CUDA path; -1 uses CPU sessions.
        app.prepare(ctx_id=selection.device_id, det_size=(640, 640))
        logger.info(
            "face models loaded: %s on %s (device %d)",
            self._name,
            selection.active_provider,
            selection.device_id,
        )
        return app

    @staticmethod
    def decode_image(image_b64: str) -> np.ndarray:
        try:
            raw = base64.b64decode(image_b64)
            return np.asarray(Image.open(io.BytesIO(raw)).convert("RGB"))
        except Exception as exc:  # noqa: BLE001 - surface a uniform error
            raise ValueError(f"invalid image payload: {exc}") from exc

    def detect(self, image_b64: str) -> list[dict[str, float]]:
        img = self.decode_image(image_b64)
        with self._device() as worker:
            faces = self._analysis_for(worker).get(img)
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

        Small faces are refined from upscaled crops; their recognition forward
        passes are collected and sent through the recognizer in chunks of
        ``FACES_RECOGNITION_BATCH`` so one GPU submission covers many faces.
        """
        img = self.decode_image(image_b64)
        with self._device() as worker:
            analysis = self._analysis_for(worker)
            return self._describe_faces(img, analysis.get(img), analysis)

    def _describe_faces(
        self, img: np.ndarray, faces: list[Any], analysis: Any
    ) -> list[dict[str, Any]]:
        """Return box, confidence and (refined) embedding for every detection."""
        descriptions = [self._box_description(face) for face in faces]

        # Collect the refined small-face crops first: the recognizer can then be
        # called once per chunk instead of once per face.
        refinements: list[tuple[int, np.ndarray, Any]] = []
        for index, face in enumerate(faces):
            x1, y1, x2, y2 = (float(v) for v in face.bbox)
            if x2 - x1 >= SMALL_FACE_WIDTH:
                continue
            match = self._upscaled_match(img, (x1, y1, x2, y2), analysis)
            if match is None:
                continue
            crop, candidate = match
            if self._settings.recognition_batch <= 1:
                # Batch size one keeps today's exact numerics: the detection on
                # the upscaled crop already produced the refined embedding.
                descriptions[index]["embedding"] = np.asarray(
                    candidate.normed_embedding, dtype=float
                ).tolist()
            else:
                refinements.append((index, crop, candidate))

        if refinements:
            self._embed_crops(analysis, refinements, descriptions)
        return descriptions

    @staticmethod
    def _box_description(face: Any) -> dict[str, Any]:
        """The box and confidence of one detection, with its frame embedding."""
        x1, y1, x2, y2 = (float(v) for v in face.bbox)
        return {
            "x": x1,
            "y": y1,
            "width": x2 - x1,
            "height": y2 - y1,
            "confidence": float(face.det_score),
            "embedding": np.asarray(face.normed_embedding, dtype=float).tolist(),
        }

    def _embed_crops(
        self,
        analysis: Any,
        refinements: list[tuple[int, np.ndarray, Any]],
        descriptions: list[dict[str, Any]],
    ) -> None:
        """Embed refined crops in chunks, one recognizer pass per chunk."""
        from insightface.utils import face_align

        recognizer = analysis.models["recognition"]
        batch = max(1, self._settings.recognition_batch)
        for start in range(0, len(refinements), batch):
            chunk = refinements[start : start + batch]
            crops = np.stack(
                [face_align.norm_crop(crop, candidate.kps) for _, crop, candidate in chunk]
            )
            features = _l2_normalize(np.asarray(recognizer.get_feat(crops), dtype=float))
            for (index, _, _), embedding in zip(chunk, features):
                descriptions[index]["embedding"] = [float(v) for v in embedding]

    def _upscaled_match(
        self, img: np.ndarray, box: tuple[float, float, float, float], analysis: Any
    ) -> Optional[tuple[np.ndarray, Any]]:
        """Find the face of an upscaled padded crop that refines one small face.

        The crop keeps ``UPSCALE_PADDING`` box widths of context — a tight crop
        starves the detector — and is upscaled so the face is about
        ``UPSCALE_TARGET_WIDTH`` pixels wide before detection runs on it. The
        detection in that crop must overlap the expected position
        (``UPSCALE_MIN_IOU``) or the frame embedding is kept. Returns the crop
        and the matched detection, or ``None`` when the crop yields no face.
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
        upscaled = np.asarray(crop)
        faces = analysis.get(upscaled)
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
        return upscaled, best

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
        with self._device() as worker:
            analysis = self._analysis_for(worker)
            faces = analysis.get(img)
            if not faces:
                return []
            face = self._select_face(faces, box) if box is not None else self._largest_face(faces)
            if face is None:
                return []
            return self._describe_faces(img, [face], analysis)[0]["embedding"]

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
    def cluster_graph(
        face_ids: Sequence[int],
        edges: Sequence[tuple[int, int, float]],
        solo_faces: Optional[Sequence[bool]] = None,
        min_cluster_size: int = 2,
    ) -> list[int]:
        """Group faces from candidate links into people, one label per face.

        Core queries the vector store for each unassigned face's neighbours
        inside the clustering radius and posts those links instead of every
        embedding. Every threshold stays here: the strict floor
        (``CLUSTER_MIN_SIMILARITY``), the relaxed recognition floor
        (``RECOGNITION_MIN_SIMILARITY``) and the runner-up margin
        (``RECOGNITION_MARGIN``). Faces that belong to no person keep ``-1``.
        """
        count = len(face_ids)
        if count == 0:
            return []

        index: dict[int, int] = {}
        for position, face_id in enumerate(face_ids):
            index.setdefault(int(face_id), position)

        links: dict[tuple[int, int], float] = {}
        for face_id, candidate_id, similarity in edges:
            left, right = index.get(int(face_id)), index.get(int(candidate_id))
            if left is None or right is None or left == right:
                continue
            key = (min(left, right), max(left, right))
            links[key] = max(links.get(key, 0.0), float(similarity))

        def similarity(left: int, right: int) -> float:
            if left == right:
                return 1.0
            return links.get((min(left, right), max(left, right)), 0.0)

        # Strict pass: connected faces at or above the clustering floor form a
        # group, exactly like the density pass does over the full matrix.
        parent = list(range(count))

        def find(node: int) -> int:
            while parent[node] != node:
                parent[node] = parent[parent[node]]
                node = parent[node]
            return node

        for (left, right), value in links.items():
            if value < CLUSTER_MIN_SIMILARITY:
                continue
            root_left, root_right = find(left), find(right)
            if root_left != root_right:
                parent[root_right] = root_left

        components: dict[int, list[int]] = {}
        for position in range(count):
            components.setdefault(find(position), []).append(position)

        minimum = max(2, int(min_cluster_size))
        labels = [-1] * count
        groups: dict[int, list[int]] = {}
        next_label = 0
        for members in sorted(components.values(), key=lambda members: members[0]):
            if len(members) < minimum:
                continue
            for member in members:
                labels[member] = next_label
            groups[next_label] = members
            next_label += 1

        def is_solo(position: int) -> bool:
            return 0 <= position < len(solo_faces or []) and bool(solo_faces[position])

        def decisive(position: int, targets: dict[int, list[int]]) -> Optional[int]:
            best_label: Optional[int] = None
            best_score = runner_up = -1.0
            for label, members in targets.items():
                score = max(similarity(position, member) for member in members)
                if score > best_score:
                    best_score, runner_up, best_label = score, best_score, label
                elif score > runner_up:
                    runner_up = score
            if (
                best_label is not None
                and best_score >= RECOGNITION_MIN_SIMILARITY
                and best_score - runner_up >= RECOGNITION_MARGIN
            ):
                return best_label
            return None

        # Only portraits may recognise a leftover: a face cropped from a group
        # shot is similar to the people standing next to it.
        recognition_targets = {
            label: [member for member in members if is_solo(member)]
            for label, members in groups.items()
        }
        recognition_targets = {
            label: members for label, members in recognition_targets.items() if members
        }

        leftovers = [position for position, label in enumerate(labels) if label == -1]
        for position in leftovers:
            label = decisive(position, recognition_targets)
            if label is not None:
                labels[position] = label

        anchors = [position for position in leftovers if is_solo(position) and labels[position] == -1]
        if not anchors:
            return labels

        # A portrait anchors a new person together with the leftovers that match
        # it unambiguously; leftovers never become targets themselves, or a
        # lookalike chain would merge strangers.
        anchor_targets = {anchor: [anchor] for anchor in anchors}
        anchored_members = {anchor: [anchor] for anchor in anchors}
        for position in leftovers:
            if labels[position] != -1 or position in anchored_members:
                continue
            anchor = decisive(position, anchor_targets)
            if anchor is not None:
                anchored_members[anchor].append(position)

        for members in anchored_members.values():
            if len(members) < minimum:
                continue
            for member in members:
                labels[member] = next_label
            next_label += 1
        return labels

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
