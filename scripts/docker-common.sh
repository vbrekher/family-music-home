#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
need() { command -v "$1" >/dev/null || { echo "Required dependency: $1" >&2; exit 1; }; }
local_docker() {
  need docker; need grep
  [[ -z "${CI:-}${GITHUB_ACTIONS:-}" ]] || { echo 'Cloud/CI builds are prohibited.' >&2; exit 1; }
  export DOCKER_CONTEXT=desktop-linux
  [[ -z "${DOCKER_HOST:-}" ]] || { echo "Unset DOCKER_HOST; desktop-linux context is required." >&2; exit 1; }
  local endpoint
  endpoint=$(docker context inspect "$(docker context show)" --format '{{.Endpoints.docker.Host}}')
  [[ "${DOCKER_HOST:-$endpoint}" == unix://* ]] || { echo 'Local Docker Desktop Unix socket required.' >&2; exit 1; }
  docker info --format '{{.OperatingSystem}}' | grep -qi 'Docker Desktop' || { echo 'Docker Desktop required.' >&2; exit 1; }
}
sha() { git -C "$ROOT" rev-parse HEAD; }
