"""Face detection tests over the shared ``tests/dataset/faces`` photo set."""

from __future__ import annotations

import pytest

from support import (
    GROUP_PHOTOS,
    NO_FACE_IMAGE,
    PORTRAIT_FILES,
    all_face_photos,
    describe_boxes,
    image_dimensions,
    repository_dataset_dir,
)

# SCRFD occasionally returns a box a few pixels outside the image border; allow
# that much slack instead of asserting a box is exactly inside the frame.
OUT_OF_BOUNDS_TOLERANCE = 8.0

ALL_FACE_PHOTOS = all_face_photos(repository_dataset_dir())


@pytest.mark.parametrize("photo", PORTRAIT_FILES)
def test_portrait_contains_a_confident_face(service, min_detection_confidence, photo) -> None:
    """Every single-person portrait yields at least one confident face."""
    boxes = service.detect_file(photo)
    assert boxes, f"{photo}: no face detected (expected at least one)"

    best = max(box["confidence"] for box in boxes)
    assert best >= min_detection_confidence, (
        f"{photo}: best confidence {best:.3f} < {min_detection_confidence:.2f}; "
        f"detections: {describe_boxes(boxes)}"
    )


@pytest.mark.parametrize("photo,min_faces", sorted(GROUP_PHOTOS.items()))
def test_group_photo_contains_several_faces(
    service, min_detection_confidence, photo, min_faces
) -> None:
    """Group photos expose several faces above the confidence threshold."""
    boxes = service.detect_file(photo)
    confident = [box for box in boxes if box["confidence"] >= min_detection_confidence]
    assert len(confident) >= min_faces, (
        f"{photo}: {len(confident)} confident face(s) < expected {min_faces}; "
        f"all detections: {describe_boxes(boxes)}"
    )


@pytest.mark.parametrize("photo", sorted(GROUP_PHOTOS) + list(PORTRAIT_FILES))
def test_boxes_are_geometrically_valid(service, min_detection_confidence, photo) -> None:
    """Every confident box has a positive size and lies inside the image frame."""
    width, height = image_dimensions(service.dataset_dir, photo)
    for box in service.detect_file(photo):
        if box["confidence"] < min_detection_confidence:
            continue
        assert box["width"] > 0 and box["height"] > 0, f"{photo}: degenerate box {box}"
        assert -OUT_OF_BOUNDS_TOLERANCE <= box["x"] <= width + OUT_OF_BOUNDS_TOLERANCE, (
            f"{photo}: box x={box['x']} outside image width {width}"
        )
        assert -OUT_OF_BOUNDS_TOLERANCE <= box["y"] <= height + OUT_OF_BOUNDS_TOLERANCE, (
            f"{photo}: box y={box['y']} outside image height {height}"
        )
        assert box["x"] + box["width"] <= width + OUT_OF_BOUNDS_TOLERANCE, (
            f"{photo}: box right edge {box['x'] + box['width']} outside image width {width}"
        )
        assert box["y"] + box["height"] <= height + OUT_OF_BOUNDS_TOLERANCE, (
            f"{photo}: box bottom edge {box['y'] + box['height']} outside image height {height}"
        )


def test_image_without_faces_has_no_detections(service, min_detection_confidence) -> None:
    """The control image from the shared dataset must stay face-free."""
    boxes = service.detect_file(NO_FACE_IMAGE)
    confident = [box for box in boxes if box["confidence"] >= min_detection_confidence]
    assert not confident, (
        f"{NO_FACE_IMAGE}: unexpected faces above {min_detection_confidence:.2f}: "
        f"{describe_boxes(confident)}"
    )


@pytest.mark.parametrize("photo", ALL_FACE_PHOTOS)
def test_every_dataset_photo_returns_a_valid_response(service, photo) -> None:
    """The whole dataset is processable and confidence values stay in range."""
    boxes = service.detect_file(photo)
    for box in boxes:
        assert 0.0 <= box["confidence"] <= 1.0, f"{photo}: confidence out of range {box}"
        assert set(box) >= {"x", "y", "width", "height", "confidence"}, (
            f"{photo}: unexpected box shape {sorted(box)}"
        )
