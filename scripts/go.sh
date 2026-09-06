#!/bin/sh
# Use the shared Go toolchain for builds and tests (requires Go 1.21+ to bootstrap).
set -eu

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
. "$project_dir/versions.env"

export GOTOOLCHAIN="go$GO_VERSION"
exec go "$@"
