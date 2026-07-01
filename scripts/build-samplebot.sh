#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../plugins/samplebot"

# GOWORK=off isolates the build from the root go.work workspace so the
# samplebot module is compiled as a standalone WASM guest. -trimpath drops
# absolute build paths so regenerating the committed fixture yields a smaller,
# less noisy diff. -buildmode=c-shared emits a WASI *reactor* (with an
# _initialize export the host calls before handle_update) rather than a command
# module — required so the Go runtime is initialized before an exported function
# runs; a command module traps with runtime.notInitialized().
GOWORK=off GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -trimpath -o samplebot.wasm .
echo "samplebot.wasm built successfully"
