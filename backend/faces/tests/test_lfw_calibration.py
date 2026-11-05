"""Hermetic tests for the LFW calibration harness.

Nothing here reaches the network or loads a model: the Kaggle download opener is
injected, it serves a synthetic archive wrapped exactly as the Kaggle per-file
endpoint delivers it, and the engine factory returns deterministic vectors, so
the protocol, the download transport, the cache, the sweep and the dataset
lifecycle are covered in milliseconds.
"""

from __future__ import annotations

import base64
import io
import tarfile
import urllib.error
import zipfile
from dataclasses import dataclass
from pathlib import Path

import numpy as np
import pytest

from tools import lfw_calibration as lfw
from tools.lfw_calibration import (
    ACCEPTANCE_FLOOR,
    ARCHIVE_NAME,
    FOLD_COUNT,
    KAGGLE_DATASET,
    KAGGLE_IMAGES_DIR,
    PAIRS_NAME,
    PAIRS_PER_FOLD,
    TOTAL_PAIRS,
    UNDETECTED_SIMILARITY,
    CalibrationError,
    EmbeddingCache,
    FoldScores,
    LazyEngine,
    LazyKaggleOpener,
    candidate_rows,
    dataset_url,
    evaluate,
    fold_scores,
    format_candidates,
    image_path,
    main,
    pair_similarity,
    parse_pairs,
    resolve_token,
    resolve_work_dir,
    select_threshold,
    threshold_grid,
)

# ---------------------------------------------------------------------------
# Synthetic dataset
# ---------------------------------------------------------------------------

ALICE_PORTRAIT = "lfw/Alice/Alice_0001.jpg"
ALICE_PROFILE = "lfw/Alice/Alice_0002.jpg"
BOB_PORTRAIT = "lfw/Bob/Bob_0001.jpg"


def jpeg(seed: int) -> bytes:
    """Distinct, valid JPEG bytes per seed."""
    from PIL import Image

    image = Image.new("RGB", (16, 16), color=(seed % 256, (seed * 7) % 256, 32))
    buffer = io.BytesIO()
    image.save(buffer, format="JPEG")
    return buffer.getvalue()


def pairs_text(folds: int = FOLD_COUNT, per_fold: int = PAIRS_PER_FOLD // 2) -> str:
    """The official layout: per fold, ``per_fold`` same and ``per_fold`` different."""
    header = f"{folds}\t{per_fold}"
    lines = [header]
    for _fold in range(folds):
        for _ in range(per_fold):
            lines.append("Alice\t1\t2")
        for _ in range(per_fold):
            lines.append("Alice\t1\tBob\t1")
    return "\n".join(lines) + "\n"


def write_dataset(
    work_dir: Path, images: dict[str, bytes], pairs: str | None = None
) -> None:
    """Lay out an extracted LFW copy (``lfw/<name>/<name>_NNNN.jpg`` + pairs)."""
    for relative, data in images.items():
        target = work_dir / relative
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(data)
    (work_dir / lfw.PAIRS_NAME).write_text(pairs or pairs_text(), encoding="utf-8")


def archive_bytes(images: dict[str, bytes]) -> bytes:
    """A ``.tgz`` archive with the given members."""
    buffer = io.BytesIO()
    with tarfile.open(fileobj=buffer, mode="w:gz") as tar:
        for relative, data in images.items():
            info = tarfile.TarInfo(relative)
            info.size = len(data)
            info.mtime = 0
            tar.addfile(info, io.BytesIO(data))
    return buffer.getvalue()


def zip_bytes(members: dict[str, bytes]) -> bytes:
    """A ZIP container, as the Kaggle per-file endpoint returns binaries."""
    buffer = io.BytesIO()
    with zipfile.ZipFile(buffer, "w", zipfile.ZIP_STORED) as archive:
        for name, data in members.items():
            archive.writestr(name, data)
    return buffer.getvalue()


def funneled(images: dict[str, bytes]) -> dict[str, bytes]:
    """The archive layout Kaggle ships: ``lfw_funneled/<Name>/<Name>_NNNN.jpg``."""
    canonical = f"{lfw.IMAGES_DIR}/"
    return {
        f"{KAGGLE_IMAGES_DIR}/{relative.removeprefix(canonical)}": data
        for relative, data in images.items()
    }


def kaggle_payloads(
    images: dict[str, bytes],
    *,
    wrap: bool = True,
    pairs: "str | None" = None,
) -> dict[str, bytes]:
    """Canned responses for both dataset files, ZIP-wrapped like Kaggle does."""
    tarball = archive_bytes(funneled(images))
    return {
        dataset_url(PAIRS_NAME): (pairs or pairs_text()).encode(),
        dataset_url(ARCHIVE_NAME): (
            zip_bytes({ARCHIVE_NAME: tarball}) if wrap else tarball
        ),
    }


class FakeResponse(io.BytesIO):
    """Minimal stand-in for ``urlopen``: a context-managed readable stream."""

    def __enter__(self) -> "FakeResponse":
        return self

    def __exit__(self, *_exc: object) -> bool:
        self.close()
        return False


class FakeOpener:
    """Serve canned payloads per URL and remember what was requested."""

    def __init__(self, payloads: dict[str, bytes]) -> None:
        self.payloads = payloads
        self.urls: list[str] = []

    def __call__(self, url: str) -> FakeResponse:
        self.urls.append(url)
        if url not in self.payloads:
            raise urllib.error.URLError(f"no route to {url}")
        return FakeResponse(self.payloads[url])


@dataclass
class FakeEngine:
    """Deterministic engine: one known embedding per image payload."""

    vectors: dict[bytes, list[float] | None]
    name: str = "buffalo_l"

    def __post_init__(self) -> None:
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


def unit(cosine: float) -> list[float]:
    """A 2-D vector at *cosine* from ``[1, 0]``."""
    sine = float(np.sqrt(max(0.0, 1.0 - cosine * cosine)))
    return [cosine, sine]


ALICE_VECTOR = unit(1.0)
ALICE_PROFILE_VECTOR = unit(0.9)
BOB_VECTOR = unit(0.0)
WEAK_SAME = unit(0.3)
WEAK_DIFFERENT = unit(0.4)


def dataset_images() -> dict[str, bytes]:
    return {
        ALICE_PORTRAIT: jpeg(1),
        ALICE_PROFILE: jpeg(2),
        BOB_PORTRAIT: jpeg(3),
    }


def fake_dataset(work_dir: Path) -> dict[str, bytes]:
    images = dataset_images()
    write_dataset(work_dir, images)
    return images


# ---------------------------------------------------------------------------
# 1.1 — CLI surface
# ---------------------------------------------------------------------------


def test_help_lists_every_command(capsys: pytest.CaptureFixture[str]) -> None:
    with pytest.raises(SystemExit) as exit_info:
        main(["--help"])
    assert exit_info.value.code == 0
    printed = capsys.readouterr().out
    for command in ("download", "verify", "calibrate", "clean"):
        assert command in printed


# ---------------------------------------------------------------------------
# 1.2 / 1.3 — pairs protocol, download and extraction
# ---------------------------------------------------------------------------


def test_image_path_zero_pads_the_number() -> None:
    assert image_path("Adel_Abdel_Mahmoud", 1) == (
        "lfw/Adel_Abdel_Mahmoud/Adel_Abdel_Mahmoud_0001.jpg"
    )
    assert image_path("Ada", 13233) == "lfw/Ada/Ada_13233.jpg"


def test_parse_pairs_splits_the_ten_official_folds() -> None:
    pairs = parse_pairs(pairs_text())
    assert len(pairs) == TOTAL_PAIRS
    assert {pair.fold for pair in pairs} == set(range(1, FOLD_COUNT + 1))
    for fold in range(1, FOLD_COUNT + 1):
        of_fold = [pair for pair in pairs if pair.fold == fold]
        assert len(of_fold) == PAIRS_PER_FOLD
        assert sum(1 for pair in of_fold if pair.same) == PAIRS_PER_FOLD // 2
        assert sum(1 for pair in of_fold if not pair.same) == PAIRS_PER_FOLD // 2


def test_parse_pairs_resolves_identity_paths() -> None:
    pairs = parse_pairs(pairs_text())
    same = pairs[0]
    different = pairs[PAIRS_PER_FOLD // 2]
    assert (same.same, same.left, same.right) == (
        True,
        ALICE_PORTRAIT,
        ALICE_PROFILE,
    )
    assert (different.same, different.left, different.right) == (
        False,
        ALICE_PORTRAIT,
        BOB_PORTRAIT,
    )


def test_parse_pairs_rejects_a_wrong_fold_count() -> None:
    with pytest.raises(CalibrationError, match="expected 10 folds"):
        parse_pairs(pairs_text(folds=5))


def test_parse_pairs_rejects_a_short_pairs_file() -> None:
    truncated = "\n".join(pairs_text().splitlines()[:100]) + "\n"
    with pytest.raises(CalibrationError, match="pairs; expected 6000"):
        parse_pairs(truncated)


def test_parse_pairs_rejects_an_unexpected_line() -> None:
    lines = pairs_text().splitlines()
    lines[1] = "Alice 1 2 3 4"
    with pytest.raises(CalibrationError, match="unexpected pairs line"):
        parse_pairs("\n".join(lines) + "\n")


def test_dataset_url_points_at_the_kaggle_dataset() -> None:
    assert dataset_url(PAIRS_NAME) == (
        f"https://www.kaggle.com/api/v1/datasets/download/{KAGGLE_DATASET}/{PAIRS_NAME}"
    )


def test_download_unwraps_the_kaggle_zip_and_normalises_the_tree(
    tmp_path: Path,
) -> None:
    opener = FakeOpener(kaggle_payloads(dataset_images()))
    assert main(["download", "--work-dir", str(tmp_path)], opener=opener) == 0
    assert (tmp_path / ALICE_PORTRAIT).is_file()
    assert (tmp_path / BOB_PORTRAIT).is_file()
    assert (tmp_path / PAIRS_NAME).is_file()
    assert not (tmp_path / KAGGLE_IMAGES_DIR).exists()
    assert opener.urls == [dataset_url(PAIRS_NAME), dataset_url(ARCHIVE_NAME)]


def test_download_accepts_an_unwrapped_archive(tmp_path: Path) -> None:
    opener = FakeOpener(kaggle_payloads(dataset_images(), wrap=False))
    assert main(["download", "--work-dir", str(tmp_path)], opener=opener) == 0
    assert (tmp_path / ALICE_PORTRAIT).is_file()
    assert not (tmp_path / KAGGLE_IMAGES_DIR).exists()


def test_download_rejects_a_zip_without_the_archive(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    payloads = kaggle_payloads(dataset_images())
    payloads[dataset_url(ARCHIVE_NAME)] = zip_bytes({"other.tgz": b"payload"})
    assert main(["download", "--work-dir", str(tmp_path)], opener=FakeOpener(payloads)) == 1
    assert "expected exactly 1" in capsys.readouterr().err
    assert not (tmp_path / lfw.IMAGES_DIR).exists()


def test_download_rejects_a_malformed_pairs_file(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    opener = FakeOpener(kaggle_payloads(dataset_images(), pairs=pairs_text(folds=4)))
    assert main(["download", "--work-dir", str(tmp_path)], opener=opener) == 1
    assert not (tmp_path / lfw.IMAGES_DIR).exists()
    assert "expected 10 folds" in capsys.readouterr().err
    assert dataset_url(ARCHIVE_NAME) not in opener.urls


def test_download_reports_an_unreachable_host(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    opener = FakeOpener({})
    assert main(["download", "--work-dir", str(tmp_path)], opener=opener) == 1
    assert "cannot download" in capsys.readouterr().err


def test_verify_reports_the_protocol(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    fake_dataset(tmp_path)
    assert main(["verify", "--work-dir", str(tmp_path)]) == 0
    printed = capsys.readouterr().out
    assert "6000" in printed and "10 folds" in printed
    assert "images: 3" in printed


def test_verify_rejects_missing_images(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    fake_dataset(tmp_path)
    (tmp_path / ALICE_PROFILE).unlink()
    assert main(["verify", "--work-dir", str(tmp_path)]) == 1
    assert "missing" in capsys.readouterr().err


# ---------------------------------------------------------------------------
# 1.4 — Kaggle credentials
# ---------------------------------------------------------------------------


def test_token_prefers_the_environment_over_the_file(tmp_path: Path) -> None:
    token_file = tmp_path / "access_token"
    token_file.write_text("from-file\n", encoding="utf-8")

    assert resolve_token(str(token_file), {"KAGGLE_API_TOKEN": " from-env "}) == "from-env"
    assert resolve_token(str(token_file), {"KAGGLE_ACCESS_TOKEN": "legacy"}) == "legacy"
    assert resolve_token(str(token_file), {}) == "from-file"


def test_token_is_stripped_read_from_the_file(tmp_path: Path) -> None:
    token_file = tmp_path / "access_token"
    token_file.write_text("  KGAT_value  \n", encoding="utf-8")
    assert resolve_token(str(token_file), {}) == "KGAT_value"


def test_missing_token_fails_the_download(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    absent = tmp_path / "absent-token"
    code = main(
        [
            "download",
            "--work-dir",
            str(tmp_path / "work"),
            "--kaggle-token-file",
            str(absent),
        ],
        environ={},
    )
    assert code == 1
    assert "no Kaggle access token" in capsys.readouterr().err


def test_lazy_opener_resolves_the_token_on_first_download(tmp_path: Path) -> None:
    lazy = LazyKaggleOpener(str(tmp_path / "absent-token"), {})
    with pytest.raises(CalibrationError, match="no Kaggle access token"):
        lazy(dataset_url(PAIRS_NAME))


# ---------------------------------------------------------------------------
# 1.5 — content-hash embedding cache
# ---------------------------------------------------------------------------


def test_cache_keys_on_digest_model_and_preprocessing(tmp_path: Path) -> None:
    cache = EmbeddingCache(tmp_path / "embeddings.json", "buffalo_l", "v1")
    cache.put("abc", [0.1, 0.2])
    cache.save()
    assert cache.has("abc")
    assert cache.key("abc") == "abc|buffalo_l|v1"

    reloaded = EmbeddingCache(tmp_path / "embeddings.json", "buffalo_l", "v1")
    assert reloaded.get("abc") == [0.1, 0.2]

    other_model = EmbeddingCache(tmp_path / "embeddings.json", "antelopev2", "v1")
    assert not other_model.has("abc")

    other_version = EmbeddingCache(tmp_path / "embeddings.json", "buffalo_l", "v2")
    assert not other_version.has("abc")


def test_lazy_engine_rejects_a_model_mismatch() -> None:
    engine = LazyEngine(lambda: FakeEngine({}, name="antelopev2"), "buffalo_l")
    with pytest.raises(CalibrationError, match="--model-id antelopev2"):
        engine.get()
    assert not engine.loaded


def test_calibration_embeds_each_distinct_image_once(tmp_path: Path) -> None:
    images = fake_dataset(tmp_path)
    engine = FakeEngine(
        {images[ALICE_PORTRAIT]: ALICE_VECTOR, images[ALICE_PROFILE]: ALICE_PROFILE_VECTOR, images[BOB_PORTRAIT]: unit(0.0)}
    )
    assert (
        main(
            ["calibrate", "--work-dir", str(tmp_path), "--keep"],
            opener=FakeOpener({}),
            engine_factory=lambda: engine,
        )
        == 0
    )
    assert sorted(engine.analyzed) == sorted(images.values())
    assert len(engine.analyzed) == len(images)


def test_second_run_reuses_the_cache_and_loads_no_engine(tmp_path: Path) -> None:
    images = fake_dataset(tmp_path)
    vectors = {
        images[ALICE_PORTRAIT]: ALICE_VECTOR,
        images[ALICE_PROFILE]: ALICE_PROFILE_VECTOR,
        images[BOB_PORTRAIT]: BOB_VECTOR,
    }
    factories: list[FakeEngine] = []

    def factory() -> FakeEngine:
        engine = FakeEngine(vectors)
        factories.append(engine)
        return engine

    first = main(
        ["calibrate", "--work-dir", str(tmp_path), "--keep"],
        opener=FakeOpener({}),
        engine_factory=factory,
    )
    second = main(
        ["calibrate", "--work-dir", str(tmp_path), "--keep"],
        opener=FakeOpener({}),
        engine_factory=factory,
    )
    assert (first, second) == (0, 0)
    assert len(factories) == 1, "a warm cache must not construct an engine"
    assert len(factories[0].analyzed) == len(images)


# ---------------------------------------------------------------------------
# 1.5 — scoring, sweep, A/B table and the acceptance gate
# ---------------------------------------------------------------------------


def test_undetected_face_counts_as_a_non_match() -> None:
    pairs = parse_pairs(pairs_text())
    # Alice's profile yields no face; Bob's portrait is an orthogonal identity.
    embeddings = {
        ALICE_PORTRAIT: np.asarray(ALICE_VECTOR),
        ALICE_PROFILE: None,
        BOB_PORTRAIT: np.asarray(BOB_VECTOR),
    }
    same_pair = pairs[0]
    different_pair = pairs[PAIRS_PER_FOLD // 2]
    assert same_pair.same and different_pair.same is False
    assert pair_similarity(same_pair, embeddings) == UNDETECTED_SIMILARITY
    assert pair_similarity(different_pair, embeddings) == pytest.approx(0.0)

    evaluation = evaluate(fold_scores(pairs, embeddings), 0.25)
    # Every pair stays in the denominator: the same-person pairs with an
    # undetected face are errors, the different-person pairs are correct.
    assert evaluation.pairs == TOTAL_PAIRS
    assert evaluation.mean == pytest.approx(0.5)


def test_reported_accuracy_is_the_unweighted_fold_mean() -> None:
    scores = [
        FoldScores(
            fold=1,
            same=np.full(50, 0.9),
            different=np.full(50, 0.0),
        ),
        FoldScores(
            fold=2,
            same=np.full(500, 0.1),
            different=np.full(500, 0.9),
        ),
    ]
    evaluation = evaluate(scores, 0.25)
    assert evaluation.per_fold == (1.0, 0.0)
    assert evaluation.mean == pytest.approx(0.5)
    pooled = 100 / 1100
    assert evaluation.mean != pytest.approx(pooled)


def test_sweep_covers_the_documented_grid() -> None:
    grid = threshold_grid()
    assert grid[0] == 0.05
    assert grid[-1] == 0.95
    assert len(grid) == 181
    assert all(later > earlier for earlier, later in zip(grid, grid[1:]))


def test_select_threshold_breaks_ties_toward_the_stricter_value() -> None:
    rows = [(0.20, 0.90), (0.30, 0.95), (0.40, 0.95)]
    assert select_threshold(rows) == (0.40, 0.95)
    assert select_threshold([(0.20, 0.90), (0.25, 0.91)]) == (0.25, 0.91)


def test_ab_table_scores_candidates_on_identical_pairs(tmp_path: Path) -> None:
    images = fake_dataset(tmp_path)
    embeddings = {
        ALICE_PORTRAIT: np.asarray(ALICE_VECTOR),
        ALICE_PROFILE: np.asarray(ALICE_PROFILE_VECTOR),
        BOB_PORTRAIT: np.asarray(BOB_VECTOR),
    }
    pairs = parse_pairs((tmp_path / lfw.PAIRS_NAME).read_text())
    scores = fold_scores(pairs, embeddings)

    shipped = evaluate(scores, 0.25)
    selected_threshold, selected_accuracy = select_threshold(
        [(tau, evaluate(scores, tau).mean) for tau in threshold_grid()]
    )
    assert selected_accuracy >= shipped.mean
    rows = candidate_rows(shipped, evaluate(scores, selected_threshold))

    assert [row.label for row in rows] == ["shipped", "sweep"]
    assert {row.pairs for row in rows} == {TOTAL_PAIRS}
    assert {row.folds for row in rows} == {FOLD_COUNT}
    printed = format_candidates(rows)
    assert "shipped" in printed and "sweep" in printed
    assert str(TOTAL_PAIRS) in printed


def test_calibration_reports_the_selected_threshold(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    images = fake_dataset(tmp_path)
    engine = FakeEngine(
        {
            images[ALICE_PORTRAIT]: ALICE_VECTOR,
            images[ALICE_PROFILE]: ALICE_PROFILE_VECTOR,
            images[BOB_PORTRAIT]: BOB_VECTOR,
        }
    )
    assert (
        main(
            [
                "calibrate",
                "--work-dir",
                str(tmp_path),
                "--keep",
                "--shipped-threshold",
                "0.25",
            ],
            opener=FakeOpener({}),
            engine_factory=lambda: engine,
        )
        == 0
    )
    printed = capsys.readouterr().out
    # The sweep finds 0.9 (same-person pairs sit at 0.9) and reports it.
    assert "selected 0.9 (1.0000)" in printed
    assert "sweep" in printed


def test_calibration_reports_per_fold_accuracy(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    images = fake_dataset(tmp_path)
    engine = FakeEngine(
        {
            images[ALICE_PORTRAIT]: ALICE_VECTOR,
            images[ALICE_PROFILE]: ALICE_PROFILE_VECTOR,
            images[BOB_PORTRAIT]: BOB_VECTOR,
        }
    )
    assert (
        main(
            ["calibrate", "--work-dir", str(tmp_path), "--keep"],
            opener=FakeOpener({}),
            engine_factory=lambda: engine,
        )
        == 0
    )
    printed = capsys.readouterr().out
    for fold in range(1, FOLD_COUNT + 1):
        # One row per fold: the fold number with its pair count.
        assert f"{fold:>4}  {PAIRS_PER_FOLD:>5}" in printed
    assert "accepted" in printed


def test_below_floor_run_fails_and_writes_no_config(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    images = fake_dataset(tmp_path)
    engine = FakeEngine(
        {
            images[ALICE_PORTRAIT]: ALICE_VECTOR,
            images[ALICE_PROFILE]: WEAK_SAME,
            images[BOB_PORTRAIT]: WEAK_DIFFERENT,
        }
    )
    config = tmp_path / "config.py"
    original = "EMBEDDING_DIM = 512\nRECOGNITION_MIN_SIMILARITY = 0.25\n"
    config.write_text(original, encoding="utf-8")

    code = main(
        [
            "calibrate",
            "--work-dir",
            str(tmp_path),
            "--shipped-threshold",
            "0.25",
            "--apply-config",
            str(config),
        ],
        opener=FakeOpener({}),
        engine_factory=lambda: engine,
    )

    captured = capsys.readouterr()
    assert code == 1
    assert config.read_text(encoding="utf-8") == original
    assert f"below the {ACCEPTANCE_FLOOR:.2f} floor" in captured.err
    assert "best candidate threshold" in captured.err
    assert "work directory retained" in captured.err
    assert tmp_path.exists()


def test_accepted_run_writes_the_selected_threshold(tmp_path: Path) -> None:
    images = fake_dataset(tmp_path)
    engine = FakeEngine(
        {
            images[ALICE_PORTRAIT]: ALICE_VECTOR,
            images[ALICE_PROFILE]: ALICE_PROFILE_VECTOR,
            images[BOB_PORTRAIT]: BOB_VECTOR,
        }
    )
    config = tmp_path / "config.py"
    config.write_text("RECOGNITION_MIN_SIMILARITY = 0.25\n", encoding="utf-8")

    code = main(
        [
            "calibrate",
            "--work-dir",
            str(tmp_path),
            "--keep",
            "--shipped-threshold",
            "0.25",
            "--apply-config",
            str(config),
        ],
        opener=FakeOpener({}),
        engine_factory=lambda: engine,
    )

    assert code == 0
    assert config.read_text(encoding="utf-8") == "RECOGNITION_MIN_SIMILARITY = 0.9\n"


# ---------------------------------------------------------------------------
# 1.6 — dataset lifecycle
# ---------------------------------------------------------------------------


def test_successful_run_removes_the_work_directory(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    images = dataset_images()
    work_dir = tmp_path / "flashbacks-lfw-run"
    write_dataset(work_dir, images)
    engine = FakeEngine(
        {
            images[ALICE_PORTRAIT]: ALICE_VECTOR,
            images[ALICE_PROFILE]: ALICE_PROFILE_VECTOR,
            images[BOB_PORTRAIT]: BOB_VECTOR,
        }
    )

    code = main(
        ["calibrate", "--work-dir", str(work_dir)],
        opener=FakeOpener({}),
        engine_factory=lambda: engine,
    )

    assert code == 0
    assert not work_dir.exists()
    assert "work directory removed" in capsys.readouterr().out


def test_failed_run_retains_the_work_directory(
    tmp_path: Path, capsys: pytest.CaptureFixture[str]
) -> None:
    images = dataset_images()
    work_dir = tmp_path / "flashbacks-lfw-failure"
    write_dataset(work_dir, images)
    engine = FakeEngine(
        {
            images[ALICE_PORTRAIT]: ALICE_VECTOR,
            images[ALICE_PROFILE]: WEAK_SAME,
            images[BOB_PORTRAIT]: WEAK_DIFFERENT,
        }
    )

    code = main(
        ["calibrate", "--work-dir", str(work_dir), "--shipped-threshold", "0.25"],
        opener=FakeOpener({}),
        engine_factory=lambda: engine,
    )

    assert code == 1
    assert (work_dir / ALICE_PORTRAIT).is_file()
    assert "work directory retained" in capsys.readouterr().err

    assert main(["clean", "--work-dir", str(work_dir)]) == 0
    assert not work_dir.exists()


def test_clean_is_idempotent(tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
    missing = tmp_path / "absent"
    assert main(["clean", "--work-dir", str(missing)]) == 0
    assert "nothing to clean" in capsys.readouterr().out


def test_clean_without_a_work_directory_fails(
    capsys: pytest.CaptureFixture[str],
) -> None:
    assert main(["clean"], environ={}) == 1
    assert "no work directory given" in capsys.readouterr().err


def test_work_directory_comes_from_the_environment(tmp_path: Path) -> None:
    environ = {"LFW_WORK_DIR": str(tmp_path)}
    assert resolve_work_dir(None, environ, create=True) == tmp_path
    assert resolve_work_dir("other", environ, create=True) == Path("other")


def test_calibration_downloads_when_no_dataset_exists(tmp_path: Path) -> None:
    images = dataset_images()
    work_dir = tmp_path / "fresh"
    opener = FakeOpener(kaggle_payloads(images))
    engine = FakeEngine(
        {
            images[ALICE_PORTRAIT]: ALICE_VECTOR,
            images[ALICE_PROFILE]: ALICE_PROFILE_VECTOR,
            images[BOB_PORTRAIT]: BOB_VECTOR,
        }
    )

    code = main(
        ["calibrate", "--work-dir", str(work_dir)],
        opener=opener,
        engine_factory=lambda: engine,
    )

    assert code == 0
    assert opener.urls == [dataset_url(PAIRS_NAME), dataset_url(ARCHIVE_NAME)]
    assert not work_dir.exists()
