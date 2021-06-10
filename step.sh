#!/bin/bash

set -eu -o pipefail

SCRIPT_DIR=$(unset CDPATH && cd "${0%/*}" &>/dev/null && pwd)

export WALDO_UPLOAD_TOKEN="$upload_token"
export WALDO_VARIANT_NAME="$variant_name"

waldo_build_flavor=""
waldo_build_suffix=${build_path##*.}

case $waldo_build_suffix in
    apk)     waldo_build_flavor="Android" ;;
    app|ipa) waldo_build_flavor="iOS" ;;
    *)       waldo_build_flavor="unknown" ;;
esac

export WALDO_USER_AGENT_OVERRIDE="Waldo BitriseStep/${waldo_build_flavor} v${BITRISE_STEP_VERSION}"

if [[ -z $symbols_path && $find_symbols == true ]]; then
    exec "$SCRIPT_DIR"/WaldoCLI.sh "$build_path" --include_symbols
else
    exec "$SCRIPT_DIR"/WaldoCLI.sh "$build_path" "$symbols_path"
fi
