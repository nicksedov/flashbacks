"""Execution-provider selection tests.

The service must run on the provider it was configured for, keep the CPU
default when nothing is configured, and degrade to CPU (with a warning) when a
requested accelerator is missing — a CPU-only staging host must stay usable.
"""

from __future__ import annotations

import logging

from app.config import FaceSettings, resolve_providers
from app.engine import FaceEngine

CUDA = "CUDAExecutionProvider"
CPU = "CPUExecutionProvider"


def _engine(environ: dict[str, str], available: list[str]) -> FaceEngine:
    return FaceEngine(settings=FaceSettings.from_env(environ), available=available)


def test_gpu_provider_selected_when_available() -> None:
    engine = _engine({"FACES_PROVIDERS": "cuda,cpu", "FACES_GPU_ID": "1"}, [CUDA, CPU])

    selection = engine.provider_selection()

    assert selection.active_provider == "cuda"
    assert selection.active == (CUDA, CPU)
    # ctx_id points at the configured GPU, not at CPU.
    assert selection.device_id == 1


def test_cpu_provider_default() -> None:
    """An unconfigured service stays on CPU even when CUDA is offered."""
    engine = _engine({}, [CUDA, CPU])

    selection = engine.provider_selection()

    assert selection.active_provider == "cpu"
    assert selection.active == (CPU,)
    assert selection.device_id == -1


def test_cuda_unavailable_falls_back(caplog) -> None:
    with caplog.at_level(logging.WARNING, logger="faces.config"):
        engine = _engine({"FACES_PROVIDERS": "cuda,cpu"}, [CPU])
        selection = engine.provider_selection()

    assert selection.active_provider == "cpu"
    assert selection.device_id == -1
    assert selection.requested_providers == ["cuda", "cpu"]
    # The operator must be able to see why the GPU path is not live.
    assert "CUDAExecutionProvider is unavailable" in caplog.text
    assert "using CPUExecutionProvider" in caplog.text


def test_unavailable_sole_provider_falls_back_to_cpu(caplog) -> None:
    with caplog.at_level(logging.WARNING, logger="faces.config"):
        selection = resolve_providers(["cuda"], [CPU])

    assert selection.active == (CPU,)
    assert selection.device_id == -1
    assert "falling back to CPUExecutionProvider" in caplog.text


def test_gpu_id_defaults_to_zero_for_an_active_gpu() -> None:
    selection = resolve_providers(["cuda"], [CUDA, CPU])

    assert selection.active == (CUDA,)
    assert selection.device_id == 0


def test_full_names_are_accepted_in_priority_order() -> None:
    selection = resolve_providers(
        ["TensorrtExecutionProvider", "cuda", "CPUExecutionProvider"], [CPU, CUDA]
    )

    # TensorRT is not offered, so the next requested provider wins.
    assert selection.active == (CUDA, CPU)
    assert selection.active_provider == "cuda"


def test_dropped_leading_provider_is_reported() -> None:
    selection = resolve_providers(["cuda", "cpu"], [CPU])

    assert selection.requested == (CUDA, CPU)
    assert selection.active_provider == "cpu"
