#!/bin/bash
MODEL_NAME=$1

if [ -z "$MODEL_NAME" ]; then
  echo "Usage: ./pull_ollama_model.sh <model_name>"
  exit 1
fi

CONTAINER_NAME="ollama-main"

set -a            # Automatically export all variables defined hereafter
source .env       # Load your main .env file
set +a            # Stop auto-exporting

# The Failsafe: Check if the container is running
IF_RUNNING=$(docker ps -q -f name=^/${CONTAINER_NAME}$ -f status=running)

if [ -n "$IF_RUNNING" ]; then
  echo "ERROR: $CONTAINER_NAME is currently running!"
  echo "Please run 'docker compose down' before dropping supplies to avoid corruption."
  exit 1
fi

echo "--- Clearance Granted: $CONTAINER_NAME is offline. ---"

# Your transient puller command
docker run -d \
  --name ollama-puller \
  --user "${SERVICE_UID}:${SERVICE_UID}" \
  --group-add "${VIDEO_GID}" \
  --group-add "${RENDER_GID}" \
  --shm-size 2gb \
  --device /dev/kfd:/dev/kfd \
  --device /dev/dri:/dev/dri \
  -v /home/${SERVICE_USER}/ollama_main:/ollama \
  -e HSA_OVERRIDE_GFX_VERSION=11.5.1 \
  -e HIP_FORCE_DEV_RESET=1 \
  -e GPU_MAX_ALLOC_PERCENT=100 \
  -e HSA_ENABLE_SDMA=0 \
  -e GGML_NGPUS=1 \
  -e OLLAMA_FLASH_ATTENTION=true \
  -e GGML_CUDA_NO_PINNED_TRANSFER=1 \
  -e OLLAMA_CONTEXT_LENGTH=32768 \
  -e OLLAMA_MODELS=/ollama/models \
  -e HOME=/ollama \
  -e OLLAMA_HOST=0.0.0.0 \
  ollama/ollama:rocm \
  serve

# 3. Wait for the server to be ready (usually 2-5 seconds)
echo "Waiting for server to initialize..."
sleep 5

# 4. Execute the pull inside that container
docker exec -it ollama-puller ollama pull $MODEL_NAME

# 5. Clean up
echo "--- Supply Drop Complete. Scuttling Supply Ship. ---"
docker rm -f ollama-puller