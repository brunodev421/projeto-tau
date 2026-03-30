#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "${ROOT_DIR}/.venv/bin/activate" 2>/dev/null || true
export PATH="${ROOT_DIR}/.tooling/go/bin:${PATH}"
cd "${ROOT_DIR}/bfa-go"
go run ./cmd/server

