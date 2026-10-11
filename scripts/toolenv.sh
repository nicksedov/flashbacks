#!/usr/bin/env bash
#
# toolenv.sh — export the environment for tools bootstrapped into .tools/ (no root).
#
# Source it from other scripts (or from an interactive shell):
#
#   . scripts/toolenv.sh
#
# It is a no-op for tool trees that are not present, so it is always safe to
# source. It is idempotent: re-sourcing never duplicates PATH / LD_LIBRARY_PATH
# entries nor CGO flags.
#
# Exposes:
#   • .tools/bin            → PATH                         (e.g. exiftool wrapper)
#   • .tools/tesseract/usr  → PATH, LD_LIBRARY_PATH,       (Tesseract CLI + libtesseract)
#                             CGO_CFLAGS, CGO_LDFLAGS,
#                             TESSDATA_PREFIX
#
# CGO_*FLAGS let `go build` link the OCR service (gosseract, CGO) against the
# project-local libtesseract/libleptonica and embed an rpath, so the compiled
# binary loads the local libraries at runtime too — independent of any system
# install.

# Resolve the repository root from this file's location (works when sourced).
_toolenv_root="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")/.." && pwd)"
_toolenv_tools="$_toolenv_root/.tools"

# ─── ExifTool (and any other .tools/bin wrappers) ────────────────────────────
if [ -d "$_toolenv_tools/bin" ]; then
  case ":${PATH:-}:" in
    *":$_toolenv_tools/bin:"*) ;;
    *) PATH="$_toolenv_tools/bin:${PATH:-}" ;;
  esac
fi

# ─── Project-local Tesseract ─────────────────────────────────────────────────
_toolenv_tess="$_toolenv_tools/tesseract"
if [ -d "$_toolenv_tess/usr" ]; then
  _toolenv_tess_bin="$_toolenv_tess/usr/bin"
  _toolenv_tess_inc="$_toolenv_tess/usr/include"
  _toolenv_tess_data=""

  # The library directory is architecture-specific (e.g. usr/lib/x86_64-linux-gnu).
  _toolenv_tess_lib="$(ls -d "$_toolenv_tess"/usr/lib/*-linux-gnu 2>/dev/null | head -n1 || true)"

  # tessdata lives under a versioned directory: usr/share/tesseract-ocr/<major>/tessdata.
  for _toolenv_d in "$_toolenv_tess"/usr/share/tesseract-ocr/*/tessdata; do
    if [ -d "$_toolenv_d" ]; then _toolenv_tess_data="$_toolenv_d"; fi
  done
  unset _toolenv_d

  if [ -d "$_toolenv_tess_bin" ]; then
    case ":${PATH:-}:" in
      *":$_toolenv_tess_bin:"*) ;;
      *) PATH="$_toolenv_tess_bin:${PATH:-}" ;;
    esac
  fi

  if [ -n "$_toolenv_tess_lib" ] && [ -d "$_toolenv_tess_lib" ]; then
    case ":${LD_LIBRARY_PATH:-}:" in
      *":$_toolenv_tess_lib:"*) ;;
      *) LD_LIBRARY_PATH="$_toolenv_tess_lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}" ;;
    esac
    # gosseract's C++ bridge (tessbridge.cpp) is compiled by the C++ compiler,
    # so the include dir must be on CGO_CPPFLAGS / CGO_CXXFLAGS — CGO_CFLAGS
    # alone does not reach it. Set all three to be safe.
    case "${CGO_CFLAGS:-}" in
      *"$_toolenv_tess_inc"*) ;;
      *) CGO_CFLAGS="-I$_toolenv_tess_inc ${CGO_CFLAGS:-}" ;;
    esac
    case "${CGO_CPPFLAGS:-}" in
      *"$_toolenv_tess_inc"*) ;;
      *) CGO_CPPFLAGS="-I$_toolenv_tess_inc ${CGO_CPPFLAGS:-}" ;;
    esac
    case "${CGO_CXXFLAGS:-}" in
      *"$_toolenv_tess_inc"*) ;;
      *) CGO_CXXFLAGS="-I$_toolenv_tess_inc ${CGO_CXXFLAGS:-}" ;;
    esac
    # rpath embeds the local libdir into the binary; rpath-link lets the linker
    # resolve the transitive dependencies of libtesseract/libleptonica.
    # --disable-new-dtags forces DT_RPATH (instead of DT_RUNPATH) so the loader
    # also applies it to those transitive dependencies at runtime.
    case "${CGO_LDFLAGS:-}" in
      *"$_toolenv_tess_lib"*) ;;
      *) CGO_LDFLAGS="-L$_toolenv_tess_lib -Wl,--disable-new-dtags -Wl,-rpath,$_toolenv_tess_lib -Wl,-rpath-link,$_toolenv_tess_lib ${CGO_LDFLAGS:-}" ;;
    esac
    export LD_LIBRARY_PATH CGO_CFLAGS CGO_CPPFLAGS CGO_CXXFLAGS CGO_LDFLAGS
  fi

  if [ -n "$_toolenv_tess_data" ]; then
    export TESSDATA_PREFIX="$_toolenv_tess_data"
  fi
fi

export PATH

unset _toolenv_root _toolenv_tools _toolenv_tess _toolenv_tess_bin \
      _toolenv_tess_lib _toolenv_tess_inc _toolenv_tess_data
