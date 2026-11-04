"""Analysis endpoint tests.

``/faces/analyze`` detects and embeds every face of an image in one model pass,
so the caller never has to re-run detection per face (which made group photos
exceed the request budget and lose most of their faces).
"""

from __future__ import annotations

from app.engine import EMBEDDING_DIM


def test_analyze_returns_every_face_with_its_embedding(client, image) -> None:
    response = client.post("/faces/analyze", json={"image": image})

    assert response.status_code == 200
    faces = response.json()["faces"]
    assert len(faces) == 1

    face = faces[0]
    assert face["width"] == 30.0
    assert face["height"] == 32.0
    assert face["confidence"] == 0.99
    assert len(face["embedding"]) == EMBEDDING_DIM


def test_analyze_rejects_an_invalid_image(client) -> None:
    response = client.post("/faces/analyze", json={"image": "not-an-image"})

    assert response.status_code == 400


def test_analyze_of_an_image_without_faces_is_empty(client, monkeypatch) -> None:
    from app.main import create_app
    from fastapi.testclient import TestClient

    class NoFaceEngine:
        def analyze(self, image_b64: str) -> list[dict]:
            return []

    client = TestClient(create_app(engine=NoFaceEngine()))
    response = client.post("/faces/analyze", json={"image": "anything"})

    assert response.status_code == 200
    assert response.json()["faces"] == []
