#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../plugins/samplebot"

# GOWORK=off isolates the build from the root go.work workspace so the
# samplebot module is compiled as a standalone WASM guest.
GOWORK=off GOOS=wasip1 GOARCH=wasm go build -o samplebot.wasm .
echo "samplebot.wasm built successfully"
