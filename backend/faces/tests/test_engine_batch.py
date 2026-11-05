"""Batched recognition forward-pass tests.

Small faces are refined from upscaled crops. Their aligned crops are collected
and sent through the recognizer in chunks of ``FACES_RECOGNITION_BATCH`` so one
submission covers many faces; a batch size of one keeps the per-image path and
therefore its exact numerics.
"""

from __future__ import annotations

import base64
import io

import numpy as np
import pytest
from PIL import Image

from app.config import EMBEDDING_DIM, FaceSettings
from app.engine import FaceEngine

FRAME_WIDTH = 128
REFINED_BOX = (80.0, 80.0, 240.0, 240.0)
#: All five landmarks land inside a 320 px upscaled crop.
KPS = np.array(
    [[80.0, 80.0], [160.0, 80.0], [120.0, 120.0], [80.0, 160.0], [160.0, 160.0]],
    dtype=np.float32,
)
UNIT = np.zeros(EMBEDDING_DIM)
UNIT[0] = 1.0

#: Boxes narrower than SMALL_FACE_WIDTH (64 px) on a 128 px frame.
SMALL_BOXES = [(10, 10, 30, 30), (40, 10, 60, 30), (70, 10, 90, 30)]


def _frame_b64(width: int = FRAME_WIDTH) -> str:
    img = Image.new("RGB", (width, width), color=(90, 60, 40))
    buf = io.BytesIO()
    img.save(buf, format="JPEG")
    return base64.b64encode(buf.getvalue()).decode()


class FakeFace:
    def __init__(self, bbox: tuple[float, float, float, float]) -> None:
        self.bbox = np.asarray(bbox, dtype=np.float32)
        self.det_score = 0.99
        self.kps = KPS
        self.normed_embedding = UNIT


class FakeRecognition:
    """Records how many crops each recognizer call received."""

    def __init__(self) -> None:
        self.calls: list[int] = []

    def get_feat(self, crops) -> np.ndarray:
        self.calls.append(len(crops))
        return np.stack([UNIT.copy() for _ in range(len(crops))])


class InsightFaceRecognition(FakeRecognition):
    """Mimics the real ``insightface`` ``ArcFaceONNX.get_feat`` contract.

    The installed recognizer wraps any non-list argument into a one-element
    list, so a stacked ndarray reaches OpenCV as a single four-dimensional
    image and ``cv2.dnn.blobFromImages`` rejects it. This fake fails the same
    way, which keeps the engine's call shape honest.
    """

    def get_feat(self, crops) -> np.ndarray:
        if not isinstance(crops, (list, tuple)):
            raise TypeError(
                "get_feat expects a list of aligned crops, "
                f"got a single {type(crops).__name__}"
            )
        for crop in crops:
            assert getattr(crop, "ndim", None) == 3, "each crop must be HxWxC"
        return super().get_feat(crops)


class FakeAnalysis:
    """Detects the configured boxes on the frame and one face per crop."""

    def __init__(self, frame_boxes, crop_faces: bool = True) -> None:
        self.frame_boxes = frame_boxes
        self.crop_faces = crop_faces
        self.models = {"recognition": FakeRecognition()}

    def get(self, img):
        if img.shape[1] == FRAME_WIDTH:
            return [FakeFace(box) for box in self.frame_boxes]
        return [FakeFace(REFINED_BOX)] if self.crop_faces else []


def _engine(analysis: FakeAnalysis, recognition_batch: int) -> FaceEngine:
    engine = FaceEngine(settings=FaceSettings(recognition_batch=recognition_batch))
    # Inject the fake session so no model is downloaded; the batching code under
    # test is the real one.
    engine._analysis = analysis
    return engine


def _cosine(a: list[float], b: list[float]) -> float:
    va, vb = np.asarray(a, dtype=float), np.asarray(b, dtype=float)
    return float(va @ vb / (np.linalg.norm(va) * np.linalg.norm(vb)))


def test_aligned_crops_are_chunked_by_the_configured_batch() -> None:
    analysis = FakeAnalysis(SMALL_BOXES)
    engine = _engine(analysis, recognition_batch=2)

    faces = engine.analyze(_frame_b64())

    assert len(faces) == 3
    # Three crops with a chunk size of two means two recognizer passes.
    assert analysis.models["recognition"].calls == [2, 1]
    for face in faces:
        assert face["embedding"] == pytest.approx(UNIT.tolist())


def test_refined_crops_reach_the_recognizer_as_a_list() -> None:
    analysis = FakeAnalysis(SMALL_BOXES)
    analysis.models["recognition"] = InsightFaceRecognition()
    engine = _engine(analysis, recognition_batch=32)

    faces = engine.analyze(_frame_b64())

    assert len(faces) == 3
    assert analysis.models["recognition"].calls == [3]


def test_batch_size_one_matches_the_batched_embeddings() -> None:
    single_analysis = FakeAnalysis(SMALL_BOXES)
    batched_analysis = FakeAnalysis(SMALL_BOXES)

    single = _engine(single_analysis, recognition_batch=1).analyze(_frame_b64())
    batched = _engine(batched_analysis, recognition_batch=32).analyze(_frame_b64())

    assert single_analysis.models["recognition"].calls == []
    assert batched_analysis.models["recognition"].calls == [3]
    for left, right in zip(single, batched):
        assert _cosine(left["embedding"], right["embedding"]) >= 0.99


def test_large_faces_keep_their_frame_embedding() -> None:
    analysis = FakeAnalysis([(0, 0, 100, 100)])
    engine = _engine(analysis, recognition_batch=32)

    faces = engine.analyze(_frame_b64())

    assert analysis.models["recognition"].calls == []
    assert faces[0]["embedding"] == pytest.approx(UNIT.tolist())


def test_a_crop_without_a_face_falls_back_to_the_frame_embedding() -> None:
    analysis = FakeAnalysis(SMALL_BOXES, crop_faces=False)
    engine = _engine(analysis, recognition_batch=32)

    faces = engine.analyze(_frame_b64())

    assert analysis.models["recognition"].calls == []
    assert faces[0]["embedding"] == pytest.approx(UNIT.tolist())
