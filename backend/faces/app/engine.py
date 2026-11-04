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
            logger.info("face models loaded", extra={"name": self._name})
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

    def embed(self, image_b64: str, box: Optional[dict[str, float]] = None) -> list[float]:
        img = self.decode_image(image_b64)
        if box is not None:
            img = self._crop(img, box)
        faces = self.ensure_loaded().get(img)
        if not faces:
            return []
        embedding = np.asarray(faces[0].normed_embedding, dtype=float)
        return embedding.tolist()

    @staticmethod
    def _crop(img: np.ndarray, box: dict[str, float]) -> np.ndarray:
        height, width = img.shape[:2]
        x = max(0, int(box.get("x", 0.0)))
        y = max(0, int(box.get("y", 0.0)))
        w = max(1, int(box.get("width", 0.0)))
        h = max(1, int(box.get("height", 0.0)))
        x2 = min(width, x + w)
        y2 = min(height, y + h)
        if x2 <= x or y2 <= y:
            return img
        return img[y:y2, x:x2]

    @staticmethod
    def cluster(embeddings: list[list[float]], min_cluster_size: int = 5) -> list[int]:
        """Cluster embeddings with HDBSCAN using the cosine metric.

        Cosine distance is evaluated by L2-normalizing each row and running
        HDBSCAN with Euclidean distance, which orders points identically to
        cosine distance. A label of -1 marks noise.
        """
        import hdbscan

        if not embeddings:
            return []
        arr = np.asarray(embeddings, dtype=float)
        norms = np.linalg.norm(arr, axis=1, keepdims=True)
        norms[norms == 0] = 1.0
        normalized = arr / norms
        clusterer = hdbscan.HDBSCAN(
            min_cluster_size=min_cluster_size,
            metric="euclidean",
            cluster_selection_method="eom",
        )
        labels = clusterer.fit_predict(normalized)
        return [int(label) for label in labels]
