FROM python:3.11-slim

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
