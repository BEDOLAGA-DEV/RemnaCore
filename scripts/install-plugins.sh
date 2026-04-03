#!/usr/bin/env bash
#
# install-plugins.sh — Build all WASM plugins and install them into a running
# RemnaCore instance via the admin API.
#
# Usage:
#   API_URL=http://localhost:4000 API_TOKEN=<jwt> ./scripts/install-plugins.sh
#
# Environment variables:
#   API_URL    — RemnaCore base URL (default: http://localhost:4000)
#   API_TOKEN  — Admin JWT token (required)
#
# The script discovers plugins by looking for directories under plugins/ that
# contain a plugin.toml manifest. Each plugin is compiled to WASM, then the
# manifest and binary are base64-encoded and POSTed to POST /api/admin/plugins.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
PLUGINS_DIR="${PROJECT_ROOT}/plugins"

API_URL="${API_URL:-http://localhost:4000}"
API_TOKEN="${API_TOKEN:-}"

# --- Validation ---

if [ -z "${API_TOKEN}" ]; then
    echo "ERROR: API_TOKEN environment variable is required" >&2
    echo "Usage: API_URL=http://localhost:4000 API_TOKEN=<jwt> $0" >&2
    exit 1
fi

if ! command -v go >/dev/null 2>&1; then
    echo "ERROR: go is not installed or not in PATH" >&2
    exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
    echo "ERROR: curl is not installed or not in PATH" >&2
    exit 1
fi

# --- Constants ---

readonly INSTALL_ENDPOINT="${API_URL}/api/admin/plugins"
readonly ENABLE_ENDPOINT_PREFIX="${API_URL}/api/admin/plugins"

# --- Counters ---

total=0
succeeded=0
failed=0

# --- Main loop ---

for plugin_dir in "${PLUGINS_DIR}"/*/; do
    plugin_name="$(basename "${plugin_dir}")"

    # Skip directories without a manifest.
    if [ ! -f "${plugin_dir}/plugin.toml" ]; then
        continue
    fi

    # Skip the templates directory.
    if [ "${plugin_name}" = "templates" ]; then
        continue
    fi

    total=$((total + 1))
    echo "=== [${total}] ${plugin_name} ==="

    # Build WASM binary.
    echo "  Building WASM..."
    if ! (cd "${plugin_dir}" && GOWORK=off GOOS=wasip1 GOARCH=wasm go build -o plugin.wasm . 2>&1); then
        echo "  FAILED: WASM compilation error" >&2
        failed=$((failed + 1))
        continue
    fi

    wasm_file="${plugin_dir}/plugin.wasm"
    if [ ! -f "${wasm_file}" ]; then
        echo "  FAILED: plugin.wasm not produced" >&2
        failed=$((failed + 1))
        continue
    fi

    wasm_size="$(wc -c < "${wasm_file}" | tr -d ' ')"
    echo "  Built plugin.wasm (${wasm_size} bytes)"

    # Base64-encode manifest and WASM binary.
    manifest_b64="$(base64 < "${plugin_dir}/plugin.toml")"
    wasm_b64="$(base64 < "${wasm_file}")"

    # Build JSON payload. The "wasm" field is []byte in Go, which expects
    # base64-encoded content when sent as a JSON string.
    payload="$(printf '{"manifest":"%s","wasm":"%s"}' "${manifest_b64}" "${wasm_b64}")"

    # POST to install endpoint.
    echo "  Installing to ${INSTALL_ENDPOINT}..."
    http_code="$(curl -s -o /tmp/remnacore-plugin-response.json -w '%{http_code}' \
        -X POST "${INSTALL_ENDPOINT}" \
        -H "Authorization: Bearer ${API_TOKEN}" \
        -H "Content-Type: application/json" \
        -d "${payload}")"

    if [ "${http_code}" -ge 200 ] && [ "${http_code}" -lt 300 ]; then
        echo "  Installed (HTTP ${http_code})"

        # Enable the plugin.
        echo "  Enabling ${plugin_name}..."
        enable_code="$(curl -s -o /dev/null -w '%{http_code}' \
            -X POST "${ENABLE_ENDPOINT_PREFIX}/${plugin_name}/enable" \
            -H "Authorization: Bearer ${API_TOKEN}" \
            -H "Content-Type: application/json")"

        if [ "${enable_code}" -ge 200 ] && [ "${enable_code}" -lt 300 ]; then
            echo "  Enabled (HTTP ${enable_code})"
        else
            echo "  WARNING: enable returned HTTP ${enable_code} (plugin installed but not enabled)"
        fi

        succeeded=$((succeeded + 1))
    else
        echo "  FAILED: HTTP ${http_code}" >&2
        if [ -f /tmp/remnacore-plugin-response.json ]; then
            echo "  Response:" >&2
            cat /tmp/remnacore-plugin-response.json >&2
            echo "" >&2
        fi
        failed=$((failed + 1))
    fi

    # Clean up WASM binary.
    rm -f "${wasm_file}"

    echo ""
done

# --- Summary ---

echo "=== Summary ==="
echo "  Total:     ${total}"
echo "  Succeeded: ${succeeded}"
echo "  Failed:    ${failed}"

if [ "${failed}" -gt 0 ]; then
    exit 1
fi
