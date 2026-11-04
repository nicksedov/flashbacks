"""HTTP API surface for the faces service."""

from __future__ import annotations

from typing import Any, Optional

from fastapi import APIRouter, HTTPException, Request
from pydantic import BaseModel, Field

from app.config import (
    CLUSTER_MIN_SIMILARITY,
    RECOGNITION_MARGIN,
    RECOGNITION_MIN_SIMILARITY,
)

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


class AnalyzedFace(BaseModel):
    x: float
    y: float
    width: float
    height: float
    confidence: float = 0.0
    embedding: list[float]


class AnalyzeRequest(BaseModel):
    image: str = Field(description="Base64-encoded image bytes.")


class AnalyzeResponse(BaseModel):
    faces: list[AnalyzedFace]


class BatchEntry(BaseModel):
    """The outcome of analysing one image of a batch."""

    faces: list[AnalyzedFace] = Field(default_factory=list)
    #: Set when this image failed; the other entries of the batch still carry faces.
    error: Optional[str] = None


class AnalyzeBatchRequest(BaseModel):
    images: list[str] = Field(
        min_length=1,
        description="Base64-encoded image bytes, one entry per image.",
    )


class AnalyzeBatchResponse(BaseModel):
    results: list[BatchEntry]


class EmbedRequest(BaseModel):
    image: str = Field(description="Base64-encoded image bytes.")
    box: Optional[FaceBox] = None


class EmbedResponse(BaseModel):
    embedding: list[float]


class ClusterRequest(BaseModel):
    embeddings: list[list[float]]
    # Two is the calibrated floor: a person photographed twice must still form a
    # cluster. Requiring more faces labels whole small galleries as noise.
    minClusterSize: int = 2
    # One flag per embedding: true when the face is the sole face of its photo.
    # Such a portrait may anchor a person for a face matched from a group shot.
    soloFaces: Optional[list[bool]] = None


class ClusterResponse(BaseModel):
    labels: list[int]


class FaceLink(BaseModel):
    """One candidate-similarity edge between two detected faces."""

    faceId: int
    candidateId: int
    similarity: float


class ClusterGraphRequest(BaseModel):
    """Faces to label plus the candidate links the vector store answered."""

    faceIds: list[int]
    soloFaces: Optional[list[bool]] = None
    edges: list[FaceLink] = Field(default_factory=list)


class ClusterGraphResponse(BaseModel):
    labels: list[int]


def _engine(request: Request):
    return request.app.state.engine


def _settings(request: Request):
    return request.app.state.settings


def _analyze_entry(engine: Any, image_b64: str) -> BatchEntry:
    """Analyse one image of a batch, containing any failure to its own entry.

    One unreadable photo must not abort the scan batch, so a decode or model
    failure becomes an ``error`` entry instead of a failed request.
    """
    try:
        return BatchEntry(faces=[AnalyzedFace(**face) for face in engine.analyze(image_b64)])
    except ValueError as exc:
        return BatchEntry(error=str(exc))
    except Exception as exc:  # noqa: BLE001 - isolate one image from the batch
        return BatchEntry(error=f"analysis failed: {exc}")


@router.get("/faces/health")
def health(request: Request) -> dict[str, Any]:
    """Report liveness, the active execution provider and the thresholds.

    Provider resolution intersects configuration with what the installed
    runtime offers, so an operator can confirm a GPU deployment from here
    without triggering a model load.
    """
    engine = _engine(request)
    selection = engine.provider_selection() if hasattr(engine, "provider_selection") else None
    payload: dict[str, Any] = {
        "status": "ok",
        # The faces service owns every calibrated similarity threshold, so the
        # caller reads them from here instead of configuring its own copies.
        "thresholds": {
            "clusterMinSimilarity": CLUSTER_MIN_SIMILARITY,
            "recognitionMinSimilarity": RECOGNITION_MIN_SIMILARITY,
            "recognitionMargin": RECOGNITION_MARGIN,
        },
    }
    if selection is not None:
        payload["activeProvider"] = selection.active_provider
        payload["requestedProviders"] = selection.requested_providers
    return payload


@router.post("/faces/analyze", response_model=AnalyzeResponse)
def analyze(req: AnalyzeRequest, request: Request) -> AnalyzeResponse:
    """Detect and embed every face of an image in a single model pass."""
    try:
        faces = _engine(request).analyze(req.image)
    except ValueError as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    except Exception as exc:  # noqa: BLE001 - surface a uniform server error
        raise HTTPException(status_code=500, detail=f"analysis failed: {exc}") from exc
    return AnalyzeResponse(faces=[AnalyzedFace(**face) for face in faces])


@router.post("/faces/analyze-batch", response_model=AnalyzeBatchResponse)
def analyze_batch(req: AnalyzeBatchRequest, request: Request) -> AnalyzeBatchResponse:
    """Detect and embed every face of several images in one request.

    The response carries exactly one entry per submitted image, in request
    order, so the caller never needs an id map. Requests larger than
    ``FACES_MAX_BATCH`` are rejected with ``422`` before any model work.
    """
    settings = _settings(request)
    if len(req.images) > settings.max_batch:
        raise HTTPException(
            status_code=422,
            detail=(
                f"batch of {len(req.images)} images exceeds the "
                f"FACES_MAX_BATCH limit of {settings.max_batch}"
            ),
        )

    engine = _engine(request)
    with request.app.state.batch_semaphore:
        results = [_analyze_entry(engine, image_b64) for image_b64 in req.images]
    return AnalyzeBatchResponse(results=results)


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
        labels = _engine(request).cluster(req.embeddings, req.minClusterSize, req.soloFaces)
    except Exception as exc:  # noqa: BLE001 - surface a uniform server error
        raise HTTPException(status_code=500, detail=f"clustering failed: {exc}") from exc
    return ClusterResponse(labels=labels)


@router.post("/faces/cluster-graph", response_model=ClusterGraphResponse)
def cluster_graph(req: ClusterGraphRequest, request: Request) -> ClusterGraphResponse:
    """Group faces from candidate links, one label per submitted face id.

    The caller queries the vector store for each face's neighbours inside the
    clustering radius; this service keeps ownership of the similarity
    thresholds and turns the graph into person labels.
    """
    edges = [(edge.faceId, edge.candidateId, edge.similarity) for edge in req.edges]
    try:
        labels = _engine(request).cluster_graph(req.faceIds, edges, req.soloFaces)
    except Exception as exc:  # noqa: BLE001 - surface a uniform server error
        raise HTTPException(status_code=500, detail=f"clustering failed: {exc}") from exc
    return ClusterGraphResponse(labels=labels)
