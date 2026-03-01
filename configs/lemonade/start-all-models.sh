#!/bin/bash

# Start server with -1 for "Warden-mode" (Unlimited RAM management)
lemonade-server serve --host "0.0.0.0" --port "11434" --max-loaded-models -1 --no-tray &
SERVER_PID=$!

echo "Checking Bunker integrity..."
until curl -s http://localhost:42421/api/v1/health | grep -q "ok"; do
  sleep 2
done

echo "--- RESIDENT LOADING COMMENCE ---"
JSON_POST='curl -s -H "Content-Type: application/json" -X POST'

echo "Cell 1: Gemma 27B (The Brain) [131k Context]"
$JSON_POST http://localhost:42421/api/v1/load -d '{
  "model_name": "user.gemma-3-27B", 
  "ctx_size": 131072,
  "llamacpp_args": "-fa on --no-mmap"
}'
sleep 5

# echo "Cell 2: Qwen3 8B (The Router) [32k Context]"
# $JSON_POST http://localhost:42421/api/v1/load -d '{
#   "model_name": "Qwen3-8B-GGUF", 434
#   "ctx_size": 32768,
#   "llamacpp_args": "-fa on"
# }'
# sleep 5

#echo "Cell 3: Flux (The Visionary)"
#$JSON_POST http://localhost:42421/api/v1/load -d '{
#  "model_name": "Flux-2-Klein-9B-GGUF", 
#  "ctx_size": 32768,
#  "params": "-fa on --no-mmap",
#  "width": 1280,
#  "height": 720,
#  "steps": 10,
#  "cfg_scale": 3.5
#}'

echo "--- ALL INMATES SECURED ---"
wait $SERVER_PID
