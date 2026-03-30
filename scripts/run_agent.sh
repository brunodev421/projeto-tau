#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "${ROOT_DIR}/.venv/bin/activate"
cd "${ROOT_DIR}/agent-service"
uvicorn app.main:app --host 0.0.0.0 --port "${AGENT_PORT:-8090}"

