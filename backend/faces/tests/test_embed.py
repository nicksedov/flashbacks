"""Embedding endpoint tests."""

from __future__ import annotations

import logging
import math
import sys
import types

import numpy as np

from app.engine import EMBEDDING_DIM, FaceEngine


class StubFace:
    """Minimal stand-in for an insightface ``Face``."""

    #: Landmarks inside an upscaled crop, used by the batched recognizer path.
    KPS = np.array(
        [[120.0, 120.0], [220.0, 120.0], [170.0, 170.0], [120.0, 220.0], [220.0, 220.0]],
        dtype=float,
    )

    def __init__(
        self,
        bbox: tuple[float, float, float, float],
        embedding: list[float],
        det_score: float = 0.99,
    ) -> None:
        self.bbox = np.asarray(bbox, dtype=float)
        self.normed_embedding = np.asarray(embedding, dtype=float)
        self.det_score = det_score
        self.kps = self.KPS


class StubRecognition:
    """Recognizer stand-in returning one fixed feature per submitted crop."""

    def __init__(self, embedding: list[float]) -> None:
        self.embedding = np.asarray(embedding, dtype=float)
        self.calls: list[int] = []

    def get_feat(self, crops) -> np.ndarray:
        self.calls.append(len(crops))
        return np.stack([self.embedding for _ in range(len(crops))])


class StubAnalysis:
    """Deterministic replacement for ``FaceAnalysis`` that records calls."""

    def __init__(self, faces: list[StubFace]) -> None:
        self.faces = faces
        self.calls = 0
        self.models = {"recognition": StubRecognition([1.0, 0.0, 0.0])}

    def get(self, img: np.ndarray) -> list[StubFace]:  # noqa: ARG002 - image content is irrelevant
        self.calls += 1
        return list(self.faces)


class SequencedAnalysis:
    """Returns one prepared detection list per ``get`` call."""

    def __init__(
        self,
        responses: list[list[StubFace]],
        recognition_embedding: list[float] | None = None,
    ) -> None:
        self.responses = list(responses)
        self.calls = 0
        self.models = {
            "recognition": StubRecognition(recognition_embedding or [1.0, 0.0, 0.0])
        }

    def get(self, img: np.ndarray) -> list[StubFace]:  # noqa: ARG002 - image content is irrelevant
        self.calls += 1
        return self.responses.pop(0)


def engine_with(faces: list[StubFace]) -> tuple[FaceEngine, StubAnalysis]:
    """Return a ``FaceEngine`` whose model is replaced by a stub analysis."""
    engine = FaceEngine()
    stub = StubAnalysis(faces)
    engine.ensure_loaded = lambda: stub  # type: ignore[method-assign]
    return engine, stub


def test_analyze_rejects_a_crop_face_that_is_not_the_detected_face(image) -> None:
    """A dense group photo puts other faces inside the upscaled crop.

    The refined embedding is only accepted when the face found in the crop sits
    where the original detection was; otherwise the neighbouring face's vector
    would be stored and two different people would be merged.
    """
    detected = StubFace((10.0, 10.0, 40.0, 50.0), [1.0, 0.0, 0.0])
    neighbour = StubFace((500.0, 500.0, 560.0, 580.0), [0.0, 1.0, 0.0])
    analysis = SequencedAnalysis([[detected], [neighbour]])
    engine = FaceEngine()
    engine.ensure_loaded = lambda: analysis  # type: ignore[method-assign]

    faces = engine.analyze(image)

    assert len(faces) == 1
    assert faces[0]["embedding"] == [1.0, 0.0, 0.0]


def test_analyze_uses_the_refined_embedding_when_the_crop_face_matches(image) -> None:
    """The face found in the crop overlaps the detection: its vector wins."""
    detected = StubFace((10.0, 10.0, 40.0, 50.0), [1.0, 0.0, 0.0])
    refined = StubFace((53.0, 53.0, 213.0, 267.0), [0.0, 0.0, 1.0])
    analysis = SequencedAnalysis([[detected], [refined]], recognition_embedding=[0.0, 0.0, 1.0])
    engine = FaceEngine()
    engine.ensure_loaded = lambda: analysis  # type: ignore[method-assign]

    faces = engine.analyze(image)

    assert len(faces) == 1
    assert faces[0]["embedding"] == [0.0, 0.0, 1.0]


def test_embed_returns_512_vector(client, image) -> None:
    response = client.post("/faces/embed", json={"image": image})
    assert response.status_code == 200
    embedding = response.json()["embedding"]
    assert len(embedding) == EMBEDDING_DIM


def test_embed_accepts_a_box(client, image) -> None:
    box = {"x": 0.0, "y": 0.0, "width": 32.0, "height": 32.0, "confidence": 0.9}
    response = client.post("/faces/embed", json={"image": image, "box": box})
    assert response.status_code == 200
    assert len(response.json()["embedding"]) == EMBEDDING_DIM


# ─── Engine-level behaviour ──────────────────────────────────────────────────


def test_engine_box_embeds_the_overlapping_face(image) -> None:
    """A detection box selects the detected face it overlaps, not a crop."""
    left = StubFace((10.0, 10.0, 60.0, 80.0), [1.0, 0.0, 0.0])
    right = StubFace((200.0, 40.0, 260.0, 120.0), [0.0, 1.0, 0.0])
    engine, stub = engine_with([left, right])

    box = {"x": 200.0, "y": 40.0, "width": 60.0, "height": 80.0}
    embedding = engine.embed(image, box)

    assert embedding == [0.0, 1.0, 0.0]
    # Detection must run once over the whole image — a tight crop would make
    # SCRFD miss the face and return an empty embedding.
    assert stub.calls == 1


def test_engine_box_without_overlap_returns_no_embedding(image) -> None:
    """A box matching no detected face yields an empty embedding."""
    faces = [StubFace((10.0, 10.0, 60.0, 80.0), [1.0, 0.0, 0.0])]
    engine, _ = engine_with(faces)

    box = {"x": 400.0, "y": 400.0, "width": 50.0, "height": 50.0}
    assert engine.embed(image, box) == []


def test_engine_without_box_embeds_the_largest_face(image) -> None:
    """Without a box the biggest detected face is the portrait subject."""
    small = StubFace((10.0, 10.0, 40.0, 40.0), [1.0, 0.0, 0.0])
    large = StubFace((100.0, 100.0, 220.0, 260.0), [0.0, 0.0, 1.0])
    engine, _ = engine_with([small, large])

    assert engine.embed(image) == [0.0, 0.0, 1.0]


def test_engine_returns_no_embedding_when_no_face_is_detected(image) -> None:
    engine, _ = engine_with([])

    assert engine.embed(image) == []
    assert engine.embed(image, {"x": 0.0, "y": 0.0, "width": 10.0, "height": 10.0}) == []


def test_engine_embedding_is_l2_normalised(image) -> None:
    """The engine returns the model's normalised vector unchanged."""
    face = StubFace(
        (10.0, 10.0, 60.0, 80.0),
        [0.6, 0.8, 0.0, 0.0],
    )
    engine, _ = engine_with([face])

    embedding = engine.embed(image)
    assert math.isclose(math.sqrt(sum(value * value for value in embedding)), 1.0, rel_tol=1e-9)


def test_model_loading_logs_without_reserved_log_record_keys(monkeypatch, caplog) -> None:
    """Loading the model must not raise on a reserved ``LogRecord`` attribute.

    Python 3.12 raises when ``extra`` overwrites ``LogRecord.name``, which turned
    the first inference after a service start into an HTTP 500.
    """

    class StubAnalysis:
        def __init__(self, name: str, providers: list[str]) -> None:
            self.name = name
            self.providers = providers

        def prepare(self, **kwargs: object) -> None:  # noqa: ARG002 - signature parity
            return None

    package = types.ModuleType("insightface")
    module = types.ModuleType("insightface.app")
    module.FaceAnalysis = StubAnalysis  # type: ignore[attr-defined]
    monkeypatch.setitem(sys.modules, "insightface", package)
    monkeypatch.setitem(sys.modules, "insightface.app", module)

    engine = FaceEngine()
    with caplog.at_level(logging.INFO):
        engine.ensure_loaded()

    assert "face models loaded" in caplog.text
