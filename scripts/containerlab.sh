#!/bin/sh
# Load test-image versions after sudo, which normally clears the environment.
set -eu

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
exec sudo sh -eu -c '
    . "$1/versions.env"
    shift
    exec clab "$@"
' sh "$project_dir" "$@"
