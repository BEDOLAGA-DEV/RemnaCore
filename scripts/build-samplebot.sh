#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../plugins/samplebot"

# GOWORK=off isolates the build from the root go.work workspace so the
# samplebot module is compiled as a standalone WASM guest. -trimpath drops
# absolute build paths so regenerating the committed fixture yields a smaller,
# less noisy diff.
GOWORK=off GOOS=wasip1 GOARCH=wasm go build -trimpath -o samplebot.wasm .
echo "samplebot.wasm built successfully"
