"""Health and error-handling tests for the faces HTTP API."""

from __future__ import annotations

import requests

# Payload of "AAA" is a valid base64 alphabet but decodes to an unparsable image,
# and this one is not even valid base64 — both must be rejected with HTTP 400.
INVALID_IMAGE_PAYLOADS = ("not-an-image", "AAA")


def test_health_returns_ok(health_url: str) -> None:
    """GET /faces/health answers 200 with a stable body."""
    response = requests.get(health_url, timeout=10)
    assert response.status_code == 200, f"Expected 200, got {response.status_code}: {response.text}"
    assert response.json() == {"status": "ok"}


def test_detect_rejects_get(detect_url: str) -> None:
    """Detection is POST-only."""
    response = requests.get(detect_url, timeout=10)
    assert response.status_code == 405, f"Expected 405, got {response.status_code}: {response.text}"


def test_detect_rejects_missing_image(detect_url: str) -> None:
    """A body without the ``image`` field fails validation."""
    response = requests.post(detect_url, json={}, timeout=10)
    assert response.status_code == 422, f"Expected 422, got {response.status_code}: {response.text}"


def test_detect_rejects_invalid_image(detect_url: str, service) -> None:
    """An undecodable image payload is a client error, not a 500."""
    for payload in INVALID_IMAGE_PAYLOADS:
        response = requests.post(detect_url, json={"image": payload}, timeout=service.timeout)
        assert response.status_code == 400, (
            f"payload {payload!r}: expected 400, got {response.status_code}: {response.text}"
        )
        assert "detail" in response.json()


def test_embed_rejects_invalid_image(embed_url: str, service) -> None:
    """Embedding rejects the same broken payloads as detection."""
    for payload in INVALID_IMAGE_PAYLOADS:
        response = requests.post(embed_url, json={"image": payload}, timeout=service.timeout)
        assert response.status_code == 400, (
            f"payload {payload!r}: expected 400, got {response.status_code}: {response.text}"
        )


def test_cluster_with_no_embeddings_returns_no_labels(service) -> None:
    """Clustering an empty set is a no-op."""
    assert service.cluster([]) == []
