"""Clustering tests: the dataset identities must cluster into distinct people.

The photo set provides two identities with more than one portrait (Obama with
two, Einstein with three), which is what makes clustering meaningful at this
size. HDBSCAN may legitimately label a photo as noise (``-1``) — a single old or
unusual shot — so noise is tolerated; splitting one person across clusters or
merging two people into one cluster is not.
"""

from __future__ import annotations

from dataclasses import dataclass

from support import IDENTITY_PORTRAITS


@dataclass(frozen=True)
class Portrait:
    """A dataset photo annotated with the person it shows."""

    identity: str
    photo: str


def _cluster_portraits(service, min_cluster_size: int) -> tuple[list[Portrait], list[int]]:
    """Embed every person portrait and cluster the resulting gallery."""
    portraits: list[Portrait] = []
    embeddings: list[list[float]] = []
    for identity, photos in IDENTITY_PORTRAITS.items():
        for photo in photos:
            embedding = service.portrait_embedding(photo)
            assert embedding, f"{photo}: no embedding — no face was detected"
            portraits.append(Portrait(identity=identity, photo=photo))
            embeddings.append(embedding)

    labels = service.cluster(embeddings, min_cluster_size=min_cluster_size)
    assert len(labels) == len(portraits), (
        f"/cluster returned {len(labels)} label(s) for {len(portraits)} embedding(s)"
    )
    return portraits, labels


def _format_labels(portraits: list[Portrait], labels: list[int]) -> str:
    return ", ".join(
        f"{portrait.photo}={label}" for portrait, label in zip(portraits, labels)
    )


def _labels_by_identity(portraits: list[Portrait], labels: list[int]) -> dict[str, list[int]]:
    grouped: dict[str, list[int]] = {}
    for portrait, label in zip(portraits, labels):
        grouped.setdefault(portrait.identity, []).append(label)
    return grouped


def test_clusters_never_mix_different_people(service, cluster_min_size) -> None:
    """A cluster must not contain photos of two different people."""
    portraits, labels = _cluster_portraits(service, cluster_min_size)

    identities_by_label: dict[int, set[str]] = {}
    for portrait, label in zip(portraits, labels):
        if label == -1:
            continue
        identities_by_label.setdefault(label, set()).add(portrait.identity)

    mixed = {
        label: sorted(identities)
        for label, identities in identities_by_label.items()
        if len(identities) > 1
    }
    assert not mixed, (
        f"clusters merge different people: {mixed}; labels: {_format_labels(portraits, labels)}"
    )


def test_same_person_is_not_split_between_clusters(service, cluster_min_size) -> None:
    """Photos of one person never land in two different clusters."""
    portraits, labels = _cluster_portraits(service, cluster_min_size)

    split: dict[str, set[int]] = {}
    for identity, identity_labels in _labels_by_identity(portraits, labels).items():
        distinct = {label for label in identity_labels if label != -1}
        if len(distinct) > 1:
            split[identity] = distinct

    assert not split, (
        f"the same person was split across clusters: {split}; "
        f"labels: {_format_labels(portraits, labels)}"
    )


def test_at_least_one_identity_forms_its_own_cluster(service, cluster_min_size) -> None:
    """Multi-portrait identities must actually cluster (not everything is noise)."""
    portraits, labels = _cluster_portraits(service, cluster_min_size)

    fully_clustered = []
    for identity, identity_labels in _labels_by_identity(portraits, labels).items():
        if len(identity_labels) < 2:
            continue
        if all(label != -1 for label in identity_labels) and len(set(identity_labels)) == 1:
            fully_clustered.append(identity)

    assert fully_clustered, (
        "no identity with several photos formed a cluster of its own; "
        f"labels: {_format_labels(portraits, labels)}"
    )
