#!/usr/bin/env bash

set -Eeuo pipefail
source "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/_build-common.sh"

usage() {
  cat <<'EOF'
Usage: ./scripts/build-macos.sh [--skip-tests] [--skip-sign]

Builds bin/VibeDock.app for the current Mac architecture. The bundle is
ad-hoc signed by default. Set MACOS_SIGN_IDENTITY to use a Developer ID.
EOF
}

skip_sign=0
for argument in "$@"; do
  case "${argument}" in
    --skip-tests) BUILD_SKIP_TESTS=1 ;;
    --skip-sign) skip_sign=1 ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; fail "Unknown option: ${argument}" ;;
  esac
done

assert_uname Darwin
check_toolchain
run_checks

log "Building the macOS application bundle"
app_path="${REPO_ROOT}/bin/VibeDock.app"
executable_path="${REPO_ROOT}/bin/vibedock"
build_production_executable "${executable_path}"
mkdir -p "${app_path}/Contents/MacOS" "${app_path}/Contents/Resources"
cp "${executable_path}" "${app_path}/Contents/MacOS/vibedock"
cp "${REPO_ROOT}/build/darwin/Info.plist" "${app_path}/Contents/Info.plist"
cp "${REPO_ROOT}/build/darwin/AppIcon.icns" "${app_path}/Contents/Resources/AppIcon.icns"
[[ -d "${app_path}" ]] || fail "The build completed without producing ${app_path}."

if [[ "${skip_sign}" == "0" ]]; then
  require_command codesign "codesign is required to sign the macOS application bundle."
  sign_identity="${MACOS_SIGN_IDENTITY:--}"
  log "Signing VibeDock.app with identity '${sign_identity}'"
  if [[ "${sign_identity}" == "-" ]]; then
    codesign --force --deep --sign - --timestamp=none "${app_path}"
  else
    codesign --force --deep --sign "${sign_identity}" --timestamp "${app_path}"
  fi
  codesign --verify --deep --strict "${app_path}"
fi

log "Built ${app_path} ($(uname -m))"
