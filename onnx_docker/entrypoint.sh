#!/usr/bin/env bash
set -e

# If the generic libonnxruntime.so doesn't exist yet, but the genai one is mounted, link it dynamically
if [ ! -f /usr/lib/libonnxruntime.so ] && [ -f /usr/lib/libonnxruntime-genai.so ]; then
    ln -s /usr/lib/libonnxruntime-genai.so /usr/lib/libonnxruntime.so
    echo "==> Dynamically linked libonnxruntime.so -> libonnxruntime-genai.so"
fi

# Execute the main Go worker binary passed as CMD
exec "$@"