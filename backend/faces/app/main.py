"""FastAPI application for the faces service."""

from __future__ import annotations

import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI

from app.api import router
from app.engine import FaceEngine
from app.registry import from_env

logging.basicConfig(level=logging.INFO)

_registry = from_env()


@asynccontextmanager
async def lifespan(app: FastAPI):
    _registry.start()
    yield
    _registry.stop()


def create_app(engine: FaceEngine | None = None) -> FastAPI:
    app = FastAPI(title="Flashbacks faces", version="0.1.0", lifespan=lifespan)
    app.state.engine = engine if engine is not None else FaceEngine()
    app.include_router(router)
    return app


app = create_app()
