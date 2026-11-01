#!/usr/bin/env bash
source "$(dirname "$0")/docker-common.sh"
need git; local_docker
revision=$(sha)
[[ -z "$(git -C "$ROOT" status --porcelain --untracked-files=normal)" ]] || { echo 'Commit source before building an exact Doctor image.' >&2; exit 1; }
[[ "$(docker buildx inspect desktop-linux | awk '/^Driver:/ {print $2}')" == docker ]] || { echo 'Local Docker builder required.' >&2; exit 1; }
docker buildx build --builder desktop-linux --platform linux/amd64 --provenance=false --sbom=false --load --build-arg "GIT_SHA=$revision" -t "navidrome-family-doctor:$revision" -t navidrome-family-doctor:local -f "$ROOT/Dockerfile.family-doctor" "$ROOT"
echo "Local read-only Doctor companion built: navidrome-family-doctor:$revision"
