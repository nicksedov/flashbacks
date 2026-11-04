FROM python:3.11-slim

# Runtime dependencies for onnxruntime / InsightFace / Pillow.
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        libgl1 \
        libglib2.0-0 \
        libgomp1 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY pyproject.toml README.md ./
COPY app ./app

RUN pip install --no-cache-dir .

ENV FACES_PORT=5173

EXPOSE 5173

CMD ["sh", "-c", "uvicorn app.main:app --host 0.0.0.0 --port ${FACES_PORT}"]
