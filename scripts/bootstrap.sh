#!/usr/bin/env bash
#
# bootstrap.sh — provision host-native development prerequisites without root.
#
# The IDE container ships with Go, Node, Python and Tesseract, but exiftool is
# not installed and there is no passwordless apt. This script fills the gap by
# installing ExifTool locally into .tools/ (a pure-Perl distribution) and by
# setting up the Python virtualenv used by the OCR pytest suite.
#
# Usage:
#   scripts/bootstrap.sh [all|godeps|webapp|pytest|exiftool]
#   (no argument = all)

set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TOOLS_DIR="$ROOT_DIR/.tools"
TOOLS_BIN="$TOOLS_DIR/bin"

PYTEST_DIR="$ROOT_DIR/tests/ocr/integration"
PYTEST_VENV="$PYTEST_DIR/.venv"

GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; NC='\033[0m'
info() { printf '%b\n' "${GREEN}[bootstrap]${NC} $*"; }
warn() { printf '%b\n' "${YELLOW}[bootstrap]${NC} $*"; }
err()  { printf '%b\n' "${RED}[bootstrap]${NC} $*" >&2; }

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || { err "required command not found: $1"; exit 1; }
}

# ─── Go module dependencies ──────────────────────────────────────────────────
ensure_godeps() {
  local mod
  for mod in backend/core backend/exif backend/ocr tools/embeddings-builder; do
    [[ -f "$ROOT_DIR/$mod/go.mod" ]] || continue
    info "go mod download · $mod"
    ( cd "$ROOT_DIR/$mod" && go mod download )
  done
}

# ─── webapp (npm) ────────────────────────────────────────────────────────────
ensure_webapp() {
  if [[ -d "$ROOT_DIR/webapp/node_modules/.bin" ]]; then
    info "webapp dependencies already installed"
    return 0
  fi
  require_cmd npm
  info "Installing webapp dependencies (npm ci) …"
  if [[ -f "$ROOT_DIR/webapp/package-lock.json" ]]; then
    ( cd "$ROOT_DIR/webapp" && npm ci )
  else
    ( cd "$ROOT_DIR/webapp" && npm install )
  fi
}

# ─── Python venv for pytest ──────────────────────────────────────────────────
ensure_pytest() {
  require_cmd python3

  if [[ ! -x "$PYTEST_VENV/bin/python" ]]; then
    info "Creating Python virtualenv at tests/ocr/integration/.venv"
    python3 -m venv "$PYTEST_VENV"
  fi

  info "Installing pytest requirements …"
  "$PYTEST_VENV/bin/python" -m pip install --quiet --upgrade pip
  "$PYTEST_VENV/bin/python" -m pip install --quiet -r "$PYTEST_DIR/requirements.txt"

  info "pytest ready · $("$PYTEST_VENV/bin/python" -m pytest --version 2>/dev/null | head -n1)"
}

# ─── ExifTool (no-root local install) ────────────────────────────────────────
ensure_exiftool() {
  if command -v exiftool >/dev/null 2>&1; then
    info "exiftool already available · $(exiftool -ver 2>/dev/null)"
    return 0
  fi

  if [[ -x "$TOOLS_BIN/exiftool" ]]; then
    info "exiftool already bootstrapped · $("$TOOLS_BIN/exiftool" -ver 2>/dev/null)"
    return 0
  fi

  require_cmd curl
  require_cmd tar
  require_cmd perl

  mkdir -p "$TOOLS_DIR" "$TOOLS_BIN"

  info "Resolving the latest ExifTool release …"
  local tag
  tag="$(curl -fsSL --max-time 30 https://api.github.com/repos/exiftool/exiftool/tags \
    | grep -oE '"name": *"[0-9]+\.[0-9]+"' \
    | head -n1 | grep -oE '[0-9]+\.[0-9]+' || true)"

  # Fallback: scrape the highest version mentioned on exiftool.org
  if [[ -z "$tag" ]]; then
    tag="$(curl -fsS --max-time 30 https://exiftool.org/ \
      | grep -oE 'Image-ExifTool-[0-9]+\.[0-9]+' \
      | grep -oE '[0-9]+\.[0-9]+' | sort -V | tail -n1 || true)"
  fi

  if [[ -z "$tag" ]]; then
    err "Could not determine the ExifTool version (network blocked?)"
    err "Install exiftool manually, or download it from https://exiftool.org/"
    exit 1
  fi

  info "Downloading ExifTool $tag (GitHub release tarball) …"
  curl -fsSL --max-time 120 \
    "https://github.com/exiftool/exiftool/archive/refs/tags/$tag.tar.gz" \
    -o "$TOOLS_DIR/et.tar.gz"
  tar -xzf "$TOOLS_DIR/et.tar.gz" -C "$TOOLS_DIR"
  rm -f "$TOOLS_DIR/et.tar.gz"

  ln -sfn "$TOOLS_DIR/exiftool-$tag" "$TOOLS_DIR/exiftool-dist"

  # Wrapper (not a symlink) so the Perl script resolves its own lib/ directory.
  # LC_ALL=C silences perl's locale warning on this container.
  cat >"$TOOLS_BIN/exiftool" <<'WRAPPER'
#!/bin/sh
export LC_ALL=C LANG=C
dir="$(cd "$(dirname "$0")/.." && pwd)"
exec perl "$dir/exiftool-dist/exiftool" "$@"
WRAPPER
  chmod +x "$TOOLS_BIN/exiftool"

  info "ExifTool installed · $("$TOOLS_BIN/exiftool" -ver 2>/dev/null)"
  warn "Add it to your PATH for ad-hoc use: export PATH=\"$TOOLS_BIN:\$PATH\""
}

# ─── Main ────────────────────────────────────────────────────────────────────
main() {
  local target="${1:-all}"
  case "$target" in
    all)      ensure_godeps; ensure_webapp; ensure_pytest; ensure_exiftool ;;
    godeps)   ensure_godeps ;;
    webapp)   ensure_webapp ;;
    pytest)   ensure_pytest ;;
    exiftool) ensure_exiftool ;;
    -h|--help|help)
      sed -n '2,15p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      ;;
    *)
      err "Unknown target: $target (expected: all|godeps|webapp|pytest|exiftool)"
      exit 2
      ;;
  esac
  info "Done."
}

main "$@"
