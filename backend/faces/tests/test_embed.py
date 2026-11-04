"""Embedding endpoint tests."""

from __future__ import annotations

from app.engine import EMBEDDING_DIM


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
