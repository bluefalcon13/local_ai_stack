#!/bin/bash

# --- 0. Strict Error Handling ---
set -euo pipefail

# --- 1. Load and Validate Environment ---
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="$PROJECT_ROOT/.env"

if [[ ! -f "$ENV_FILE" ]]; then
    echo "❌ Error: .env file not found at $ENV_FILE" >&2
    exit 1
fi

# Load and export variables so Docker Compose can see them 
set -a
source "$ENV_FILE"
set +a

# Essential validation
REQUIRED_VARS=("CONFIG_ROOT" "BASE_DOMAIN" "SERVICE_UID" "SERVICE_USER")
for var in "${REQUIRED_VARS[@]}"; do
    if [[ -z "${!var:-}" ]]; then
        echo "❌ Error: Required environment variable '$var' is not set in .env" >&2
        exit 1
    fi
done

echo "🚀 Starting Fishernode Bootstrap with UID: $SERVICE_UID"

# --- 2. Identity Validation & User Creation ---

# Get raw system state (Null if missing)
CURRENT_NAME_FOR_UID=$(id -nu "$SERVICE_UID" 2>/dev/null || echo "")
CURRENT_UID_FOR_NAME=$(id -u "$SERVICE_USER" 2>/dev/null || echo "")

# Logic Gate 1: Mismatch Checks (The "Stop" signals)
if [[ -n "$CURRENT_UID_FOR_NAME" && "$CURRENT_UID_FOR_NAME" != "$SERVICE_UID" ]]; then
    echo "❌ Identity Conflict: User '$SERVICE_USER' exists but is UID $CURRENT_UID_FOR_NAME (Expected $SERVICE_UID)" >&2
    exit 1
fi

if [[ -n "$CURRENT_NAME_FOR_UID" && "$CURRENT_NAME_FOR_UID" != "$SERVICE_USER" ]]; then
    echo "❌ Identity Conflict: UID $SERVICE_UID is already taken by user '$CURRENT_NAME_FOR_UID'" >&2
    exit 1
fi

# Logic Gate 2: The "Go" signals
if [[ -z "$CURRENT_NAME_FOR_UID" && -z "$CURRENT_UID_FOR_NAME" ]]; then
    echo "👤 Identity Vacant. Creating service user '$SERVICE_USER' with UID $SERVICE_UID..."
    
    # Ensure hardware groups exist for the Strix Halo
    for group in render video apex; do
        getent group "$group" >/dev/null || sudo groupadd "$group"
    done
    
    sudo useradd -u "$SERVICE_UID" -m -s /usr/sbin/nologin -G render,video,apex "$SERVICE_USER"
    echo "✅ User created successfully."
else
    echo "✅ Identity Verified: System matches .env ($SERVICE_USER:$SERVICE_UID)."
fi

# --- 3. Prepare Data Directories ---
echo "📂 Preparing data directories in /home/$SERVICE_USER..."
DATA_DIRS=(caddy_config webui_data langgraph_db ollama_main phoenix_data)
for dir in "${DATA_DIRS[@]}"; do
    sudo mkdir -p "/home/$SERVICE_USER/$dir"
done

sudo chown -R "$SERVICE_UID:$SERVICE_UID" "/home/$SERVICE_USER"
sudo chmod -R 700 "/home/$SERVICE_USER"

# --- 4. Setting up the Bind Mount Portal ---
sudo mkdir -p "$CONFIG_ROOT"
sudo mkdir -p "$PROJECT_ROOT/configs"/{caddy/cloudflare_certs,langgraph/agents,searxng}

if mountpoint -q "$CONFIG_ROOT"; then
    echo "✅ $CONFIG_ROOT is already mounted."
else
    sudo mount --bind "$PROJECT_ROOT/configs" "$CONFIG_ROOT"
fi

# --- 5. Applying Permission Sweep ---
# Ensure the service user (via UID) has access to the config portal 
sudo chown -R "$USER:$SERVICE_UID" "$PROJECT_ROOT/configs"
sudo chmod -R 750 "$PROJECT_ROOT/configs"

# Ensure mTLS certs are readable by the service user but private (640)
CERT_PATH="$PROJECT_ROOT/configs/caddy/cloudflare_certs"
if [ -d "$CERT_PATH" ]; then
    sudo chmod 640 "$CERT_PATH"/* 2>/dev/null || true
fi

echo "✨ Bootstrap complete. You can now run: docker compose up -d"
echo "--------------------------------------------------------"
echo ""
echo "To make this mount permanent, add this line to /etc/fstab:"
echo "$PROJECT_ROOT/configs $MOUNT_POINT none bind,nofail 0 0"
echo ""
echo "--------------------------------------------------------"