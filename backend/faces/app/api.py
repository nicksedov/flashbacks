"""HTTP API surface for the faces service."""

from __future__ import annotations

from typing import Optional

from fastapi import APIRouter, HTTPException, Request
from pydantic import BaseModel, Field

router = APIRouter()


class FaceBox(BaseModel):
    x: float
    y: float
    width: float
    height: float
    confidence: float = 0.0


class DetectRequest(BaseModel):
    image: str = Field(description="Base64-encoded image bytes.")


class DetectResponse(BaseModel):
    boxes: list[FaceBox]


class EmbedRequest(BaseModel):
    image: str = Field(description="Base64-encoded image bytes.")
    box: Optional[FaceBox] = None


class EmbedResponse(BaseModel):
    embedding: list[float]


class ClusterRequest(BaseModel):
    embeddings: list[list[float]]
    minClusterSize: int = 5


class ClusterResponse(BaseModel):
    labels: list[int]


def _engine(request: Request):
    return request.app.state.engine


@router.get("/faces/health")
def health() -> dict[str, str]:
    return {"status": "ok"}


@router.post("/faces/detect", response_model=DetectResponse)
def detect(req: DetectRequest, request: Request) -> DetectResponse:
    try:
        boxes = _engine(request).detect(req.image)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    except Exception as exc:  # noqa: BLE001 - surface a uniform server error
        raise HTTPException(status_code=500, detail=f"detection failed: {exc}") from exc
    return DetectResponse(boxes=[FaceBox(**box) for box in boxes])


@router.post("/faces/embed", response_model=EmbedResponse)
def embed(req: EmbedRequest, request: Request) -> EmbedResponse:
    box = req.box.model_dump() if req.box is not None else None
    try:
        embedding = _engine(request).embed(req.image, box)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    except Exception as exc:  # noqa: BLE001 - surface a uniform server error
        raise HTTPException(status_code=500, detail=f"embedding failed: {exc}") from exc
    return EmbedResponse(embedding=embedding)


@router.post("/faces/cluster", response_model=ClusterResponse)
def cluster(req: ClusterRequest, request: Request) -> ClusterResponse:
    try:
        labels = _engine(request).cluster(req.embeddings, req.minClusterSize)
    except Exception as exc:  # noqa: BLE001 - surface a uniform server error
        raise HTTPException(status_code=500, detail=f"clustering failed: {exc}") from exc
    return ClusterResponse(labels=labels)
