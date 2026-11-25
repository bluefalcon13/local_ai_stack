#!/usr/bin/env bash
set -euo pipefail

DEST_DIR="${1:-$HOME/models/gguf}"
mkdir -p "$DEST_DIR"

echo "=== Pulling Native Strix-Halo RAG Models into $DEST_DIR ==="

# 1. Nomic Embed v1.5 (Q8_0) - 8192 Token Context Window, MTEB Leader
EMBED_FILE="$DEST_DIR/nomic-embed-text-v1.5.Q8_0.gguf"
if [ ! -f "$EMBED_FILE" ]; then
    echo "Downloading nomic-embed-text-v1.5.Q8_0.gguf (~570MB)..."
    curl -L --progress-bar "https://huggingface.co/nomic-ai/nomic-embed-text-v1.5-GGUF/resolve/main/nomic-embed-text-v1.5.Q8_0.gguf" -o "$EMBED_FILE"
else
    echo "nomic-embed-text-v1.5.Q8_0.gguf already exists, skipping."
fi

# 2. BGE Reranker Large (Q8_0) - Cross-Encoder SOTA
RERANK_FILE="$DEST_DIR/bge-reranker-large-Q8_0.gguf"
if [ ! -f "$RERANK_FILE" ]; then
    echo "Downloading bge-reranker-large-Q8_0.gguf (~590MB)..."
    curl -L --progress-bar "https://huggingface.co/kongfly/bge-reranker-large-Q8_0-GGUF/resolve/main/bge-reranker-large-q8_0.gguf" -o "$RERANK_FILE"
else
    echo "bge-reranker-large-Q8_0.gguf already exists, skipping."
fi

echo "=== All RAG models ready in $DEST_DIR ==="
ls -lh "$DEST_DIR"

