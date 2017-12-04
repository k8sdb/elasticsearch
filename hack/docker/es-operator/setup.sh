#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

GOPATH=$(go env GOPATH)
SRC=$GOPATH/src
BIN=$GOPATH/bin
ROOT=$GOPATH
REPO_ROOT=$GOPATH/src/github.com/kubedb/elasticsearch

source "$REPO_ROOT/hack/libbuild/common/kubedb_image.sh"

APPSCODE_ENV=${APPSCODE_ENV:-dev}
IMG=es-operator

DIST=$GOPATH/src/github.com/kubedb/elasticsearch/dist
mkdir -p $DIST
if [ -f "$DIST/.tag" ]; then
	export $(cat $DIST/.tag | xargs)
fi

clean() {
    pushd $REPO_ROOT/hack/docker/es-operator
    rm -f es-operator Dockerfile
    popd
}

build_binary() {
    pushd $REPO_ROOT
    ./hack/builddeps.sh
    ./hack/make.py build es-operator
    detect_tag $DIST/.tag
    popd
}

build_docker() {
    pushd $REPO_ROOT/hack/docker/es-operator
    cp $DIST/es-operator/es-operator-alpine-amd64 es-operator
    chmod 755 es-operator

    cat >Dockerfile <<EOL
FROM alpine

RUN set -x \
  && apk update \
  && apk add ca-certificates \
  && rm -rf /var/cache/apk/*

COPY es-operator /es-operator

USER nobody:nobody
ENTRYPOINT ["/es-operator"]
EOL
    local cmd="docker build -t kubedb/$IMG:$TAG ."
    echo $cmd; $cmd

    rm es-operator Dockerfile
    popd
}

build() {
    build_binary
    build_docker
}

docker_push() {
    if [ "$APPSCODE_ENV" = "prod" ]; then
        echo "Nothing to do in prod env. Are you trying to 'release' binaries to prod?"
        exit 1
    fi
    if [ "$TAG_STRATEGY" = "git_tag" ]; then
        echo "Are you trying to 'release' binaries to prod?"
        exit 1
    fi
    hub_canary
}

docker_release() {
    if [ "$APPSCODE_ENV" != "prod" ]; then
        echo "'release' only works in PROD env."
        exit 1
    fi
    if [ "$TAG_STRATEGY" != "git_tag" ]; then
        echo "'apply_tag' to release binaries and/or docker images."
        exit 1
    fi
    hub_up
}

source_repo $@
