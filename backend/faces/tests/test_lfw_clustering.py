"""Hermetic tests for the LFW 1:N clustering calibration.

The dataset is written into ``tmp_path`` in the canonical layout, no opener is
allowed to fire, and the engine is a fake that returns one known vector per
payload. The labeller and the metrics are the production ones.
"""

from __future__ import annotations

import base64
import io
from pathlib import Path

import numpy as np
import pytest

from tools import lfw_clustering
from tools.clustering_benchmark import build_edges, label_faces, score_labels
from tools.lfw_clustering import main


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


def jpeg(seed: int) -> bytes:
    """Distinct, valid JPEG bytes per seed."""
    from PIL import Image

    image = Image.new("RGB", (16, 16), color=(seed % 256, (seed * 7) % 256, 32))
    buffer = io.BytesIO()
    image.save(buffer, format="JPEG")
    return buffer.getvalue()


class FakeEngine:
    """Deterministic engine: one known embedding per image payload."""

    name = "buffalo_l"

    def __init__(self, vectors: dict[bytes, list[float]]) -> None:
        self.vectors = vectors
        self.analyzed: list[bytes] = []

    def analyze(self, image_b64: str) -> list[dict]:
        raw = base64.b64decode(image_b64)
        self.analyzed.append(raw)
        vector = self.vectors.get(raw)
        if vector is None:
            return []
        return [
            {
                "x": 0.0,
                "y": 0.0,
                "width": 10.0,
                "height": 10.0,
                "confidence": 0.99,
                "embedding": vector,
            }
        ]


class NoDownloadOpener:
    """Proves a prepared work directory never reaches the network."""

    def __call__(self, url: str) -> object:
        raise AssertionError(f"unexpected download of {url}")


def pairs_text(folds: int = 10, per_fold: int = 300) -> str:
    """The official pairs layout, referencing the images this fixture writes."""
    lines = [f"{folds}\t{per_fold}"]
    for _fold in range(folds):
        for _ in range(per_fold):
            lines.append("Alice\t1\t2")
        for _ in range(per_fold):
            lines.append("Alice\t1\tBob\t1")
    return "\n".join(lines) + "\n"


def write_dataset(work_dir: Path, per_identity: int) -> dict[str, list[bytes]]:
    """Write ``lfw/<Name>/<Name>_NNNN.jpg`` plus a valid pairs file."""
    payloads = {name: [] for name in ("Alice", "Bob")}
    seed = 1
    for identity in payloads:
        for number in range(1, per_identity + 1):
            path = work_dir / "lfw" / identity / f"{identity}_{number:04d}.jpg"
            path.parent.mkdir(parents=True, exist_ok=True)
            data = jpeg(seed)
            path.write_bytes(data)
            payloads[identity].append(data)
            seed += 1
    (work_dir / "pairs.txt").write_text(pairs_text(), encoding="utf-8")
    return payloads


def axis(index: int, dimension: int = 8) -> list[float]:
    vector = [0.0] * dimension
    vector[index] = 1.0
    return vector


def blend(left: int, right: int, cosine: float, dimension: int = 8) -> list[float]:
    sine = float(np.sqrt(max(0.0, 1.0 - cosine * cosine)))
    vector = [0.0] * dimension
    vector[left] = cosine
    vector[right] = sine
    return vector


def separate_vectors(payloads: dict[str, list[bytes]]) -> dict[bytes, list[float]]:
    """Two well-separated identities: Alice on axis 0, Bob on axis 2."""
    vectors: dict[bytes, list[float]] = {}
    for number, data in enumerate(payloads["Alice"]):
        vectors[data] = blend(0, 1, 0.95 - 0.01 * number)
    for number, data in enumerate(payloads["Bob"]):
        vectors[data] = blend(2, 3, 0.95 - 0.01 * number)
    return vectors


def merged_vectors(payloads: dict[str, list[bytes]]) -> dict[bytes, list[float]]:
    """A lookalike pair: every face sits at 0.98 of the same axis."""
    return {
        data: blend(0, 1, 0.98)
        for data in payloads["Alice"] + payloads["Bob"]
    }


# ---------------------------------------------------------------------------
# Graph and metrics against the LFW layout
# ---------------------------------------------------------------------------


def test_graph_and_metrics_use_the_identity_folders(tmp_path: Path) -> None:
    payloads = write_dataset(tmp_path, per_identity=4)
    images = lfw_clustering.lfw_subset(tmp_path)
    vectors = [np.asarray(separate_vectors(payloads)[image.path.read_bytes()]) for image in images]

    edges = build_edges(vectors, [image.photo for image in images])
    labels = label_faces(images, edges)
    score = score_labels(
        labels,
        [image.identity for image in images],
        algorithm="graph-v1",
        floor=0.4,
        top_k=16,
    )

    assert {image.identity for image in images} == {"Alice", "Bob"}
    assert score.identity_merges == 0
    assert score.identity_integrity == 1.0


def test_subset_skips_single_photo_identities(tmp_path: Path) -> None:
    work_dir = tmp_path / "lfw"
    solo = work_dir / "Carol" / "Carol_0001.jpg"
    solo.parent.mkdir(parents=True)
    solo.write_bytes(jpeg(99))
    write_dataset(tmp_path, per_identity=2)

    images = lfw_clustering.lfw_subset(tmp_path)

    assert "Carol" not in {image.identity for image in images}


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------


def test_score_reports_a_clean_separation(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    work_dir = tmp_path / "work"
    work_dir.mkdir()
    payloads = write_dataset(work_dir, per_identity=4)
    engine = FakeEngine(separate_vectors(payloads))

    code = main(
        ["score", "--work-dir", str(work_dir), "--algorithm", "graph-v1"],
        opener=NoDownloadOpener(),
        engine_factory=lambda: engine,
        environ={},
    )

    printed = capsys.readouterr().out
    assert code == 0
    assert "identity merges" in printed
    integrity = next(
        line for line in printed.splitlines() if line.startswith("identity integrity")
    )
    assert integrity.split()[-1] == "1.0000"
    assert not work_dir.exists()


def test_score_gate_fails_when_identities_merge(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    work_dir = tmp_path / "work"
    work_dir.mkdir()
    payloads = write_dataset(work_dir, per_identity=4)
    engine = FakeEngine(merged_vectors(payloads))

    code = main(
        [
            "score",
            "--work-dir",
            str(work_dir),
            "--algorithm",
            "graph-v1",
            "--max-merge-rate",
            "0",
        ],
        opener=NoDownloadOpener(),
        engine_factory=lambda: engine,
        environ={},
    )

    captured = capsys.readouterr()
    assert code == 1
    assert "gate failed: merge rate" in captured.err
    assert work_dir.exists(), "a failed gate keeps the work directory"
    assert "work directory retained" in captured.err


def test_sweep_fails_when_merges_exceed_the_gate(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    work_dir = tmp_path / "work"
    work_dir.mkdir()
    payloads = write_dataset(work_dir, per_identity=4)
    engine = FakeEngine(merged_vectors(payloads))

    code = main(
        ["sweep", "--work-dir", str(work_dir), "--max-merges", "0"],
        opener=NoDownloadOpener(),
        engine_factory=lambda: engine,
        environ={},
    )

    captured = capsys.readouterr()
    assert code == 1
    assert "gate failed: best configuration merges" in captured.err
    assert "recommended:" in captured.out


def test_sweep_without_a_gate_reports_and_succeeds(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    work_dir = tmp_path / "work"
    work_dir.mkdir()
    payloads = write_dataset(work_dir, per_identity=4)
    engine = FakeEngine(merged_vectors(payloads))

    code = main(
        ["sweep", "--work-dir", str(work_dir), "--quiet"],
        opener=NoDownloadOpener(),
        engine_factory=lambda: engine,
        environ={},
    )

    assert code == 0
    assert not work_dir.exists()


def test_hard_negatives_reports_the_ceiling(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    work_dir = tmp_path / "work"
    work_dir.mkdir()
    payloads = write_dataset(work_dir, per_identity=4)
    engine = FakeEngine(merged_vectors(payloads))

    code = main(
        ["hard-negatives", "--work-dir", str(work_dir), "--floors", "0.40,0.99"],
        opener=NoDownloadOpener(),
        engine_factory=lambda: engine,
        environ={},
    )

    printed = capsys.readouterr().out
    assert code == 0
    assert "best cross-identity similarity" in printed
    assert "faces whose nearest foreign neighbour >= 0.400" in printed
    assert "faces whose nearest foreign neighbour >= 0.990" in printed


def test_json_output_records_the_score(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    import json

    work_dir = tmp_path / "work"
    work_dir.mkdir()
    payloads = write_dataset(work_dir, per_identity=4)
    out = tmp_path / "score.json"

    code = main(
        ["score", "--work-dir", str(work_dir), "--quiet", "--json-out", str(out)],
        opener=NoDownloadOpener(),
        engine_factory=lambda: FakeEngine(separate_vectors(payloads)),
        environ={},
    )

    assert code == 0
    payload = json.loads(out.read_text(encoding="utf-8"))
    assert payload["identityMerges"] == 0
    assert payload["faces"] == 8
