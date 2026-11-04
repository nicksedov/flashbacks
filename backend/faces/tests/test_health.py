"""Health endpoint tests."""

from __future__ import annotations


def test_health_returns_ok(client) -> None:
    response = client.get("/faces/health")
    assert response.status_code == 200
    assert response.json() == {"status": "ok"}
