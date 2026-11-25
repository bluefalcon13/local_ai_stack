#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMAGE="${VLLM_IMAGE:-rocm/vllm:rocm10.1.0_ubuntu24.04_py3.14-pytorch_2.13.0_vllm-0.29.0}"
OUT_DIR="${OUT_DIR:-$HOME/models/gemma-4-26B-A4B-it-qat-w4a16-ct}"
GROUP_SIZE="${GROUP_SIZE:-32}"

if [[ ! -f "$SCRIPT_DIR/pack_gemma4_26b_w4a16.py" ]]; then
    echo "ERROR: Missing $SCRIPT_DIR/pack_gemma4_26b_w4a16.py" >&2
    exit 1
fi

avail_gib=$(awk '/MemAvailable/ {printf "%d", $2/1024/1024}' /proc/meminfo)
if (( avail_gib < 55 )) && [[ "${FORCE:-0}" != "1" ]]; then
    echo "WARNING: Only ${avail_gib} GiB MemAvailable; packing the 48.6 GiB BF16 checkpoint requires ~55+ GiB free RAM." >&2
    echo "Stop code-assistant first (e.g. 'docker compose stop code-assistant') or re-run with FORCE=1 to override." >&2
    exit 1
fi

mkdir -p "$OUT_DIR"

TTY_FLAG="-i"
if [[ -t 0 ]]; then
    TTY_FLAG="-it"
fi

echo "Running disposable packer container ($IMAGE)..."
echo "  Cache (ro) : $HOME/.cache/huggingface"
echo "  Output dir : $OUT_DIR"
echo "  Group size : $GROUP_SIZE"

docker run --rm "$TTY_FLAG" \
    -w /tmp \
    -e HF_HUB_OFFLINE=1 \
    -e HIP_VISIBLE_DEVICES="" \
    -e GROUP_SIZE="$GROUP_SIZE" \
    -v "$HOME/.cache/huggingface:/root/.cache/huggingface:ro" \
    -v "$OUT_DIR:/out" \
    -v "$SCRIPT_DIR/pack_gemma4_26b_w4a16.py:/pack.py:ro" \
    "$IMAGE" \
    bash -lc "pip install --no-cache-dir llmcompressor && python3 /pack.py"

echo ""
echo "Done! To serve the packed checkpoint in docker-compose.yml:"
echo "  1. Mount ~/models:/models:ro on code-assistant"
echo "  2. Set LRM_MODEL=/models/$(basename "$OUT_DIR")"
