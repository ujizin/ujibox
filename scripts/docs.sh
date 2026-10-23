#!/bin/sh
# Runs Zensical for the docs in a local virtual environment.
#
#   scripts/docs.sh          serve the docs at http://localhost:8000
#   scripts/docs.sh build    write the site to site/
#   scripts/docs.sh <args>   pass other arguments to zensical
#
# The first run creates .venv-docs and installs Zensical in it. Zensical
# needs Python 3.11 or later. Set PYTHON to use a specific interpreter.
set -eu

ZENSICAL_VERSION="0.0.72"
ROOT=$(cd "$(dirname "$0")/.." && pwd)
VENV="$ROOT/.venv-docs"

find_python() {
    for p in "${PYTHON:-}" python3.14 python3.13 python3.12 python3.11 python3; do
        [ -n "$p" ] || continue
        command -v "$p" >/dev/null 2>&1 || continue
        if "$p" -c 'import sys; sys.exit(0 if sys.version_info >= (3, 11) else 1)' 2>/dev/null; then
            echo "$p"
            return 0
        fi
    done
    return 1
}

if [ ! -x "$VENV/bin/zensical" ] || ! "$VENV/bin/zensical" --version 2>/dev/null | grep -q "$ZENSICAL_VERSION"; then
    if ! PY=$(find_python); then
        echo "Zensical needs Python 3.11 or later. Install it (brew install python) or set PYTHON." >&2
        exit 1
    fi
    echo "Installing Zensical $ZENSICAL_VERSION in .venv-docs with $PY" >&2
    rm -rf "$VENV"
    "$PY" -m venv "$VENV"
    "$VENV/bin/pip" install --quiet --disable-pip-version-check "zensical==$ZENSICAL_VERSION"
fi

cd "$ROOT"
if [ $# -eq 0 ]; then
    set -- serve
fi
case "$1" in
    build) shift; exec "$VENV/bin/zensical" build --clean "$@" ;;
    *) exec "$VENV/bin/zensical" "$@" ;;
esac
