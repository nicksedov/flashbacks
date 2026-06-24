"""
Pytest configuration for OCR integration tests.

Base URL configuration (priority order):
1. --base-url CLI argument:  pytest integration/ --base-url http://localhost:5174
2. OCR_CLASSIFIER_URL environment variable
3. Default: http://ocr:5174
"""

import os
import pytest


def pytest_addoption(parser):
    parser.addoption(
        "--base-url",
        action="store",
        default=None,
        help="Base URL of the OCR service "
             "(default: $OCR_CLASSIFIER_URL or http://ocr:5174)",
    )


@pytest.fixture(scope="session")
def base_url(request):
    """Return the configured base URL for the OCR Classifier service."""
    cli_value = request.config.getoption("--base-url")
    if cli_value:
        return cli_value.rstrip("/")

    env_value = os.environ.get("OCR_CLASSIFIER_URL")
    if env_value:
        return env_value.rstrip("/")

    return "http://ocr:5174"


@pytest.fixture(scope="session")
def api_url(base_url):
    """Return the API prefix URL."""
    return f"{base_url}/ocr/api"


@pytest.fixture(scope="session")
def health_url(api_url):
    """Return the health check endpoint URL."""
    return f"{api_url}/health"


@pytest.fixture(scope="session")
def classify_url(api_url):
    """Return the classify endpoint URL."""
    return f"{api_url}/v1/classify"


@pytest.fixture(scope="session")
def dataset_dir():
    """Return the dataset directory path relative to project root."""
    # When running tests from the project root
    return os.path.join(os.path.dirname(os.path.dirname(__file__)), "dataset")
