#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ ! -x "${ROOT_DIR}/.tooling/go/bin/go" ]]; then
  echo "Installing local Go toolchain..."
  mkdir -p "${ROOT_DIR}/.tooling"
  ARCHIVE="go1.26.1.darwin-arm64.tar.gz"
  curl -fsSLo "${ROOT_DIR}/.tooling/${ARCHIVE}" "https://go.dev/dl/${ARCHIVE}"
  tar -xzf "${ROOT_DIR}/.tooling/${ARCHIVE}" -C "${ROOT_DIR}/.tooling"
  rm "${ROOT_DIR}/.tooling/${ARCHIVE}"
fi

python3 -m venv "${ROOT_DIR}/.venv"
source "${ROOT_DIR}/.venv/bin/activate"
pip install --upgrade pip
pip install -e "${ROOT_DIR}/agent-service[dev]"

echo "Bootstrap complete."

