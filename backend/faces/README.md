# faces — Flashbacks face recognition service

Runs the Flashbacks face recognition ML pipeline over HTTP:

- **Detection** — SCRFD-10G (`det_10g.onnx`) via InsightFace.
- **Embedding** — ArcFace (`w600k_r50.onnx`), 512-dimensional vectors.
- **Clustering** — HDBSCAN with cosine metric (L2-normalized embeddings).

## Endpoints

| Method | Path | Description |
|---|---|---|
| GET | `/faces/health` | Health probe (`{"status": "ok"}`). |
| POST | `/faces/detect` | `{"image": "<base64>"}` → `{"boxes": [{x,y,width,height,confidence}]}` |
| POST | `/faces/embed` | `{"image": "<base64>", "box": {...}?}` → `{"embedding": [512 floats]}` |
| POST | `/faces/cluster` | `{"embeddings": [[...], ...], "minClusterSize": 5}` → `{"labels": [int]}` |

Detection and embedding models are downloaded by InsightFace on first use
(`~/.insightface`) and loaded lazily. Clustering is pure CPU.

## Local run

```bash
cd backend/faces
python -m venv .venv && . .venv/bin/activate
pip install -e ".[dev]"
uvicorn app.main:app --host 0.0.0.0 --port 5173
```

## Tests

```bash
cd backend/faces
python -m pytest
```

The unit tests exercise the endpoint plumbing with a deterministic fake engine and
the clustering with real HDBSCAN over synthetic embeddings, so they run without
downloading the ML models.

## Configuration

| Env | Default | Purpose |
|---|---|---|
| `FACES_PORT` | `5173` | HTTP listen port. |
| `REGISTRY_URL` | (empty) | Registry facade URL; empty disables registration. |
| `SERVICE_HOST` | `localhost` | Host peers use to reach this service. |
| `SERVICE_ID` | `<host>-<pid>` | Unique instance id. |
| `SERVICE_VERSION` | `dev` | Advertised version. |
| `REGISTRY_TOKEN` | (empty) | Bearer token for the registry facade. |

Registration is self-healing: the service retries with backoff until the registry
answers, heartbeats the lease, and re-registers after repeated heartbeat failures.
