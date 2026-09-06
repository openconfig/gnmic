#!/bin/sh
# Run uv with all downloaded tooling and dependencies kept inside the checkout.
set -eu

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_dir"

uv_version=0.12.10
tools_dir="$project_dir/.tools/docs"
uv_bin="$tools_dir/bin/uv"

export UV_CACHE_DIR="$tools_dir/cache"
export UV_PYTHON_INSTALL_DIR="$tools_dir/python"
export UV_PYTHON_BIN_DIR="$tools_dir/bin"
export UV_PROJECT_ENVIRONMENT="$tools_dir/venv"

installed_version=$("$uv_bin" --version 2>/dev/null || true)
case "$installed_version" in
    "uv $uv_version"|"uv $uv_version "*) ;;
    *)
        mkdir -p "$tools_dir"
        installer=$(mktemp "$tools_dir/uv-install.XXXXXX")
        trap 'rm -f "$installer"' EXIT
        trap 'exit 1' HUP INT TERM
        installer_url="https://astral.sh/uv/$uv_version/install.sh"
        if command -v curl >/dev/null 2>&1; then
            curl -fsSL "$installer_url" -o "$installer"
        elif command -v wget >/dev/null 2>&1; then
            wget -q "$installer_url" -O "$installer"
        else
            echo "Install curl or wget to download uv." >&2
            exit 1
        fi
        # Astral's installer detects the OS and architecture. Unmanaged installation
        # avoids changing shell profiles, PATH, or the user's existing uv install.
        UV_UNMANAGED_INSTALL="$tools_dir/bin" sh "$installer"
        rm -f "$installer"
        trap - EXIT HUP INT TERM
        ;;
esac

exec "$uv_bin" "$@"
