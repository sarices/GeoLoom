#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DIST_DIR="${ROOT_DIR}/dist"
PROJECT_NAME="geoloom"

require_command() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "错误：缺少依赖命令 '$cmd'" >&2
    exit 1
  fi
}

resolve_commit() {
  if git -C "$ROOT_DIR" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    git -C "$ROOT_DIR" rev-parse --short HEAD 2>/dev/null || echo "unknown"
  else
    echo "unknown"
  fi
}

require_command "go"
require_command "tar"
require_command "zip"

VERSION="${1:-${VERSION:-v0.2.5}}"
COMMIT="${2:-${COMMIT:-$(resolve_commit)}}"
BUILD_TIME="${3:-${BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}}"

TARGETS=(
  "darwin/amd64"
  "darwin/arm64"
  "linux/amd64"
  "linux/arm64"
  "windows/amd64"
  "windows/arm64"
)

if [[ ! -f "${ROOT_DIR}/configs/config.example.yaml" ]]; then
  echo "错误：缺少配置模板文件 ${ROOT_DIR}/configs/config.example.yaml" >&2
  exit 1
fi

if [[ ! -f "${ROOT_DIR}/README.md" ]]; then
  echo "错误：缺少 README.md" >&2
  exit 1
fi

if [[ ! -f "${ROOT_DIR}/CHANGELOG.md" ]]; then
  echo "错误：缺少 CHANGELOG.md" >&2
  exit 1
fi

rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"

echo "开始构建发布包"
echo "VERSION=${VERSION}"
echo "COMMIT=${COMMIT}"
echo "BUILD_TIME=${BUILD_TIME}"

for target in "${TARGETS[@]}"; do
  GOOS="${target%%/*}"
  GOARCH="${target##*/}"

  PKG_DIR_NAME="${PROJECT_NAME}_${VERSION}_${GOOS}_${GOARCH}"
  PKG_DIR="${DIST_DIR}/${PKG_DIR_NAME}"

  mkdir -p "${PKG_DIR}"

  BIN_NAME="${PROJECT_NAME}"
  if [[ "${GOOS}" == "windows" ]]; then
    BIN_NAME="${PROJECT_NAME}.exe"
  fi

  echo "[构建] ${GOOS}/${GOARCH}"
  CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" \
    go build \
      -trimpath \
      -ldflags "-X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME}" \
      -o "${PKG_DIR}/${BIN_NAME}" \
      "${ROOT_DIR}/cmd/geoloom"

  cp "${ROOT_DIR}/configs/config.example.yaml" "${PKG_DIR}/config.example.yaml"
  cp "${ROOT_DIR}/README.md" "${PKG_DIR}/README.md"
  cp "${ROOT_DIR}/CHANGELOG.md" "${PKG_DIR}/CHANGELOG.md"

  if [[ "${GOOS}" == "windows" ]]; then
    ARCHIVE="${DIST_DIR}/${PKG_DIR_NAME}.zip"
    (
      cd "${DIST_DIR}"
      zip -rq "${ARCHIVE}" "${PKG_DIR_NAME}"
    )
  else
    ARCHIVE="${DIST_DIR}/${PKG_DIR_NAME}.tar.gz"
    tar -C "${DIST_DIR}" -czf "${ARCHIVE}" "${PKG_DIR_NAME}"
  fi

done

echo "发布包构建完成，产物目录：${DIST_DIR}"
