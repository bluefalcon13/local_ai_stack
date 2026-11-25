#!/usr/bin/env bash
set -euo pipefail

MODEL_ID="${1:-amd/Qwen2.5-Coder-1.5B-Instruct_rai_1.9.0_npu}"
TARGET_DIR="${2:-$HOME/models/qwen-1.5b-npu}"

echo "==> Downloading NPU model: ${MODEL_ID} -> ${TARGET_DIR}..."
mkdir -p "${TARGET_DIR}"

hf download "${MODEL_ID}" --local-dir "${TARGET_DIR}"

echo "==> Converting any remaining symlinks to hardlinks for ONNX Runtime compatibility..."
find "${TARGET_DIR}" -type l | while read -r symlink; do
    target="$(readlink -f "$symlink")"
    if [ -f "$target" ]; then
        ln -f "$target" "$symlink" 2>/dev/null || cp -f "$target" "$symlink"
    fi
done

echo "==> Patching genai_config.json for XDNA 2 NPU static 4096 KV-cache alignment..."
if [ -f "${TARGET_DIR}/genai_config.json" ]; then
    sed -i 's/"fusion_opt_io_bind_kv_cache": "1"/"fusion_opt_io_bind_kv_cache": "0"/g; s/"max_length": 32768/"max_length": 4096/g' "${TARGET_DIR}/genai_config.json"
fi

echo "==> Ensuring model files are writable for NPU driver DMA pinning..."
chmod -R u+w "${TARGET_DIR}" 2>/dev/null || true

echo "==> Download complete: ${TARGET_DIR}"
