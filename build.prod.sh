#!/usr/bin/env bash
set -e

VERSION=0.0.3
IMAGE=ghcr.io/teiddy0207/pipevid-backend:$VERSION

docker build -f Dockerfile -t $IMAGE . --progress=plain

docker push $IMAGE

docker image rm $IMAGE
