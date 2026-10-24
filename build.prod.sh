#!/usr/bin/env bash
set -euo pipefail

cd -- "$(dirname -- "${BASH_SOURCE[0]}")"
export DOCKER_BUILDKIT=1

VERSION=0.0.10
IMAGE=ghcr.io/teiddy0207/pipevid-backend:$VERSION

docker build -f Dockerfile -t $IMAGE . --progress=plain

docker push $IMAGE

# Keep the image and BuildKit cache for subsequent builds.
