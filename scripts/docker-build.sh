#!/bin/sh
# Build the repository's Dockerfile with the shared toolchain and base image.
# Pass Docker build options as arguments; the context is always the repository.
set -eu

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
. "$project_dir/versions.env"

exec docker build \
    --build-arg "GO_VERSION=$GO_VERSION" \
    --build-arg "ALPINE_VERSION=$ALPINE_VERSION" \
    "$@" "$project_dir"
