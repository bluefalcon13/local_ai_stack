#!/bin/bash
set -euo pipefail

# --- 1. Load and Validate Environment ---
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="$PROJECT_ROOT/.env"

if [[ ! -f "$ENV_FILE" ]]; then
    echo "❌ Error: .env file not found. Copy example.env to .env and fill it out."
    exit 1
fi

# Load variables
set -a; source "$ENV_FILE"; set +a

echo "🔍 Running Mechanic's Audit on .env..."

# Unified variable list matching Caddyfile and Compose
REQUIRED_VARS=(
    "CONFIG_ROOT" "SERVICE_UID" "SERVICE_USER" "VIDEO_GID" "RENDER_GID" "APEX_GID"
    "DOMAIN_NAME" "CLOUDFLARE_EMAIL" "CLOUDFLARE_TUNNEL_TOKEN"
    "TLS_CLIENT_CERT_NAME" "TLS_CLIENT_KEY_NAME" "CLOUDFLARE_AUTH_ORIGIN_PULL_CERT_NAME"
    "WEBUI_SECRET" "SEARXNG_SECRET" "PHOENIX_SECRET"
    "POSTGRES_USER" "POSTGRES_PASSWORD" "POSTGRES_DB"
)

ERRORS=()
for var in "${REQUIRED_VARS[@]}"; do
    if [[ ! -v "$var" || -z "${!var}" ]]; then
        ERRORS+=("$var")
    fi
done

if [[ ${#ERRORS[@]} -gt 0 ]]; then
    echo "❌ AUDIT FAILED: The following variables are missing or empty in .env:"
    for err in "${ERRORS[@]}"; do
        echo "   - $err"
    done
    echo -e "\n🔧 Mechanic's Instructions:"
    echo "   - Hardware GIDs: Run 'getent group render video apex'"
    echo "   - Secrets: Run 'openssl rand -hex 32' for the secret keys"
    exit 1
fi

# --- 2. Hardware GID Sanity Check ---
for gid in "$RENDER_GID" "$VIDEO_GID" "$APEX_GID"; do
    if ! getent group "$gid" >/dev/null; then
        echo "❌ Hardware Error: GID '$gid' does not exist on this host."
        exit 1
    fi
done

echo "✅ Audit Passed. Preparing folders..."

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

# --- 3. Prepare Data Directories (Matching Compose Volumes) ---
# Unified directories for all services in your docker-compose.yml
DATA_DIRS=(
    "caddy/data" 
    "caddy/config" 
    "webui_data" 
    "langgraph_db" 
    "searxng_cache" 
    "phoenix_data" 
    "models"
)

for dir in "${DATA_DIRS[@]}"; do
    sudo mkdir -p "/home/$SERVICE_USER/$dir"
done

# Set permissions for the service user
sudo chown -R "$SERVICE_UID:$SERVICE_UID" "/home/$SERVICE_USER"
sudo chmod -R 700 "/home/$SERVICE_USER"

# --- 4. Config Portal Validation & Mounting ---
echo "📂 Validating Config Portal structure..."
CONFIG_DIRS=("caddy" "caddy/cloudflare_certs" "langgraph" "searxng" "lemonade")
for cdir in "${CONFIG_DIRS[@]}"; do
    if [ ! -d "$PROJECT_ROOT/configs/$cdir" ]; then
        echo "❌ Error: Config directory '$cdir' missing in repo." >&2
        exit 1
    fi
done

sudo chown -R "$USER:$SERVICE_UID" "$PROJECT_ROOT/configs"
sudo chmod -R 750 "$PROJECT_ROOT/configs"
sudo chmod 640 "$PROJECT_ROOT/configs/caddy/cloudflare_certs"/* 2>/dev/null || true

sudo mkdir -p "$CONFIG_ROOT"
if ! mountpoint -q "$CONFIG_ROOT"; then
    sudo mount --bind "$PROJECT_ROOT/configs" "$CONFIG_ROOT"
fi

echo "✨ Bootstrap Complete. Mechanic, you are clear to launch."
echo "--------------------------------------------------------"
echo ""
echo "To make this mount permanent, add this line to /etc/fstab:"
echo "$PROJECT_ROOT/configs $CONFIG_ROOT none bind,nofail 0 0"
echo ""
echo "--------------------------------------------------------"

print_permission_reminder(){
    printf "🚨 NETWORKING PERMISSION REMINDER 🚨\\n"
    printf "="*60 + "\\n"
    printf "To allow Caddy (running as user 1100) to bind to port 443/80,\\n"
    printf "you MUST run the following on your Arch host:\\n"
    printf "\\n  sudo sysctl -w net.ipv4.ip_unprivileged_port_start=80\\n"
    printf "\\nTo make it stick after reboot, add it here:\\n"
    printf "\\n  echo 'net.ipv4.ip_unprivileged_port_start=80' | sudo tee /etc/sysctl.d/99-caddy.conf\\n"

}

print_permission_reminder