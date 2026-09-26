#!/bin/bash
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ ! -x "$DIR/bot_bin" ]; then
    echo "bot_bin is missing; submit a prebuilt executable in the package" >&2
    exit 1
fi
exec "$DIR/bot_bin"
