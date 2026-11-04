"""Pytest configuration for the faces integration suite.

Base URL resolution (priority order):

1. ``--base-url`` command line argument:
   ``pytest tests/faces/integration --base-url http://localhost:5173``
2. ``FACES_SERVICE_URL`` environment variable
3. Default: ``http://localhost:5173``

The suite requires a running faces service with its InsightFace models already
downloaded (``make run-faces``); the service performs the actual ML inference,
so nothing heavy runs inside pytest.
"""

from __future__ import annotations

import os
from pathlib import Path
from typing import Callable

import pytest

from support import (
    DEFAULT_CLUSTER_MIN_SIZE,
    DEFAULT_GROUP_COSINE,
    DEFAULT_IMPOSTOR_COSINE,
    DEFAULT_MIN_DETECTION_CONFIDENCE,
    DEFAULT_REQUEST_TIMEOUT,
    DEFAULT_SAME_PERSON_COSINE,
    FacesService,
)

DEFAULT_BASE_URL = "http://localhost:5173"


def pytest_addoption(parser: pytest.Parser) -> None:
    group = parser.getgroup("faces")
    group.addoption(
        "--base-url",
        action="store",
        default=None,
        help=f"Base URL of the faces service (default: $FACES_SERVICE_URL or {DEFAULT_BASE_URL})",
    )
    group.addoption(
        "--faces-request-timeout",
        action="store",
        default=None,
        help=f"Per-request timeout in seconds (default: $FACES_REQUEST_TIMEOUT or {DEFAULT_REQUEST_TIMEOUT})",
    )
    group.addoption(
        "--min-detection-confidence",
        action="store",
        default=None,
        help=(
            "Detection confidence a face must reach to be counted "
            f"(default: $FACES_MIN_DETECTION_CONFIDENCE or {DEFAULT_MIN_DETECTION_CONFIDENCE})"
        ),
    )
    group.addoption(
        "--same-person-cosine",
        action="store",
        default=None,
        help=(
            "Minimum cosine similarity for two photos of the same person "
            f"(default: $FACES_SAME_PERSON_COSINE or {DEFAULT_SAME_PERSON_COSINE})"
        ),
    )
    group.addoption(
        "--impostor-cosine",
        action="store",
        default=None,
        help=(
            "Maximum cosine similarity allowed between different people "
            f"(default: $FACES_IMPOSTOR_COSINE or {DEFAULT_IMPOSTOR_COSINE})"
        ),
    )
    group.addoption(
        "--group-cosine",
        action="store",
        default=None,
        help=(
            "Minimum cosine similarity for recognising a person inside a group photo "
            f"(default: $FACES_GROUP_COSINE or {DEFAULT_GROUP_COSINE})"
        ),
    )
    group.addoption(
        "--cluster-min-size",
        action="store",
        default=None,
        help=(
            "minClusterSize passed to /faces/cluster "
            f"(default: $FACES_CLUSTER_MIN_SIZE or {DEFAULT_CLUSTER_MIN_SIZE})"
        ),
    )


def _option(
    request: pytest.FixtureRequest,
    name: str,
    env: str,
    default,
    cast: Callable[[str], object],
):
    """Resolve an option/env/default triple for a threshold fixture."""
    value = request.config.getoption(name)
    if value is not None:
        return cast(value)
    env_value = os.environ.get(env)
    if env_value:
        return cast(env_value)
    return default


@pytest.fixture(scope="session")
def base_url(request: pytest.FixtureRequest) -> str:
    """Return the configured base URL of the faces service."""
    cli_value = request.config.getoption("--base-url")
    if cli_value:
        return str(cli_value).rstrip("/")

    env_value = os.environ.get("FACES_SERVICE_URL")
    if env_value:
        return env_value.rstrip("/")

    return DEFAULT_BASE_URL


@pytest.fixture(scope="session")
def api_url(base_url: str) -> str:
    """Return the faces API prefix (the service mounts every route under /faces)."""
    return f"{base_url}/faces"


@pytest.fixture(scope="session")
def health_url(api_url: str) -> str:
    return f"{api_url}/health"


@pytest.fixture(scope="session")
def detect_url(api_url: str) -> str:
    return f"{api_url}/detect"


@pytest.fixture(scope="session")
def embed_url(api_url: str) -> str:
    return f"{api_url}/embed"


@pytest.fixture(scope="session")
def cluster_url(api_url: str) -> str:
    return f"{api_url}/cluster"


@pytest.fixture(scope="session")
def dataset_dir() -> Path:
    """Return the shared test dataset directory (``tests/dataset``)."""
    # tests/faces/integration/conftest.py -> tests/dataset
    path = Path(__file__).resolve().parents[2] / "dataset"
    if not path.is_dir():
        pytest.fail(f"shared test dataset not found: {path}", pytrace=False)
    return path


@pytest.fixture(scope="session")
def request_timeout(request: pytest.FixtureRequest) -> float:
    return _option(request, "--faces-request-timeout", "FACES_REQUEST_TIMEOUT", DEFAULT_REQUEST_TIMEOUT, float)


@pytest.fixture(scope="session")
def min_detection_confidence(request: pytest.FixtureRequest) -> float:
    return _option(
        request,
        "--min-detection-confidence",
        "FACES_MIN_DETECTION_CONFIDENCE",
        DEFAULT_MIN_DETECTION_CONFIDENCE,
        float,
    )


@pytest.fixture(scope="session")
def same_person_cosine(request: pytest.FixtureRequest) -> float:
    return _option(
        request,
        "--same-person-cosine",
        "FACES_SAME_PERSON_COSINE",
        DEFAULT_SAME_PERSON_COSINE,
        float,
    )


@pytest.fixture(scope="session")
def impostor_cosine(request: pytest.FixtureRequest) -> float:
    return _option(
        request,
        "--impostor-cosine",
        "FACES_IMPOSTOR_COSINE",
        DEFAULT_IMPOSTOR_COSINE,
        float,
    )


@pytest.fixture(scope="session")
def group_cosine(request: pytest.FixtureRequest) -> float:
    return _option(
        request,
        "--group-cosine",
        "FACES_GROUP_COSINE",
        DEFAULT_GROUP_COSINE,
        float,
    )


@pytest.fixture(scope="session")
def cluster_min_size(request: pytest.FixtureRequest) -> int:
    return _option(
        request,
        "--cluster-min-size",
        "FACES_CLUSTER_MIN_SIZE",
        DEFAULT_CLUSTER_MIN_SIZE,
        int,
    )


@pytest.fixture(scope="session")
def service(api_url: str, dataset_dir: Path, request_timeout: float) -> FacesService:
    """Return the faces HTTP client, aborting early when the service is down."""
    client = FacesService(api_url=api_url, dataset_dir=dataset_dir, timeout=request_timeout)
    try:
        client.health()
    except Exception as exc:  # noqa: BLE001 - any transport error means "not up"
        pytest.exit(
            f"faces service is not reachable at {api_url} ({exc}). "
            "Start it with `make run-faces` (or `make dev-faces`).",
            returncode=2,
        )
    return client
