#!/usr/bin/env bash

set -Eeuo pipefail
source "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/_build-common.sh"

usage() {
  cat <<'EOF'
Usage: ./scripts/build-fedora.sh [--install-deps] [--skip-tests]

Builds bin/vibedock on Fedora. --install-deps installs only the native Wails
libraries; install Go 1.25+ and Node.js 20+ separately before running it.
EOF
}

install_deps=0
for argument in "$@"; do
  case "${argument}" in
    --install-deps) install_deps=1 ;;
    --skip-tests) BUILD_SKIP_TESTS=1 ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; fail "Unknown option: ${argument}" ;;
  esac
done

assert_uname Linux
require_command dnf "dnf was not found; use this script on Fedora."

if [[ "${install_deps}" == "1" ]]; then
  log "Installing Fedora native build dependencies"
  run_as_root dnf install -y gcc-c++ pkgconf-pkg-config gtk4-devel webkitgtk6.0-devel libX11-devel
fi

check_toolchain
check_linux_native_dependencies
run_checks

log "Building the Fedora executable"
output_path="${REPO_ROOT}/bin/vibedock"
build_production_executable "${output_path}"
[[ -x "${output_path}" ]] || fail "The build completed without producing ${output_path}."
log "Built ${output_path} ($(uname -m))"
