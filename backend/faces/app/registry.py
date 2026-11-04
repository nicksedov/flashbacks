"""Self-healing registration with the Flashbacks registry REST facade."""

from __future__ import annotations

import json
import logging
import os
import socket
import threading
import time
import urllib.error
import urllib.request

logger = logging.getLogger("faces.registry")

_MAX_HEARTBEAT_FAILURES = 2
_BACKOFF_INITIAL = 0.5
_BACKOFF_MAX = 15.0


class RegistryClient:
    """Registers this instance and keeps its lease alive.

    Mirrors the Go registry client's self-healing rules: retry with backoff on
    startup, heartbeat the lease, and re-register with a fresh lease after
    repeated heartbeat failures.
    """

    def __init__(
        self,
        endpoint: str,
        service: str,
        instance_id: str,
        host: str,
        port: int,
        base_path: str = "",
        health_path: str = "",
        version: str = "dev",
        ttl: int = 15,
        token: str = "",
    ) -> None:
        self.endpoint = endpoint.rstrip("/")
        self.service = service
        self.instance_id = instance_id
        self.instance = {
            "host": host,
            "port": port,
            "basePath": base_path,
            "healthPath": health_path,
            "version": version,
        }
        self.ttl = ttl
        self.token = token
        self.lease_id: int = 0
        self._stop = threading.Event()
        self._thread: threading.Thread | None = None

    @property
    def base_path(self) -> str:
        return f"/registry/v1/services/{self.service}/{self.instance_id}"

    def _request(self, method: str, path: str, body: dict | None = None) -> dict | None:
        url = self.endpoint + path
        data = json.dumps(body).encode() if body is not None else None
        req = urllib.request.Request(url, data=data, method=method)
        req.add_header("Content-Type", "application/json")
        if self.token:
            req.add_header("Authorization", "Bearer " + self.token)
        with urllib.request.urlopen(req, timeout=15) as resp:
            raw = resp.read()
            return json.loads(raw) if raw else None

    def _register_once(self) -> bool:
        try:
            resp = self._request(
                "PUT",
                f"{self.base_path}?ttl={self.ttl}",
                self.instance,
            )
            self.lease_id = int(resp["leaseId"]) if resp else 0
            logger.info("registered in registry", extra={"instance": self.instance_id})
            return True
        except Exception as exc:  # noqa: BLE001 - retry loop handles failures
            logger.warning("registry registration failed", extra={"error": str(exc)})
            return False

    def _heartbeat(self) -> bool:
        try:
            self._request("POST", f"{self.base_path}/heartbeat", {"leaseId": self.lease_id})
            return True
        except Exception as exc:  # noqa: BLE001
            logger.warning("registry heartbeat failed", extra={"error": str(exc)})
            return False

    def start(self) -> None:
        if not self.endpoint:
            logger.info("registry integration disabled")
            return
        self._thread = threading.Thread(target=self._run, name="faces-registry", daemon=True)
        self._thread.start()

    def stop(self) -> None:
        self._stop.set()
        if self.lease_id:
            try:
                self._request("DELETE", f"{self.base_path}?leaseId={self.lease_id}")
            except Exception as exc:  # noqa: BLE001
                logger.warning("registry deregistration failed", extra={"error": str(exc)})
            self.lease_id = 0

    def _run(self) -> None:
        backoff = _BACKOFF_INITIAL
        while not self._stop.is_set():
            if self._register_once():
                break
            if self._stop.wait(min(backoff, _BACKOFF_MAX)):
                return
            backoff = min(backoff * 2, _BACKOFF_MAX)

        interval = max(1.0, self.ttl / 3)
        failures = 0
        while not self._stop.is_set():
            if self._stop.wait(interval):
                return
            if self._heartbeat():
                failures = 0
                continue
            failures += 1
            if failures < _MAX_HEARTBEAT_FAILURES:
                continue
            if self._register_once():
                failures = 0


def default_instance_id(service: str) -> str:
    host = socket.gethostname() or "local"
    return f"{service}-{host}-{os.getpid()}"


def from_env() -> RegistryClient:
    service = "faces"
    port = int(os.environ.get("FACES_PORT", "5173"))
    host = os.environ.get("SERVICE_HOST", "localhost")
    instance_id = os.environ.get("SERVICE_ID", "").strip() or default_instance_id(service)
    return RegistryClient(
        endpoint=os.environ.get("REGISTRY_URL", "").strip(),
        service=service,
        instance_id=instance_id,
        host=host,
        port=port,
        base_path="/faces",
        health_path="/faces/health",
        version=os.environ.get("SERVICE_VERSION", "dev"),
        token=os.environ.get("REGISTRY_TOKEN", ""),
    )
