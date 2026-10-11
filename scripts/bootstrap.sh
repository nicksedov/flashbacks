#!/usr/bin/env bash
#
# bootstrap.sh — provision host-native development prerequisites without root.
#
# The IDE container ships with Go, Node and Python, but exiftool and Tesseract
# are treated as project-local tools and there is no passwordless apt. This
# script fills that gap by installing ExifTool (a pure-Perl distribution) and
# Tesseract OCR (Ubuntu .deb packages unpacked without root) into .tools/, and
# by setting up the Python virtualenv used by the OCR pytest suite.
#
# Usage:
#   scripts/bootstrap.sh [all|godeps|webapp|pytest|exiftool|tesseract]
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

# ─── Tesseract OCR (no-root local install) ───────────────────────────────────
# The OCR service links against libtesseract / libLeptonica through CGO
# (gosseract). Instead of relying on system packages installed with apt + root,
# we fetch the Ubuntu .deb files and unpack them into .tools/tesseract/ — the
# same no-root pattern used for ExifTool above.
TESSERACT_DIR="$TOOLS_DIR/tesseract"
TESSERACT_PACKAGES=(
  tesseract-ocr
  libtesseract5
  libtesseract-dev
  liblept5
  libleptonica-dev
  tesseract-ocr-eng
  tesseract-ocr-rus
  tesseract-ocr-osd
  # Runtime dependencies of libtesseract / libleptonica and the CLI that are
  # not guaranteed to be present on a slimmed host image (Ubuntu 24.04 names).
  libwebpmux3
  libopenjp2-7
  libarchive13t64
  libicu74
)

# resolve_deb_filenames <Packages-file> <package>...
# Prints "<package> <Filename>" for every requested package found in the index.
resolve_deb_filenames() {
  local index="$1"; shift
  awk -v want="$*" '
    BEGIN { RS=""; FS="\n"; n=split(want, w, " ") }
    {
      pkg=""; fn=""
      for (i=1; i<=NF; i++) {
        if ($i ~ /^Package: /) pkg=substr($i, 10)
        else if ($i ~ /^Filename: /) fn=substr($i, 11)
      }
      for (j=1; j<=n; j++) if (pkg == w[j]) print pkg, fn
    }
  ' "$index"
}

ensure_tesseract() {
  local bin="$TESSERACT_DIR/usr/bin/tesseract"

  if [[ -x "$bin" ]]; then
    local lib
    lib="$(ls -d "$TESSERACT_DIR"/usr/lib/*-linux-gnu 2>/dev/null | head -n1 || true)"
    info "tesseract already bootstrapped · $(LD_LIBRARY_PATH="$lib" "$bin" --version 2>/dev/null | head -n1)"
    return 0
  fi

  require_cmd curl
  require_cmd dpkg-deb
  require_cmd awk
  require_cmd xz

  mkdir -p "$TESSERACT_DIR"

  # Install inside a subshell so the temp directory is always cleaned up.
  local tmp
  tmp="$(mktemp -d)"
  (
    trap 'rm -rf "$tmp"' EXIT
    _install_tesseract "$tmp"
  )
}

# _install_tesseract <tmp-dir> — does the actual fetch/unpack (run in a subshell).
_install_tesseract() {
  local tmp="$1"
  local bin="$TESSERACT_DIR/usr/bin/tesseract"

  local mirror="${TESSERACT_APT_MIRROR:-http://archive.ubuntu.com/ubuntu}"
  local codename
  codename="$(. /etc/os-release 2>/dev/null && echo "${VERSION_CODENAME:-noble}")"
  codename="${TESSERACT_SUITE:-${codename:-noble}}"
  local arch
  arch="$(dpkg --print-architecture 2>/dev/null || echo amd64)"

  # tesseract/leptonlib live in 'universe'; some runtime deps live in 'main'.
  info "Fetching the Ubuntu '$codename' package indexes (main + universe/$arch) …"
  : >"$tmp/Packages"
  local component
  for component in main universe; do
    if ! curl -fsSL --max-time 120 -o "$tmp/Packages-$component.xz" \
        "$mirror/dists/$codename/$component/binary-$arch/Packages.xz"; then
      err "Could not download the '$component' package index from $mirror"
      err "Check network access, or override TESSERACT_APT_MIRROR / TESSERACT_SUITE."
      return 1
    fi
    if ! xz -dc "$tmp/Packages-$component.xz" >>"$tmp/Packages" 2>/dev/null; then
      err "Failed to decompress the '$component' package index"
      return 1
    fi
  done

  local resolved=() line
  while IFS= read -r line; do
    [[ -n "$line" ]] && resolved+=("$line")
  done < <(resolve_deb_filenames "$tmp/Packages" "${TESSERACT_PACKAGES[@]}")

  if (( ${#resolved[@]} < ${#TESSERACT_PACKAGES[@]} )); then
    err "Could not resolve all Tesseract packages in the '$codename' package indexes"
    printf '  resolved: %s\n' "${resolved[*]:-<none>}" >&2
    return 1
  fi

  info "Downloading and unpacking ${#resolved[@]} packages into .tools/tesseract/ …"
  local entry name fn
  for entry in "${resolved[@]}"; do
    name="${entry%% *}"
    fn="${entry#* }"
    if ! curl -fsSL --max-time 180 -o "$tmp/$name.deb" "$mirror/$fn"; then
      err "Failed to download $name ($fn)"
      return 1
    fi
    if ! dpkg-deb -x "$tmp/$name.deb" "$TESSERACT_DIR"; then
      err "Failed to unpack $name"
      return 1
    fi
  done

  # Sanity check: the unpacked CLI must run against its own libs and tessdata.
  local lib data
  lib="$(ls -d "$TESSERACT_DIR"/usr/lib/*-linux-gnu 2>/dev/null | head -n1 || true)"
  data="$(ls -d "$TESSERACT_DIR"/usr/share/tesseract-ocr/*/tessdata 2>/dev/null | head -n1 || true)"
  if ! LD_LIBRARY_PATH="${lib}${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}" TESSDATA_PREFIX="$data" \
       "$bin" --version >/dev/null 2>&1; then
    err "Tesseract bootstrap verification failed: $bin --version"
    return 1
  fi

  info "Tesseract installed · $(LD_LIBRARY_PATH="$lib" "$bin" --version 2>/dev/null | head -n1)"
  info "Languages: $(LD_LIBRARY_PATH="$lib" TESSDATA_PREFIX="$data" "$bin" --list-langs 2>/dev/null | tail -n +2 | tr '\n' ' ')"
  warn "Project-local paths are exported by scripts/toolenv.sh (make / dev.sh handle it)"
}

# ─── Main ────────────────────────────────────────────────────────────────────
main() {
  local target="${1:-all}"
  case "$target" in
    all)       ensure_godeps; ensure_webapp; ensure_pytest; ensure_exiftool; ensure_tesseract ;;
    godeps)    ensure_godeps ;;
    webapp)    ensure_webapp ;;
    pytest)    ensure_pytest ;;
    exiftool)  ensure_exiftool ;;
    tesseract) ensure_tesseract ;;
    -h|--help|help)
      sed -n '2,13p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
      ;;
    *)
      err "Unknown target: $target (expected: all|godeps|webapp|pytest|exiftool|tesseract)"
      exit 2
      ;;
  esac
  info "Done."
}

main "$@"
