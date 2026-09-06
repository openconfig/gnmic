#!/bin/sh
# Generate tool-native version files from the shared settings, or check for drift.
set -eu

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
. "$project_dir/versions.env"

case "${1:-}" in
    "")
        printf '%s\n' "$PYTHON_VERSION" > "$project_dir/.python-version"
        ;;
    --check)
        if ! printf '%s\n' "$PYTHON_VERSION" | cmp -s "$project_dir/.python-version" -; then
            echo '.python-version is out of sync with versions.env. Run make sync-versions and commit .python-version.' >&2
            exit 1
        fi
        ;;
    *)
        echo 'Usage: sh scripts/sync-versions.sh [--check]' >&2
        exit 2
        ;;
esac
