"""FastAPI application for the faces service."""

from __future__ import annotations

import logging
import threading
from contextlib import asynccontextmanager
from typing import Optional

from fastapi import FastAPI

from app.api import router
from app.config import FaceSettings
from app.engine import FaceEngine
from app.registry import from_env

logging.basicConfig(level=logging.INFO)

_registry = from_env()


@asynccontextmanager
async def lifespan(app: FastAPI):
    _registry.start()
    yield
    _registry.stop()


def create_app(
    engine: Optional[FaceEngine] = None, settings: Optional[FaceSettings] = None
) -> FastAPI:
    """Build the application, wiring the engine and its batch limits.

    ``settings`` defaults to the engine's own settings, then to the
    environment, so tests can pin a small batch cap without touching ``os.environ``.
    """
    resolved_settings = settings or getattr(engine, "settings", None) or FaceSettings.from_env()
    app = FastAPI(title="Flashbacks faces", version="0.1.0", lifespan=lifespan)
    app.state.settings = resolved_settings
    app.state.engine = engine if engine is not None else FaceEngine(settings=resolved_settings)
    # Serialise batch analyses: two concurrent GPU batches would double VRAM use
    # while one device can only work on one batch at a time.
    app.state.batch_semaphore = threading.Semaphore(resolved_settings.max_concurrency)
    app.include_router(router)
    return app


app = create_app()
