# Faces service image.
#
# Two targets share one application payload:
#   * gpu — CUDA 13 / cuDNN 9 runtime with onnxruntime-gpu (built explicitly with
#           --target gpu; the CUDA base is pinned so a missing tag fails loudly);
#   * cpu — the default target (`docker build backend/faces`), python:3.11-slim
#           with CPU onnxruntime.
#
# The CUDA base is forced through ARG FACES_CUDA_BASE, whose default is exactly
# the pinned tag: there is no local-cache tag resolution and no CUDA 12 fallback,
# so the CUDA major cannot silently change under a build.

ARG FACES_CUDA_BASE=nvidia/cuda:13.4.2-cudnn-runtime-ubuntu24.04

# ─── GPU target ───────────────────────────────────────────────────────────────
FROM ${FACES_CUDA_BASE} AS gpu

ENV DEBIAN_FRONTEND=noninteractive

# The 24.04 base ships Python 3.12, but the service (and its dependencies) are
# validated on 3.11, so install that interpreter explicitly.
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        software-properties-common \
        ca-certificates \
        libgl1 \
        libglib2.0-0 \
        libgomp1 \
    && add-apt-repository -y ppa:deadsnakes/ppa \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
        python3.11 \
        python3.11-venv \
    && rm -rf /var/lib/apt/lists/*

# A dedicated venv keeps the 3.11 interpreter unambiguous next to the base's 3.12.
RUN python3.11 -m venv /opt/venv
ENV PATH="/opt/venv/bin:${PATH}"

RUN pip install --no-cache-dir --upgrade pip

# Unprivileged runtime user. InsightFace caches its models under
# ~/.insightface, so HOME points at a writable directory that the Compose stack
# mounts as a named volume to keep models across container recreations.
RUN useradd --create-home --uid 10001 --home-dir /home/app app

WORKDIR /app

COPY pyproject.toml README.md ./
COPY app ./app

# pyproject.toml depends on CPU onnxruntime; replace it with the wheel built for
# CUDA 13 / cuDNN 9 so the two runtimes cannot be mixed in one environment.
RUN pip install --no-cache-dir . \
    && pip uninstall -y onnxruntime \
    && pip install --no-cache-dir "onnxruntime-gpu==1.31.1" \
    && mkdir -p /home/app/.insightface \
    && chown -R app:app /home/app /app

ENV HOME=/home/app \
    PYTHONUNBUFFERED=1 \
    FACES_PORT=5173 \
    NVIDIA_DRIVER_CAPABILITIES=compute,utility

USER app

EXPOSE 5173

# CUDA context creation and model loading take longer than on CPU, hence the
# raised start period.
HEALTHCHECK --interval=15s --timeout=5s --start-period=180s --retries=5 \
  CMD python -c "import sys, urllib.request; sys.exit(0 if urllib.request.urlopen('http://127.0.0.1:5173/faces/health', timeout=3).status == 200 else 1)"

CMD ["sh", "-c", "uvicorn app.main:app --host 0.0.0.0 --port ${FACES_PORT}"]

# ─── CPU target (default) ─────────────────────────────────────────────────────
FROM python:3.11-slim AS cpu

# Runtime dependencies for onnxruntime / InsightFace / Pillow.
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        libgl1 \
        libglib2.0-0 \
        libgomp1 \
    && rm -rf /var/lib/apt/lists/*

# Unprivileged runtime user. InsightFace caches its models under
# ~/.insightface, so HOME points at a writable directory that the Compose stack
# mounts as a named volume to keep models across container recreations.
RUN useradd --create-home --uid 10001 --home-dir /home/app app

WORKDIR /app

COPY pyproject.toml README.md ./
COPY app ./app

RUN pip install --no-cache-dir . \
    && mkdir -p /home/app/.insightface \
    && chown -R app:app /home/app /app

ENV HOME=/home/app \
    PYTHONUNBUFFERED=1 \
    FACES_PORT=5173

USER app

EXPOSE 5173

# python:slim ships no curl, so probe the HTTP health endpoint with the
# interpreter the service already runs on.
HEALTHCHECK --interval=15s --timeout=5s --start-period=60s --retries=5 \
  CMD python -c "import sys, urllib.request; sys.exit(0 if urllib.request.urlopen('http://127.0.0.1:5173/faces/health', timeout=3).status == 200 else 1)"

CMD ["sh", "-c", "uvicorn app.main:app --host 0.0.0.0 --port ${FACES_PORT}"]
