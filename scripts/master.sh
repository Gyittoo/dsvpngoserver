#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_err()  { echo -e "${RED}[ERR ]${NC} $1"; }

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    log_err "Missing required command: $cmd"
    return 1
  fi
}

check_safety() {
  log_info "Running safety checks"
  require_cmd go
  require_cmd docker
  require_cmd bash

  local goversion
  goversion="$(go version | awk '{print $3}')"
  log_info "Go detected: $goversion"

  if [[ ! -f ".env" ]]; then
    if [[ -f ".env.example" ]]; then
      cp .env.example .env
      log_warn ".env not found. Created from .env.example. Please review secrets before production use."
    else
      log_err ".env.example missing. Cannot continue safely."
      exit 1
    fi
  fi

  if grep -q "replace-with-at-least-32-char-random-secret" .env; then
    log_warn "Default JWT_SECRET found. Please set strong secret before production."
  fi

  log_info "Safety checks complete"
}

install_deps() {
  log_info "Installing Go dependencies"
  go mod tidy
}

build_api() {
  log_info "Building API binary"
  go build -o bin/dsvpn-server ./cmd/server
}

start_stack() {
  log_info "Starting PostgreSQL + API with Docker Compose"
  docker compose up -d --build
  log_info "Stack started"
}

stop_stack() {
  log_info "Stopping stack"
  docker compose down
}

logs_stack() {
  docker compose logs -f --tail=200
}

usage() {
  cat <<EOF
Usage: ./scripts/master.sh <command>

Commands:
  setup        Run safety checks, install dependencies, and build API
  up           Start full Docker stack (db + api)
  down         Stop Docker stack
  logs         Follow Docker logs
  rebuild      Rebuild and restart Docker stack
  check        Run safety checks only
EOF
}

cmd="${1:-}"
case "$cmd" in
  setup)
    check_safety
    install_deps
    build_api
    ;;
  up)
    check_safety
    start_stack
    ;;
  down)
    stop_stack
    ;;
  logs)
    logs_stack
    ;;
  rebuild)
    check_safety
    docker compose down
    docker compose up -d --build
    ;;
  check)
    check_safety
    ;;
  *)
    usage
    exit 1
    ;;
esac
