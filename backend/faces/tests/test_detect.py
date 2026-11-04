"""Detection endpoint tests."""

from __future__ import annotations


def test_detect_returns_at_least_one_box(client, image) -> None:
    response = client.post("/faces/detect", json={"image": image})
    assert response.status_code == 200
    boxes = response.json()["boxes"]
    assert len(boxes) >= 1
    for box in boxes:
        assert set(box) >= {"x", "y", "width", "height", "confidence"}
