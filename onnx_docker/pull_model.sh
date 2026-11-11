#!/usr/bin/env bash
set -euo pipefail

MODEL_ID="${1:-onnx-community/Qwen2.5-Coder-3B-Instruct}"

echo "==> Downloading model: ${MODEL_ID} into shared HF cache..."

hf download "${MODEL_ID}" \
    --include "*.onnx" \
    --include "*.onnx_data" \
    --include "*.json" \
    --include "*.model" \
    --include "*.txt"

echo "==> Download complete. Cached and ready for container mount."