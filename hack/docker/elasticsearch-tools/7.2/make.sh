#!/bin/bash
set -xeou pipefail

IMG=elasticsearch-tools
TAG="7.2"
PATCH="7.2.0"

docker pull "$DOCKER_REGISTRY/$IMG:$PATCH"

docker tag "$DOCKER_REGISTRY/$IMG:$PATCH" "$DOCKER_REGISTRY/$IMG:$TAG"
docker push "$DOCKER_REGISTRY/$IMG:$TAG"
