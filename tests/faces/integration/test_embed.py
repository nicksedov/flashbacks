"""Embedding tests: shape, normalisation, determinism and box/full-image parity."""

from __future__ import annotations

import math

import pytest

from support import (
    BOHR_PORTRAIT,
    EMBEDDING_DIMENSION,
    OBAMA_FRONT,
    PORTRAIT_FILES,
    cosine,
)


@pytest.mark.parametrize("photo", PORTRAIT_FILES)
def test_portrait_embedding_is_a_normalised_vector(service, photo) -> None:
    """ArcFace returns a 512-D, finite, L2-normalised vector."""
    embedding = service.portrait_embedding(photo)
    assert embedding, f"{photo}: no embedding — no face was detected"

    assert len(embedding) == EMBEDDING_DIMENSION, (
        f"{photo}: embedding has {len(embedding)} dimensions, expected {EMBEDDING_DIMENSION}"
    )
    assert all(math.isfinite(value) for value in embedding), f"{photo}: embedding has non-finite values"

    norm = math.sqrt(math.fsum(value * value for value in embedding))
    assert abs(norm - 1.0) < 1e-3, f"{photo}: embedding norm {norm:.4f} is not unit length"


def test_embedding_of_the_same_photo_is_stable(service) -> None:
    """Two independent calls on the same bytes produce the same vector."""
    image = service.image(OBAMA_FRONT)
    first = service.embed_raw(image)
    second = service.embed_raw(image)

    assert len(first) == len(second) == EMBEDDING_DIMENSION
    assert cosine(first, second) > 1.0 - 1e-6, (
        f"embedding is not deterministic: cosine {cosine(first, second):.8f}"
    )


def test_box_embedding_matches_full_image_embedding(service) -> None:
    """Cropping to the detected face keeps the identity signal intact."""
    full_image = service.embed_file(OBAMA_FRONT)
    cropped = service.portrait_embedding(OBAMA_FRONT)

    assert cropped, f"{OBAMA_FRONT}: no face detected"
    similarity = cosine(full_image, cropped)
    assert similarity >= 0.99, (
        f"{OBAMA_FRONT}: box embedding differs from the full-image embedding "
        f"(cosine {similarity:.4f})"
    )


def test_different_people_produce_different_vectors(service) -> None:
    """Two unrelated portraits must not collapse to the same embedding."""
    obama = service.portrait_embedding(OBAMA_FRONT)
    bohr = service.portrait_embedding(BOHR_PORTRAIT)

    assert obama and bohr, "missing embedding for a portrait"
    similarity = cosine(obama, bohr)
    assert similarity < 0.9, (
        f"{OBAMA_FRONT} vs {BOHR_PORTRAIT}: cosine {similarity:.4f} is suspiciously high"
    )
