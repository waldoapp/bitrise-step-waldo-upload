#!/bin/bash

set -eu -o pipefail

SCRIPT_DIR=$(unset CDPATH && cd "${0%/*}" &>/dev/null && pwd)

export WALDO_UPLOAD_TOKEN="$upload_token"
export WALDO_VARIANT_NAME="$variant_name"

waldo_symbols=""

if [[ -z $symbols_path && $include_symbols == true ]]; then
    exec "$SCRIPT_DIR"/WaldoCLI.sh "$build_path" --include_symbols
else
    exec "$SCRIPT_DIR"/WaldoCLI.sh "$build_path" "$symbols_path"
fi
