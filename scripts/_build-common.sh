#!/usr/bin/env bash

# Shared build helpers. This file is sourced by the platform entry points and
# is not intended to be run directly.

set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"
BUILD_SKIP_TESTS=0
FRONTEND_BUILT=0

log() {
  printf '\n==> %s\n' "$*"
}

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "$2"
}

version_at_least() {
  local actual_major actual_minor actual_patch required_major required_minor required_patch
  IFS=. read -r actual_major actual_minor actual_patch <<<"${1%%-*}"
  IFS=. read -r required_major required_minor required_patch <<<"${2%%-*}"
  actual_minor="${actual_minor:-0}"
  actual_patch="${actual_patch:-0}"
  required_minor="${required_minor:-0}"
  required_patch="${required_patch:-0}"

  (( actual_major > required_major )) ||
    (( actual_major == required_major && actual_minor > required_minor )) ||
    (( actual_major == required_major && actual_minor == required_minor && actual_patch >= required_patch ))
}

check_toolchain() {
  require_command go "Go 1.25 or newer is required: https://go.dev/dl/"
  require_command npm "Node.js 20 or newer (including npm) is required: https://nodejs.org/"

  local required_go actual_go actual_node
  required_go="$(awk '$1 == "go" { print $2; exit }' "${REPO_ROOT}/go.mod")"
  actual_go="$(go env GOVERSION)"
  actual_go="${actual_go#go}"
  actual_node="$(node --version 2>/dev/null || true)"
  actual_node="${actual_node#v}"

  [[ -n "${actual_node}" ]] || fail "Node.js 20 or newer is required: https://nodejs.org/"
  version_at_least "${actual_go}" "${required_go}" || fail "Go ${required_go}+ is required; found ${actual_go}."
  version_at_least "${actual_node}" "20.0.0" || fail "Node.js 20+ is required; found ${actual_node}."

  log "Toolchain: Go ${actual_go}, Node ${actual_node}, npm $(npm --version)"
}

build_frontend() {
  if [[ "${FRONTEND_BUILT}" == "1" ]]; then
    return
  fi
  log "Installing and building the frontend"
  (
    cd "${REPO_ROOT}/frontend"
    npm ci
    npm run build
  )
  FRONTEND_BUILT=1
}

run_checks() {
  if [[ "${BUILD_SKIP_TESTS}" == "1" ]]; then
    log "Skipping verification"
    return
  fi
  log "Running backend tests and frontend checks"
  build_frontend
  (
    cd "${REPO_ROOT}/frontend"
    npm run check
  )
  (
    cd "${REPO_ROOT}"
    go test ./...
  )
}

build_production_executable() {
  local output_path="$1"
  build_frontend
  log "Generating theme-aware application icons"
  (
    cd "${REPO_ROOT}"
    go run ./tools/theme-icon
  )
  mkdir -p "$(dirname -- "${output_path}")"
  (
    cd "${REPO_ROOT}"
    go build -tags production -trimpath -ldflags="-w -s" -o "${output_path}" .
  )
}

run_as_root() {
  if [[ "${EUID}" == "0" ]]; then
    "$@"
    return
  fi
  require_command sudo "sudo is required to install native build dependencies."
  sudo "$@"
}

check_linux_native_dependencies() {
  require_command gcc "A C compiler is required. Install the native packages shown by this script."
  require_command pkg-config "pkg-config is required. Install the native packages shown by this script."

  local missing=()
  local module
  for module in gtk4 webkitgtk-6.0 x11; do
    if ! pkg-config --exists "${module}"; then
      missing+=("${module}")
    fi
  done
  if (( ${#missing[@]} > 0 )); then
    fail "Missing native modules: ${missing[*]}. Re-run with --install-deps."
  fi

  log "Native libraries: GTK $(pkg-config --modversion gtk4), WebKitGTK $(pkg-config --modversion webkitgtk-6.0)"
}

assert_uname() {
  local expected="$1"
  local actual
  actual="$(uname -s)"
  [[ "${actual}" == "${expected}" ]] || fail "This script builds on ${expected}; current platform is ${actual}."
}
