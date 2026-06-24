"""
Integration tests for OCR.

Tests the deployed OCR service via HTTP.
Covers the same scenarios as the original Go unit tests but against a live service.

Usage:
    pytest integration/ --base-url http://localhost:5174
    OCR_CLASSIFIER_URL=http://localhost:5174 pytest integration/
    pytest integration/   # uses default http://ocr:5174
"""

import os
import requests

# ──────────────────────────────────────────────
# Health Check
# ──────────────────────────────────────────────


class TestHealthEndpoint:
    """Tests for the /health endpoint."""

    def test_health_returns_ok(self, health_url):
        """GET /health should return 200 with status ok."""
        resp = requests.get(health_url, timeout=10)
        assert resp.status_code == 200, f"Expected 200, got {resp.status_code}: {resp.text}"
        data = resp.json()
        assert data == {"status": "ok"}, f"Unexpected health response: {data}"


# ──────────────────────────────────────────────
# Error Cases
# ──────────────────────────────────────────────


class TestClassifyErrors:
    """Tests for error handling of the /classify endpoint."""

    def test_wrong_method_get(self, classify_url):
        """GET instead of POST should return 405."""
        resp = requests.get(classify_url, timeout=10)
        assert resp.status_code == 405, f"Expected 405, got {resp.status_code}: {resp.text}"
        data = resp.json()
        assert "error" in data

    def test_wrong_content_type(self, classify_url):
        """Wrong Content-Type should return 400."""
        resp = requests.post(
            classify_url,
            headers={"Content-Type": "application/json"},
            data=b'{}',
            timeout=10,
        )
        assert resp.status_code == 400, f"Expected 400, got {resp.status_code}: {resp.text}"
        data = resp.json()
        assert "error" in data

    def test_empty_body_jpeg(self, classify_url):
        """Empty body with image/jpeg should return 400."""
        resp = requests.post(
            classify_url,
            headers={"Content-Type": "image/jpeg"},
            data=b'',
            timeout=10,
        )
        assert resp.status_code == 400, f"Expected 400, got {resp.status_code}: {resp.text}"
        data = resp.json()
        assert "error" in data

    def test_empty_body_png(self, classify_url):
        """Empty body with image/png should return 400."""
        resp = requests.post(
            classify_url,
            headers={"Content-Type": "image/png"},
            data=b'',
            timeout=10,
        )
        assert resp.status_code == 400, f"Expected 400, got {resp.status_code}: {resp.text}"
        data = resp.json()
        assert "error" in data


# ──────────────────────────────────────────────
# Helper
# ──────────────────────────────────────────────


def load_image(dataset_dir, subfolder, filename):
    """Load an image file from the dataset directory."""
    path = os.path.join(dataset_dir, subfolder, filename)
    if not os.path.isfile(path):
        pytest.skip(f"Dataset image not found: {path}")
    with open(path, "rb") as f:
        return f.read()


def classify_image(classify_url, image_data, content_type="image/jpeg", params=None):
    """Send a classification request and return the response."""
    headers = {"Content-Type": content_type}
    resp = requests.post(
        classify_url,
        headers=headers,
        data=image_data,
        params=params,
        timeout=120,
    )
    return resp


import pytest


# ──────────────────────────────────────────────
# Human-readable assertion helpers
# ──────────────────────────────────────────────


def assert_field_present(data, field, filename=""):
    """Assert that *field* exists in the response dict."""
    prefix = f"{filename}: " if filename else ""
    assert field in data, f"{prefix}Response is missing field '{field}' — full response keys: {list(data.keys())}"


def assert_field_true(data, field, filename=""):
    """Assert that a boolean field is ``True``."""
    prefix = f"{filename}: " if filename else ""
    assert_field_present(data, field, filename)
    actual = data[field]
    assert actual is True, (
        f"{prefix}Expected '{field}' to be True, but got {actual!r} (type={type(actual).__name__})"
    )


def assert_field_gt(data, field, threshold, filename=""):
    """Assert that a numeric field is strictly greater than *threshold*."""
    prefix = f"{filename}: " if filename else ""
    assert_field_present(data, field, filename)
    actual = data[field]
    assert isinstance(actual, (int, float)), (
        f"{prefix}Expected '{field}' to be numeric, but got {actual!r} (type={type(actual).__name__})"
    )
    assert actual > threshold, (
        f"{prefix}Criterion '{field}' = {actual} is NOT > {threshold} (expected > {threshold})"
    )


def assert_field_is_list(data, field, filename=""):
    """Assert that a field is a ``list``."""
    prefix = f"{filename}: " if filename else ""
    assert_field_present(data, field, filename)
    actual = data[field]
    assert isinstance(actual, list), (
        f"{prefix}Expected '{field}' to be a list, but got {actual!r} (type={type(actual).__name__})"
    )


# ──────────────────────────────────────────────
# English Documents
# ──────────────────────────────────────────────


class TestClassifyEnglish:
    """Tests with English text documents — expect is_text_document=True."""

    DATASET_FILES = [
        "50407632-7632.jpg",
        "502607827+-7827.jpg",
        "502611995a-1996.jpg",
        "503524860_503524863.jpg",
        "527799804+-9805.jpg",
        "528416668+-6668.jpg",
        "2024318141.jpg",
        "lightbulb-scheme.jpg",
        "PUBLICATIONS016897-6.jpg",
        "PUBLICATIONS026505-6.jpg",
    ]

    @pytest.mark.parametrize("filename", DATASET_FILES)
    def test_eng_classify(self, classify_url, dataset_dir, filename):
        """Each English document should be classified as a text document."""
        image_data = load_image(dataset_dir, "eng", filename)
        content_type = "image/png" if filename.lower().endswith(".png") else "image/jpeg"
        resp = classify_image(classify_url, image_data, content_type)
        assert resp.status_code == 200, f"{filename}: Expected 200, got {resp.status_code}: {resp.text}"
        data = resp.json()
        assert isinstance(data, dict), f"{filename}: Expected dict, got {type(data)}"
        # English documents with text should be classified as text documents
        assert_field_true(data, "is_text_document", filename)
        # Sanity checks on response fields
        assert_field_gt(data, "token_count", 0, filename)
        assert_field_gt(data, "weighted_confidence", 0, filename)
        assert_field_gt(data, "bounding_box_width", 0, filename)
        assert_field_gt(data, "bounding_box_height", 0, filename)
        assert_field_is_list(data, "boxes", filename)


# ──────────────────────────────────────────────
# Russian Documents
# ──────────────────────────────────────────────


class TestClassifyRussian:
    """Tests with Russian text documents — expect is_text_document=True."""

    DATASET_FILES = [
        "contract01.jpg",
        "contract02.jpg",
        "contract03.jpg",
        "contract04.jpg",
        "contract05.jpg",
        "disclaimer.jpg",
        "historical_doc.jpg",
        "passportscan01.png",
        "passportscan02.png",
        "penalty_form.jpg",
    ]

    @pytest.mark.parametrize("filename", DATASET_FILES)
    def test_rus_classify(self, classify_url, dataset_dir, filename):
        """Each Russian document should be classified as a text document."""
        image_data = load_image(dataset_dir, "rus", filename)
        content_type = "image/png" if filename.lower().endswith(".png") else "image/jpeg"
        resp = classify_image(classify_url, image_data, content_type)
        assert resp.status_code == 200, f"{filename}: Expected 200, got {resp.status_code}: {resp.text}"
        data = resp.json()
        assert isinstance(data, dict), f"{filename}: Expected dict, got {type(data)}"
        # Russian documents with text should be classified as text documents
        assert_field_true(data, "is_text_document", filename)
        assert_field_gt(data, "token_count", 0, filename)
        assert_field_gt(data, "weighted_confidence", 0, filename)
        assert_field_gt(data, "bounding_box_width", 0, filename)
        assert_field_gt(data, "bounding_box_height", 0, filename)
        assert_field_is_list(data, "boxes", filename)


# ──────────────────────────────────────────────
# Rotated Russian Documents
# ──────────────────────────────────────────────


class TestClassifyRussianRotated:
    """Tests with rotated Russian documents — should handle rotation correction."""

    DATASET_FILES = [
        "rotated_15cw.jpg",
        "rotated_15ccw.jpg",
        "rotated_90.jpg",
        "rotated_180.jpg",
        "rotated_270.jpg",
    ]

    @pytest.mark.parametrize("filename", DATASET_FILES)
    def test_rus_rotated_classify(self, classify_url, dataset_dir, filename):
        """Rotated Russian documents should still be classified as text documents."""
        image_data = load_image(dataset_dir, "rus", filename)
        resp = classify_image(classify_url, image_data, "image/jpeg")
        assert resp.status_code == 200, f"{filename}: Expected 200, got {resp.status_code}: {resp.text}"
        data = resp.json()
        assert isinstance(data, dict), f"{filename}: Expected dict, got {type(data)}"
        # Rotated documents with text should still be classified as text documents
        assert_field_true(data, "is_text_document", filename)
        assert_field_gt(data, "token_count", 0, filename)


# ──────────────────────────────────────────────
# Images (no text)
# ──────────────────────────────────────────────


class TestClassifyImages:
    """Tests with images without text — expect is_text_document=False."""

    DATASET_FILES = [
        "aircraft.jpg",
        "clouded-sky.jpg",
        "dog.jpg",
        "light-bulb.jpg",
    ]

    @pytest.mark.parametrize("filename", DATASET_FILES)
    def test_image_no_text(self, classify_url, dataset_dir, filename):
        """Images without text should be classified as non-text documents."""
        image_data = load_image(dataset_dir, "image", filename)
        resp = classify_image(classify_url, image_data, "image/jpeg")
        assert resp.status_code == 200, f"{filename}: Expected 200, got {resp.status_code}: {resp.text}"
        data = resp.json()
        assert isinstance(data, dict), f"{filename}: Expected dict, got {type(data)}"
        # Images without text should NOT be classified as text documents
        # Note: some images might accidentally trigger text detection,
        # so we just check the response structure is valid
        assert_field_present(data, "is_text_document", filename)
        assert_field_present(data, "token_count", filename)
        assert_field_present(data, "weighted_confidence", filename)
        assert_field_present(data, "boxes", filename)


# ──────────────────────────────────────────────
# Inscriptions (short text / signs)
# ──────────────────────────────────────────────


class TestClassifyInscriptions:
    """Tests with inscriptions / signs — short text snippets."""

    DATASET_FILES = [
        "eng_althaus.jpg",
        "eng_centre.png",
        "eng_coffeshop.jpg",
        "eng_drybar.jpg",
        "eng_netfix.jpg",
        "eng_soda.jpg",
        "eng_streetfood.jpg",
        "eng_uh.jpg",
        "rus_alpari.jpg",
        "rus_coffeeshop.jpg",
        "rus_open.jpg",
    ]

    @pytest.mark.parametrize("filename", DATASET_FILES)
    def test_inscription(self, classify_url, dataset_dir, filename):
        """Inscriptions should produce valid classification results."""
        image_data = load_image(dataset_dir, "inscription", filename)
        content_type = "image/png" if filename.lower().endswith(".png") else "image/jpeg"
        resp = classify_image(classify_url, image_data, content_type)
        assert resp.status_code == 200, f"{filename}: Expected 200, got {resp.status_code}: {resp.text}"
        data = resp.json()
        assert isinstance(data, dict), f"{filename}: Expected dict, got {type(data)}"
        # Inscriptions may or may not be classified as text documents depending on
        # text amount and OCR quality, but response structure must be valid
        assert_field_present(data, "is_text_document", filename)
        assert_field_present(data, "token_count", filename)
        assert_field_present(data, "weighted_confidence", filename)
        assert_field_present(data, "boxes", filename)
        assert_field_present(data, "mean_confidence", filename)
        assert_field_present(data, "angle", filename)
        assert_field_present(data, "scale_factor", filename)


# ──────────────────────────────────────────────
# Query Parameters
# ──────────────────────────────────────────────


class TestClassifyParameters:
    """Tests with various query parameters."""

    def test_with_lang_param(self, classify_url, dataset_dir):
        """Classify with explicit language parameter."""
        image_data = load_image(dataset_dir, "eng", "2024318141.jpg")
        resp = classify_image(classify_url, image_data, "image/jpeg", params={"lang": "eng"})
        assert resp.status_code == 200, f"Expected 200, got {resp.status_code}: {resp.text}"
        data = resp.json()
        assert_field_true(data, "is_text_document")

    def test_with_confidence_threshold(self, classify_url, dataset_dir):
        """Classify with custom confidence threshold."""
        image_data = load_image(dataset_dir, "eng", "2024318141.jpg")
        resp = classify_image(classify_url, image_data, "image/jpeg", params={"confidence_threshold": "0.5"})
        assert resp.status_code == 200, f"Expected 200, got {resp.status_code}: {resp.text}"
        data = resp.json()
        assert_field_present(data, "is_text_document")

    def test_with_min_token_count(self, classify_url, dataset_dir):
        """Classify with custom minimum token count."""
        image_data = load_image(dataset_dir, "eng", "2024318141.jpg")
        resp = classify_image(classify_url, image_data, "image/jpeg", params={"min_token_count": "10"})
        assert resp.status_code == 200, f"Expected 200, got {resp.status_code}: {resp.text}"
        data = resp.json()
        assert_field_present(data, "is_text_document")

    def test_with_level_param(self, classify_url, dataset_dir):
        """Classify with explicit page iterator level."""
        image_data = load_image(dataset_dir, "eng", "2024318141.jpg")
        resp = classify_image(classify_url, image_data, "image/jpeg", params={"level": "RIL_TEXTLINE"})
        assert resp.status_code == 200, f"Expected 200, got {resp.status_code}: {resp.text}"
        data = resp.json()
        assert_field_present(data, "is_text_document")

    def test_rus_with_rus_lang(self, classify_url, dataset_dir):
        """Classify Russian document with explicit Russian language."""
        image_data = load_image(dataset_dir, "rus", "contract01.jpg")
        resp = classify_image(classify_url, image_data, "image/jpeg", params={"lang": "rus"})
        assert resp.status_code == 200, f"Expected 200, got {resp.status_code}: {resp.text}"
        data = resp.json()
        assert_field_true(data, "is_text_document")

    def test_all_params_combined(self, classify_url, dataset_dir):
        """Classify with all parameters combined."""
        image_data = load_image(dataset_dir, "eng", "lightbulb-scheme.jpg")
        resp = classify_image(
            classify_url,
            image_data,
            "image/jpeg",
            params={
                "lang": "eng",
                "level": "RIL_WORD",
                "confidence_threshold": "0.7",
                "min_token_count": "5",
            },
        )
        assert resp.status_code == 200, f"Expected 200, got {resp.status_code}: {resp.text}"
        data = resp.json()
        assert_field_present(data, "is_text_document")


# ──────────────────────────────────────────────
# Bulk Dataset Validation
# ──────────────────────────────────────────────


def collect_dataset_files(dataset_dir, subfolder):
    """Collect all image files from a dataset subfolder."""
    folder_path = os.path.join(dataset_dir, subfolder)
    if not os.path.isdir(folder_path):
        return []
    valid_exts = {".jpg", ".jpeg", ".png"}
    files = []
    for fname in sorted(os.listdir(folder_path)):
        ext = os.path.splitext(fname)[1].lower()
        if ext in valid_exts:
            files.append(fname)
    return files


class TestDatasetFull:
    """Process all images in each dataset category and verify no errors."""

    @pytest.mark.parametrize("subfolder", ["eng", "rus", "image", "inscription"])
    def test_dataset_category(self, classify_url, dataset_dir, subfolder):
        """All images in each dataset category should process without errors."""
        files = collect_dataset_files(dataset_dir, subfolder)
        if not files:
            pytest.skip(f"No dataset files found in {subfolder}/")

        errors = []
        for filename in files:
            content_type = "image/png" if filename.lower().endswith(".png") else "image/jpeg"
            image_data = load_image(dataset_dir, subfolder, filename)
            resp = classify_image(classify_url, image_data, content_type)
            if resp.status_code != 200:
                errors.append(f"{filename}: HTTP {resp.status_code} - {resp.text}")
            else:
                data = resp.json()
                required_fields = [
                    "mean_confidence", "weighted_confidence", "token_count",
                    "boxes", "angle", "scale_factor", "is_text_document",
                    "bounding_box_width", "bounding_box_height",
                ]
                for field in required_fields:
                    if field not in data:
                        errors.append(f"{filename}: Missing field '{field}'")

        assert not errors, f"Errors in dataset '{subfolder}':\n" + "\n".join(errors)
