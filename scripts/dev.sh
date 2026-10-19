#!/usr/bin/env bash
#
# dev.sh — host-native process manager for the Flashbacks development stack.
#
# Runs core / exif / ocr (Go binaries) and webapp (Vite) directly inside the
# IDE container — no Docker, no docker-compose. Configuration is layered: the
# repository `.env` base profile (see .env.example) is loaded first, then the
# optional FLASHBACKS_ENV_FILE overlay (the test targets point it at test.env).
# Relative paths are resolved from the repository root.
#
# Usage:
#   scripts/dev.sh build   [service ...]   # compile core/exif/ocr binaries
#   scripts/dev.sh start   [service ...]   # build + start in background
#   scripts/dev.sh stop    [service ...]   # stop background services
#   scripts/dev.sh restart [service ...]   # stop then start
#   scripts/dev.sh status  [service ...]   # show PID / state / URL
#   scripts/dev.sh logs    [service ...]   # tail -f logs (all by default)
#   scripts/dev.sh run      service        # run one service in the foreground
#
# Services: core exif ocr webapp          (default for start/stop: all)
# Runtime state lives under .run/ (git-ignored): bin/, logs/, <svc>.pid

set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_DIR="$ROOT_DIR/.run"
BIN_DIR="$RUN_DIR/bin"
LOG_DIR="$RUN_DIR/logs"
# Base profile exported to every service: the repository `.env` is the single
# source of shared configuration. FLASHBACKS_ENV_FILE adds an optional overlay
# loaded afterwards (the integration-test targets point it at test.env, which
# only carries the values that differ from `.env`).
BASE_ENV_FILE="${FLASHBACKS_BASE_ENV_FILE:-$ROOT_DIR/.env}"
OVERLAY_ENV_FILE="${FLASHBACKS_ENV_FILE:-}"
# Accept relative paths regardless of the caller's working directory.
[[ "$BASE_ENV_FILE" = /* ]] || BASE_ENV_FILE="$ROOT_DIR/$BASE_ENV_FILE"
if [[ -n "$OVERLAY_ENV_FILE" && "$OVERLAY_ENV_FILE" != /* ]]; then
  OVERLAY_ENV_FILE="$ROOT_DIR/$OVERLAY_ENV_FILE"
fi

ALL_SERVICES=(registry core exif ocr webapp)

# ─── Colours / logging ───────────────────────────────────────────────────────
GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; NC='\033[0m'
info() { printf '%b\n' "${GREEN}[dev]${NC} $*"; }
warn() { printf '%b\n' "${YELLOW}[dev]${NC} $*"; }
err()  { printf '%b\n' "${RED}[dev]${NC} $*" >&2; }

usage() {
  sed -n '2,19p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

# ─── Environment ─────────────────────────────────────────────────────────────
# Source one env file and export every variable it defines.
load_profile() {
  local file="$1" label="$2"
  [[ -f "$file" ]] || return 1
  set -a
  # shellcheck disable=SC1090
  . "$file"
  set +a
  info "Loaded $label environment from ${file#"$ROOT_DIR"/}"
  return 0
}

load_env() {
  # Base profile first, then the optional test overlay on top of it. Layering
  # keeps `.env` the single source of truth: test.env only carries differences.
  if ! load_profile "$BASE_ENV_FILE" "base"; then
    warn "No .env found — run 'cp .env.example .env' and set DB credentials"
  fi
  if [[ -n "$OVERLAY_ENV_FILE" ]] && ! load_profile "$OVERLAY_ENV_FILE" "overlay"; then
    # An explicit profile was requested — fail loudly instead of silently
    # falling back to defaults.
    err "Environment profile not found: $OVERLAY_ENV_FILE"
    exit 1
  fi

  # Expose project-local tools bootstrapped into .tools/ (exiftool, tesseract)
  # to every service — including the CGO/rpath settings the OCR build needs.
  # shellcheck source=scripts/toolenv.sh
  . "$ROOT_DIR/scripts/toolenv.sh"

  # Fall back to sane defaults so the stack starts even without a full .env.
  : "${DB_HOST:=postgreshost}"
  : "${DB_PORT:=5432}"
  : "${DB_USER:=flashbacks}"
  : "${DB_PASSWORD:=postgres}"
  : "${DB_NAME:=flashbacks}"
  : "${DB_SSLMODE:=disable}"
  : "${SERVER_HOST:=0.0.0.0}"
  : "${SERVER_PORT:=5170}"
  : "${CORS_ORIGINS:=*}"
  : "${BOOTSTRAP_LOGIN:=admin}"
  : "${BOOTSTRAP_PASSWORD:=admin}"
  : "${EXIF_SERVICE_URL:=http://localhost:5172}"
  : "${OCR_SERVICE_URL:=http://localhost:5174}"
  : "${EXIF_HOST:=0.0.0.0}"
  : "${EXIF_PORT:=5172}"
  : "${OCR_HOST:=0.0.0.0}"
  : "${OCR_PORT:=5174}"
  : "${REGISTRY_DATA_DIR:=$ROOT_DIR/.run/registry-data}"
  : "${REGISTRY_CLIENT_URL:=http://127.0.0.1:2379}"
  : "${REGISTRY_PEER_URL:=http://127.0.0.1:2380}"
  : "${REGISTRY_HTTP_PORT:=5176}"
  : "${REGISTRY_HTTP_ADDR:=:${REGISTRY_HTTP_PORT}}"
  : "${REGISTRY_MASTER_KEY:=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=}"
  : "${REGISTRY_URL:=http://localhost:${REGISTRY_HTTP_PORT}}"

  export DB_HOST DB_PORT DB_USER DB_PASSWORD DB_NAME DB_SSLMODE
  export SERVER_HOST SERVER_PORT CORS_ORIGINS BOOTSTRAP_LOGIN BOOTSTRAP_PASSWORD
  export EXIF_SERVICE_URL OCR_SERVICE_URL
  export EXIF_HOST EXIF_PORT OCR_HOST OCR_PORT
  export REGISTRY_DATA_DIR REGISTRY_CLIENT_URL REGISTRY_PEER_URL
  export REGISTRY_HTTP_PORT REGISTRY_HTTP_ADDR REGISTRY_MASTER_KEY REGISTRY_URL
}

# ─── Service metadata ────────────────────────────────────────────────────────
svc_dir() {
  case "$1" in
    core|exif|ocr) echo "backend/$1" ;;
    registry)      echo "backend/registry" ;;
    webapp)        echo "webapp" ;;
  esac
}

svc_port() {
  case "$1" in
    core)     echo "${SERVER_PORT}" ;;
    exif)     echo "${EXIF_PORT}" ;;
    ocr)      echo "${OCR_PORT}" ;;
    registry) echo "${REGISTRY_HTTP_PORT}" ;;
    webapp)   echo "5180" ;;
  esac
}

svc_health_path() {
  case "$1" in
    core)     echo "/api/auth/status" ;;
    exif)     echo "/exif/health" ;;
    ocr)      echo "/ocr/api/health" ;;
    registry) echo "/registry/v1/health" ;;
    webapp)   echo "/" ;;
  esac
}

# Human-facing base URL (with the service API prefix where applicable).
svc_public_url() {
  case "$1" in
    core)     echo "http://localhost:$(svc_port core)/api" ;;
    exif)     echo "http://localhost:$(svc_port exif)/exif" ;;
    ocr)      echo "http://localhost:$(svc_port ocr)/ocr/api" ;;
    registry) echo "http://localhost:$(svc_port registry)/registry/v1" ;;
    webapp)   echo "http://localhost:$(svc_port webapp)" ;;
  esac
}

pid_file() { echo "$RUN_DIR/$1.pid"; }

is_running() {
  local f pid
  f="$(pid_file "$1")"
  [[ -f "$f" ]] || return 1
  pid="$(cat "$f" 2>/dev/null || true)"
  [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null
}

# ─── Build ───────────────────────────────────────────────────────────────────
build_bin() {
  local svc="$1" dir="$2"
  mkdir -p "$BIN_DIR"
  info "Building $svc …"
  ( cd "$ROOT_DIR/$dir" && go build -o "$BIN_DIR/$svc" ./cmd/server )
}

build_service() {
  case "$1" in
    core|exif|ocr|registry) build_bin "$1" "$(svc_dir "$1")" ;;
    webapp)                 info "webapp is served by the Vite dev server (no build step)" ;;
  esac
}

# ─── Start / stop ────────────────────────────────────────────────────────────
start_one() {
  local svc="$1" dir log
  dir="$(svc_dir "$svc")"
  log="$LOG_DIR/$svc.log"

  if is_running "$svc"; then
    warn "$svc already running (pid $(cat "$(pid_file "$svc")"))"
    return 0
  fi

  mkdir -p "$RUN_DIR" "$BIN_DIR" "$LOG_DIR"

  local -a cmd
  case "$svc" in
    core|exif|ocr)
      build_bin "$svc" "$dir"
      cmd=( env "SERVICE_NAME=$svc" "SERVICE_ID=$svc-dev" "$BIN_DIR/$svc" )
      ;;
    registry)
      build_bin "$svc" "$dir"
      cmd=( "$BIN_DIR/registry" )
      ;;
    webapp)
      if [[ ! -d "$ROOT_DIR/webapp/node_modules" ]]; then
        err "webapp dependencies missing — run: make setup-webapp"
        return 1
      fi
      cmd=( npm run dev )
      ;;
  esac

  info "Starting $svc  ·  log: .run/logs/$svc.log"
  (
    cd "$ROOT_DIR/$dir"
    # setsid detaches the service into its own process group so the whole tree
    # (npm -> vite, go -> child) can be stopped with a single signal later.
    setsid "${cmd[@]}" >"$log" 2>&1 </dev/null &
    echo $! >"$(pid_file "$svc")"
  )
}

wait_ready() {
  local svc="$1" url i
  url="http://localhost:$(svc_port "$svc")$(svc_health_path "$svc")"

  for ((i = 0; i < 120; i++)); do
    if ! is_running "$svc"; then
      err "$svc exited during startup — check .run/logs/$svc.log"
      return 1
    fi
    if curl -fsS -o /dev/null --max-time 2 "$url" 2>/dev/null; then
      info "$svc is up  ·  $(svc_public_url "$svc")"
      return 0
    fi
    sleep 0.5
  done

  warn "$svc did not pass its health check yet (still running) — check logs"
  return 1
}

stop_one() {
  local svc="$1" f pid i
  f="$(pid_file "$svc")"

  if [[ ! -f "$f" ]]; then
    info "$svc not running"
    return 0
  fi

  pid="$(cat "$f" 2>/dev/null || true)"
  if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
    info "Stopping $svc (pid $pid)"
    kill -TERM "-$pid" 2>/dev/null || kill -TERM "$pid" 2>/dev/null || true
    for ((i = 0; i < 20; i++)); do
      kill -0 "$pid" 2>/dev/null || break
      sleep 0.5
    done
    if kill -0 "$pid" 2>/dev/null; then
      warn "$svc ignored SIGTERM — sending SIGKILL"
      kill -KILL "-$pid" 2>/dev/null || kill -KILL "$pid" 2>/dev/null || true
    fi
  else
    info "$svc not running (stale pid file)"
  fi

  rm -f "$f"
}

# ─── Status / logs / foreground ──────────────────────────────────────────────
status() {
  printf '%-8s %-10s %-8s %s\n' "SERVICE" "STATE" "PID" "URL"
  local svc state pid
  for svc in "$@"; do
    if is_running "$svc"; then
      state="running"; pid="$(cat "$(pid_file "$svc")")"
    else
      state="stopped"; pid="-"
    fi
    printf '%-8s %-10s %-8s %s\n' "$svc" "$state" "$pid" "$(svc_public_url "$svc")"
  done
}

logs() {
  local -a files=()
  local svc
  for svc in "$@"; do
    files+=("$LOG_DIR/$svc.log")
  done
  if [[ ! -e "${files[0]}" ]]; then
    err "no logs found — start a service first (make start)"
    exit 1
  fi
  tail -n 40 -f "${files[@]}"
}

run_foreground() {
  local svc="$1" dir
  dir="$(svc_dir "$svc")"
  case "$svc" in
    core|exif|ocr|registry)
      build_bin "$svc" "$dir"
      info "Running $svc in the foreground (Ctrl-C to stop)"
      cd "$ROOT_DIR/$dir"
      if [[ "$svc" == registry ]]; then
        exec "$BIN_DIR/registry"
      fi
      exec env "SERVICE_NAME=$svc" "SERVICE_ID=$svc-dev" "$BIN_DIR/$svc"
      ;;
    webapp)
      info "Running webapp in the foreground (Ctrl-C to stop)"
      cd "$ROOT_DIR/$dir"
      exec npm run dev
      ;;
  esac
}

# ─── Argument parsing ────────────────────────────────────────────────────────
declare -a TARGETS
parse_targets() {
  TARGETS=()
  if [[ $# -eq 0 ]]; then
    TARGETS=("${ALL_SERVICES[@]}")
    return 0
  fi
  local s
  for s in "$@"; do
    if [[ "$s" == "all" ]]; then
      TARGETS=("${ALL_SERVICES[@]}")
      return 0
    fi
    case " ${ALL_SERVICES[*]} " in
      *" $s "*) TARGETS+=("$s") ;;
      *) err "Unknown service: $s (expected: all or: ${ALL_SERVICES[*]})"; exit 2 ;;
    esac
  done
}

main() {
  local action="${1:-}"
  [[ $# -gt 0 ]] && shift

  case "$action" in
    build)
      load_env
      parse_targets "$@"
      for s in "${TARGETS[@]}"; do build_service "$s"; done
      ;;
    start|up)
      load_env
      parse_targets "$@"
      if printf '%s\n' "${TARGETS[@]}" | grep -qx registry; then
        start_one registry
        wait_ready registry || true
      fi
      for s in "${TARGETS[@]}"; do
        [[ "$s" == registry ]] && continue
        start_one "$s"
      done
      for s in "${TARGETS[@]}"; do
        [[ "$s" == registry ]] && continue
        wait_ready "$s" || true
      done
      ;;
    stop|down)
      load_env
      parse_targets "$@"
      for s in "${TARGETS[@]}"; do stop_one "$s"; done
      ;;
    restart)
      load_env
      parse_targets "$@"
      for s in "${TARGETS[@]}"; do stop_one "$s"; done
      if printf '%s\n' "${TARGETS[@]}" | grep -qx registry; then
        start_one registry
        wait_ready registry || true
      fi
      for s in "${TARGETS[@]}"; do
        [[ "$s" == registry ]] && continue
        start_one "$s"
      done
      for s in "${TARGETS[@]}"; do
        [[ "$s" == registry ]] && continue
        wait_ready "$s" || true
      done
      ;;
    status|ps)
      load_env
      parse_targets "$@"
      status "${TARGETS[@]}"
      ;;
    logs)
      load_env
      parse_targets "$@"
      logs "${TARGETS[@]}"
      ;;
    run)
      load_env
      if [[ $# -ne 1 ]]; then err "'run' requires exactly one service"; usage; exit 2; fi
      case " ${ALL_SERVICES[*]} " in
        *" $1 "*) ;;
        *) err "Unknown service: $1"; exit 2 ;;
      esac
      run_foreground "$1"
      ;;
    ""|-h|--help|help)
      usage
      [[ -z "$action" ]] && exit 1 || exit 0
      ;;
    *)
      err "Unknown action: $action"
      usage
      exit 2
      ;;
  esac
}

main "$@"
