#!/bin/bash

# --- Configuration ---
SERVICE_USER="ai-svc"
SERVICE_UID=1100
PROJECT_ROOT="$HOME/local_ai_stack"
MOUNT_POINT="/var/lib/localaistack-configs"
CURRENT_USER=$USER

echo "🚀 Starting Fishernode Bootstrap for user: $CURRENT_USER"

# 1. Create Service User
if id "$SERVICE_USER" &>/dev/null; then
    echo "✅ User $SERVICE_USER already exists."
else
    echo "👤 Creating service user $SERVICE_USER..."
    sudo useradd -u $SERVICE_UID -m -s /usr/sbin/nologin -G render,video,apex $SERVICE_USER
fi

# 2. Prepare Data Directories (The Data Plane)
echo "📂 Preparing data directories in /home/$SERVICE_USER..."
sudo mkdir -p /home/$SERVICE_USER/{caddy_config,webui_data,langgraph_db,ollama_main,phoenix_data}
sudo chown -R $SERVICE_USER:$SERVICE_USER /home/$SERVICE_USER
sudo chmod -R 700 /home/$SERVICE_USER

# 3. Setting up the Bind Mount Portal (The Config Plane)
echo "🌉 Setting up the Bind Mount Portal at $MOUNT_POINT..."
sudo mkdir -p $MOUNT_POINT
sudo mkdir -p $PROJECT_ROOT/configs/{caddy,langgraph/agents,searxng}

if mountpoint -q "$MOUNT_POINT"; then
    echo "✅ $MOUNT_POINT is already mounted."
else
    echo "🔗 Mounting $PROJECT_ROOT/configs to $MOUNT_POINT..."
    sudo mount --bind "$PROJECT_ROOT/configs" "$MOUNT_POINT"
fi

# 4. Applying Permission Sweep
echo "🔒 Applying Permission Sweep..."
sudo chown -R $CURRENT_USER:$SERVICE_USER "$PROJECT_ROOT/configs"
sudo chmod -R 750 "$PROJECT_ROOT/configs"

# Ensure mTLS certs are readable by the service user
if [ -d "$PROJECT_ROOT/configs/caddy/cloudflare_certs" ]; then
    sudo chmod 640 "$PROJECT_ROOT/configs/caddy/cloudflare_certs"/*.pem 2>/dev/null || echo "⚠️  No .pem files found yet."
fi

echo ""
echo "✨ Setup Complete!"
echo "--------------------------------------------------------"
echo "To make this mount permanent, add this line to /etc/fstab:"
echo "$PROJECT_ROOT/configs $MOUNT_POINT none bind,nofail 0 0"