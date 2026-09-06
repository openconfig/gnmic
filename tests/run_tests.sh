#!/bin/bash

# Runs the test suite for every Go module in the repo.
#
# Usage:
#   ./tests/run_tests.sh          # coverage run (default; works with CGO_ENABLED=0)
#   ./tests/run_tests.sh --race   # race-detector run (requires CGO_ENABLED=1)

set -e

RACE=0
for arg in "$@"; do
    case "$arg" in
        --race) RACE=1 ;;
        *) echo "unknown argument: $arg" >&2; exit 2 ;;
    esac
done

if [ "$RACE" = "1" ]; then
    if [ "${CGO_ENABLED:-1}" = "0" ]; then
        echo "--race requires cgo; CGO_ENABLED=0 is set" >&2
        exit 2
    fi
    GOTEST_FLAGS=(-race -count=1 -v)
else
    GOTEST_FLAGS=(-cover -count=1 -v)
fi

function testmodule
{
    cd "$1"
    go test "${GOTEST_FLAGS[@]}" ./...
    cd "$SCRIPTPATH/.."
}

declare -a modules=("." "pkg/api" "pkg/cache")

SCRIPTPATH="$( cd -- "$(dirname "$0")" >/dev/null 2>&1 ; pwd -P )"

cd "$SCRIPTPATH/.."

for i in "${modules[@]}"
do
    echo "Running tests for module $i (race=$RACE)"
    testmodule "$i"
done
