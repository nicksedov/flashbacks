"""Multi-GPU device-pool tests.

``FACES_GPU_IDS=0,1`` keeps one model session per card. Concurrent analyses
must land on different devices instead of queueing behind a single session.
"""

from __future__ import annotations

import base64
import io
import threading
from concurrent.futures import ThreadPoolExecutor

from PIL import Image

from app.config import FaceSettings
from app.engine import FaceEngine

CUDA = "CUDAExecutionProvider"
CPU = "CPUExecutionProvider"


def _frame_b64() -> str:
    buf = io.BytesIO()
    Image.new("RGB", (128, 128), (90, 60, 40)).save(buf, format="JPEG")
    return base64.b64encode(buf.getvalue()).decode()


class ProbeAnalysis:
    """Fake session that records its device and waits for its peer."""

    def __init__(self, device_id: int, barrier: threading.Barrier, seen: list[int]) -> None:
        self.device_id = device_id
        self.barrier = barrier
        self.seen = seen
        self.models = {}

    def get(self, img):  # noqa: ARG002 - image content is irrelevant
        self.seen.append(self.device_id)
        # Only two devices make this return; one session would time out.
        self.barrier.wait(timeout=5)
        return []


def _multi_gpu_engine(barrier: threading.Barrier, seen: list[int]) -> FaceEngine:
    settings = FaceSettings(providers=(CUDA, CPU), gpu_ids=(0, 1))
    engine = FaceEngine(settings=settings, available=(CUDA, CPU))
    engine._load_analysis = lambda selection: ProbeAnalysis(  # type: ignore[method-assign]
        selection.device_id, barrier, seen
    )
    return engine


def test_gpu_ids_are_parsed_from_the_environment() -> None:
    settings = FaceSettings.from_env({"FACES_GPU_IDS": "0, 1,broken"})

    assert settings.gpu_ids == (0, 1)


def test_single_gpu_id_remains_supported() -> None:
    settings = FaceSettings.from_env({"FACES_GPU_IDS": "", "FACES_GPU_ID": "1"})

    assert settings.gpu_ids == (1,)


def test_pool_reports_every_device() -> None:
    engine = FaceEngine(
        settings=FaceSettings(providers=(CUDA, CPU), gpu_ids=(0, 1)),
        available=(CUDA, CPU),
    )

    assert engine.device_count == 2
    assert engine.device_slots() == 2
    assert engine.device_info() == [
        {"deviceId": 0, "provider": "cuda"},
        {"deviceId": 1, "provider": "cuda"},
    ]


def test_cpu_host_keeps_one_worker() -> None:
    engine = FaceEngine(
        settings=FaceSettings(providers=(CUDA, CPU), gpu_ids=(0, 1)),
        available=(CPU,),
    )

    assert engine.device_count == 1
    assert engine.device_info() == [{"deviceId": -1, "provider": "cpu"}]


def test_concurrent_analyses_use_both_devices() -> None:
    barrier = threading.Barrier(2)
    seen: list[int] = []
    engine = _multi_gpu_engine(barrier, seen)
    image = _frame_b64()

    with ThreadPoolExecutor(max_workers=2) as pool:
        results = list(pool.map(lambda _: engine.analyze(image), range(2)))

    assert results == [[], []]
    assert sorted(seen) == [0, 1]
