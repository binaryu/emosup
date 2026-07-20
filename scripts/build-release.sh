#!/usr/bin/env bash
# Local release bundle builder (mirrors CI).
# Usage: ./scripts/build-release.sh [version] [amd64|arm64|all]
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${1:-dev}"
ARCH_ARG="${2:-amd64}"
OUT="${ROOT}/release"

build_one() {
  local goarch="$1"
  local asset="emosup-linux-${goarch}"
  local dest="${OUT}/${asset}"

  echo "==> building ${asset} (${VERSION})"
  rm -rf "$dest"
  mkdir -p "${dest}/frontend" "${dest}/data/downloads"

  (
    cd "${ROOT}/frontend"
    if [[ ! -d node_modules ]]; then npm ci; fi
    npm run build
  )
  cp -R "${ROOT}/frontend/dist/." "${dest}/frontend/"

  (
    cd "${ROOT}/backend"
    CGO_ENABLED=0 GOOS=linux GOARCH="${goarch}" \
      go build -trimpath \
        -ldflags="-s -w -X main.version=${VERSION}" \
        -o "${dest}/emosup-server" \
        ./cmd/server
  )

  cp "${ROOT}/backend/data/config.example.json" "${dest}/config.example.json"
  cp "${ROOT}/scripts/emosup.service" "${dest}/emosup.service"
  cp "${ROOT}/scripts/install.sh" "${dest}/install.sh"
  chmod +x "${dest}/emosup-server" "${dest}/install.sh"
  echo "${VERSION}" > "${dest}/VERSION"

  tar -C "${OUT}" -czf "${OUT}/${asset}.tar.gz" "${asset}"
  (cd "${OUT}" && sha256sum "${asset}.tar.gz" > "${asset}.tar.gz.sha256")
  echo "==> ${OUT}/${asset}.tar.gz"
}

mkdir -p "$OUT"
case "$ARCH_ARG" in
  all)
    build_one amd64
    build_one arm64
    ;;
  amd64|arm64)
    build_one "$ARCH_ARG"
    ;;
  *)
    echo "usage: $0 [version] [amd64|arm64|all]" >&2
    exit 1
    ;;
esac
