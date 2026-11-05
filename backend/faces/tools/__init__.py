"""Offline tooling for the faces service.

The package holds operator-facing utilities that are not part of the running
service. Nothing here is imported by ``app``, so the service image and its
startup path stay free of calibration-only code.
"""
