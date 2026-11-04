"""Shared fixtures for the faces service tests."""

from __future__ import annotations

import base64
import io

import pytest
from fastapi.testclient import TestClient

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

    def detect(self, image_b64: str) -> list[dict[str, float]]:
        # Prove the payload decodes as a real image.
        FaceEngine.decode_image(image_b64)
        return [
            {"x": 10.0, "y": 12.0, "width": 30.0, "height": 32.0, "confidence": 0.99}
        ]

    def embed(self, image_b64: str, box: dict[str, float] | None = None) -> list[float]:
        FaceEngine.decode_image(image_b64)
        return [0.01] * EMBEDDING_DIM

    def cluster(self, embeddings: list[list[float]], min_cluster_size: int = 5) -> list[int]:
        return FaceEngine.cluster(embeddings, min_cluster_size)


@pytest.fixture()
def client() -> TestClient:
    app = create_app(engine=FakeEngine())
    return TestClient(app)


@pytest.fixture()
def image() -> str:
    return make_image_b64()
