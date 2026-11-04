"""Batch analysis endpoint tests.

``POST /faces/analyze-batch`` returns exactly one entry per submitted image, in
request order, and contains a per-image failure to that entry so one unreadable
photo cannot stop a scan batch.
"""

from __future__ import annotations

import base64
import io

from conftest import FakeEngine
from fastapi.testclient import TestClient
from PIL import Image

from app.config import FaceSettings, resolve_providers
from app.engine import EMBEDDING_DIM, FaceEngine
from app.main import create_app


def _image(color: tuple[int, int, int]) -> str:
    img = Image.new("RGB", (64, 64), color=color)
    buf = io.BytesIO()
    img.save(buf, format="JPEG")
    return base64.b64encode(buf.getvalue()).decode()


FACE = {
    "x": 10.0,
    "y": 12.0,
    "width": 30.0,
    "height": 32.0,
    "confidence": 0.99,
    "embedding": [0.01] * EMBEDDING_DIM,
}


class BatchEngine:
    """Fake engine returning a configurable number of faces per image."""

    def __init__(self, faces_per_image: dict[str, int] | None = None) -> None:
        self.faces_per_image = faces_per_image or {}
        self._selection = resolve_providers(("cpu",), ("CPUExecutionProvider",))

    @property
    def settings(self) -> FaceSettings:
        return FaceSettings()

    def provider_selection(self):
        return self._selection

    def analyze(self, image_b64: str) -> list[dict]:
        # Prove the payload decodes as a real image, like the real engine does.
        FaceEngine.decode_image(image_b64)
        return [dict(FACE) for _ in range(self.faces_per_image.get(image_b64, 1))]


def _client(engine, max_batch: int = 8) -> TestClient:
    return TestClient(
        create_app(engine=engine, settings=FaceSettings(max_batch=max_batch))
    )


def test_batch_returns_entry_per_image() -> None:
    first, second, third = _image((10, 0, 0)), _image((0, 10, 0)), _image((0, 0, 10))
    engine = BatchEngine({second: 2})

    response = _client(engine).post(
        "/faces/analyze-batch", json={"images": [first, second, third]}
    )

    assert response.status_code == 200
    results = response.json()["results"]
    assert len(results) == 3
    # Positional alignment: the second entry is the second image.
    assert len(results[0]["faces"]) == 1
    assert len(results[1]["faces"]) == 2
    assert len(results[2]["faces"]) == 1


def test_undecodable_image_isolated() -> None:
    first, broken, third = (
        _image((10, 0, 0)),
        "not-an-image",
        _image((0, 0, 10)),
    )

    response = _client(BatchEngine()).post(
        "/faces/analyze-batch", json={"images": [first, broken, third]}
    )

    assert response.status_code == 200
    results = response.json()["results"]
    assert len(results) == 3
    assert results[0]["error"] is None and len(results[0]["faces"]) == 1
    assert results[1]["faces"] == []
    assert "invalid image payload" in results[1]["error"]
    assert results[2]["error"] is None and len(results[2]["faces"]) == 1


def test_batch_over_the_cap_is_rejected() -> None:
    images = [_image((10, 0, 0)) for _ in range(3)]

    response = _client(BatchEngine(), max_batch=2).post(
        "/faces/analyze-batch", json={"images": images}
    )

    assert response.status_code == 422
    assert "FACES_MAX_BATCH" in response.json()["detail"]


def test_single_image_endpoint_still_works() -> None:
    """``/faces/analyze`` stays the single-image call next to the batch one."""
    response = TestClient(create_app(engine=FakeEngine())).post(
        "/faces/analyze", json={"image": _image((1, 1, 1))}
    )

    assert response.status_code == 200
    assert len(response.json()["faces"]) == 1
