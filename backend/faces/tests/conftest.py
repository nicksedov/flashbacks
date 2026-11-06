"""Shared fixtures for the faces service tests."""

from __future__ import annotations

import base64
import io

import pytest
from fastapi.testclient import TestClient

from app.config import resolve_providers
from app.engine import EMBEDDING_DIM, FaceEngine
from app.main import create_app


def make_image_b64() -> str:
    from PIL import Image

    img = Image.new("RGB", (64, 64), color=(120, 80, 60))
    buf = io.BytesIO()
    img.save(buf, format="JPEG")
    return base64.b64encode(buf.getvalue()).decode()


class FakeEngine:
    """Deterministic engine that exercises the request/response plumbing."""

    def __init__(
        self,
        providers: tuple[str, ...] = ("cpu",),
        available: tuple[str, ...] = ("CPUExecutionProvider",),
    ) -> None:
        self._selection = resolve_providers(providers, available)

    def provider_selection(self):
        return self._selection

    def device_info(self):
        return [
            {
                "deviceId": self._selection.device_id,
                "provider": self._selection.active_provider,
            }
        ]

    def detect(self, image_b64: str) -> list[dict[str, float]]:
        # Prove the payload decodes as a real image.
        FaceEngine.decode_image(image_b64)
        return [
            {"x": 10.0, "y": 12.0, "width": 30.0, "height": 32.0, "confidence": 0.99}
        ]

    def embed(self, image_b64: str, box: dict[str, float] | None = None) -> list[float]:
        FaceEngine.decode_image(image_b64)
        return [0.01] * EMBEDDING_DIM

    def analyze(self, image_b64: str) -> list[dict]:
        FaceEngine.decode_image(image_b64)
        return [
            {
                "x": 10.0,
                "y": 12.0,
                "width": 30.0,
                "height": 32.0,
                "confidence": 0.99,
                "embedding": [0.01] * EMBEDDING_DIM,
            }
        ]

    def cluster(
        self,
        embeddings: list[list[float]],
        min_cluster_size: int = 2,
        solo_faces: list[bool] | None = None,
        photo_ids: list[int] | None = None,
        widths: list[float] | None = None,
        min_clusterable_px: float | None = None,
    ) -> list[int]:
        return FaceEngine.cluster(
            embeddings,
            min_cluster_size,
            solo_faces,
            photo_ids,
            widths,
            min_clusterable_px,
        )

    def cluster_graph(
        self,
        face_ids: list[int],
        edges: list[tuple[int, int, float]],
        solo_faces: list[bool] | None = None,
        min_cluster_size: int = 2,
        **kwargs,
    ) -> list[int]:
        return FaceEngine.cluster_graph(
            face_ids, edges, solo_faces, min_cluster_size, **kwargs
        )


@pytest.fixture()
def client() -> TestClient:
    app = create_app(engine=FakeEngine())
    return TestClient(app)


@pytest.fixture()
def image() -> str:
    return make_image_b64()
