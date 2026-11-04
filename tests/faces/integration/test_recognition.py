"""Identity recognition tests over the shared ``tests/dataset/faces`` photo set.

Covered scenarios (mirroring the coverage tables in the dataset READMEs):

- the same person photographed twice is recognised;
- different people are not confused;
- a person documented in a group photo is found inside that photo, using the
  individual portraits as the gallery;
- a two-person photo matches exactly one of the two people.
"""

from __future__ import annotations

import pytest

from support import (
    GROUP_IDENTITY_PAIRS,
    IMPOSTOR_PAIRS,
    MERKEL_PORTRAIT,
    OBAMA_BIDEN,
    OBAMA_FRONT,
    OBAMA_PROFILE,
    SAME_PERSON_PAIRS,
    best_match,
    cosine,
)


@pytest.mark.parametrize("first,second,pair_bound", SAME_PERSON_PAIRS)
def test_same_person_photos_are_recognised(
    service, same_person_cosine, first, second, pair_bound
) -> None:
    """Two photos of one person must exceed the same-person cosine bound.

    ``pair_bound`` carries the per-pair expectation (older/newer Einstein gets a
    looser bound); ``--same-person-cosine`` can relax it further but never
    tighten it.
    """
    threshold = min(pair_bound, same_person_cosine)
    first_embedding = service.portrait_embedding(first)
    second_embedding = service.portrait_embedding(second)
    assert first_embedding and second_embedding, f"missing embedding: {first} / {second}"

    similarity = cosine(first_embedding, second_embedding)
    assert similarity >= threshold, (
        f"{first} vs {second}: cosine {similarity:.3f} < {threshold:.2f} — "
        "the same person was not recognised"
    )


@pytest.mark.parametrize("first,second", IMPOSTOR_PAIRS)
def test_different_people_are_not_confused(service, impostor_cosine, first, second) -> None:
    """Portraits of different people stay below the impostor bound."""
    first_embedding = service.portrait_embedding(first)
    second_embedding = service.portrait_embedding(second)
    assert first_embedding and second_embedding, f"missing embedding: {first} / {second}"

    similarity = cosine(first_embedding, second_embedding)
    assert similarity < impostor_cosine, (
        f"{first} vs {second}: cosine {similarity:.3f} >= {impostor_cosine:.2f} — "
        "different people look too similar to the model"
    )


@pytest.mark.parametrize("group_photo,identity,pair_bound", GROUP_IDENTITY_PAIRS)
def test_person_is_recognised_inside_a_group_photo(
    service, group_cosine, group_photo, identity, pair_bound
) -> None:
    """At least one face of the group photo matches the identity's portraits.

    ``pair_bound`` carries the per-pair expectation (a tiny face in a wide group
    photo gets a looser bound); ``--group-cosine`` can relax it further but never
    tighten it.
    """
    threshold = min(pair_bound, group_cosine)
    references = service.identity_references(identity)
    assert references, f"{identity}: no gallery embedding — the portrait photo yielded no face"

    faces = service.face_embeddings(group_photo)
    assert faces, f"{group_photo}: no faces detected"

    scored = [best_match(embedding, references) for _, embedding in faces]
    best_similarity = max(scored)
    assert best_similarity >= threshold, (
        f"{group_photo}: {identity} was not found among {len(scored)} face(s) — "
        f"best cosine {best_similarity:.3f} < {threshold:.2f}; "
        f"all similarities: {[round(value, 3) for value in scored]}"
    )


def test_two_person_group_matches_only_the_expected_person(
    service, group_cosine, impostor_cosine
) -> None:
    """``obama_biden.jpg`` shows Obama and Biden: exactly one face is Obama."""
    references = service.identity_references("obama")
    assert references, "obama: no gallery embedding"

    faces = service.face_embeddings(OBAMA_BIDEN)
    assert len(faces) >= 2, f"{OBAMA_BIDEN}: expected at least two faces, got {len(faces)}"

    similarities = [best_match(embedding, references) for _, embedding in faces]
    matches = [value for value in similarities if value >= group_cosine]
    assert len(matches) == 1, (
        f"{OBAMA_BIDEN}: expected exactly one Obama match, got {len(matches)} "
        f"(impostor bound {impostor_cosine:.2f}); similarities: {[round(v, 3) for v in similarities]}"
    )


def test_same_person_is_closer_than_an_impostor(service) -> None:
    """The same-person signal must dominate an unrelated portrait."""
    obama_front = service.portrait_embedding(OBAMA_FRONT)
    obama_profile = service.portrait_embedding(OBAMA_PROFILE)
    merkel = service.portrait_embedding(MERKEL_PORTRAIT)
    assert obama_front and obama_profile and merkel, "missing embedding for a portrait"

    same_person = cosine(obama_front, obama_profile)
    impostor = cosine(obama_front, merkel)
    assert same_person > impostor, (
        f"Obama(profile) similarity {same_person:.3f} is not above the Merkel impostor {impostor:.3f}"
    )
