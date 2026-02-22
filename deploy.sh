#!/bin/bash

# This script downloads and installs/updates the latest release of emosup for Linux.
# It requires curl, jq, and unzip to be installed.

set -e

# --- Configuration ---
REPO="binaryu/emosup"
INSTALL_PATH="/usr/local/bin/emosup"
SERVICE_NAME="emosup.service"
SERVICE_PATH="/etc/systemd/system/$SERVICE_NAME"

# --- Helper Functions ---
echo_info() {
    echo -e "\033[32m[INFO]\033[0m $1"
}

echo_error() {
    echo -e "\033[31m[ERROR]\033[0m $1" >&2
    exit 1
}

echo_warn() {
    echo -e "\033[33m[WARN]\033[0m $1"
}

# --- Prerequisite & Dependency Check ---
echo_info "This script will install or update the emosup panel."
read -p "Have you already installed OpenList and Aria2 on this server? [y/N] " -n 1 -r
echo ""
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo_error "Please install OpenList and Aria2 before proceeding. See README for details."
fi

for cmd in curl jq unzip; do
    if ! command -v $cmd &> /dev/null; then
        echo_error "$cmd is not installed. Please install it first (e.g., 'sudo apt-get install $cmd')."
    fi
done

# --- Update or Install ---
if [ -f "$INSTALL_PATH" ]; then
    echo_info "Existing installation found at $INSTALL_PATH."
    read -p "Do you want to update to the latest version? [y/N] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo_info "Update cancelled."
        exit 0
    fi
    
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        echo_info "Stopping the existing emosup service..."
        sudo systemctl stop "$SERVICE_NAME"
    fi
else
    echo_info "No existing installation found. Proceeding with new installation."
fi

# --- Architecture Detection ---
ARCH=$(uname -m)
case $ARCH in
    x86_64) ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    *) echo_error "Unsupported architecture: $ARCH. Only amd64 and arm64 are supported." ;;
esac
echo_info "Detected architecture: $ARCH"

# --- Fetch Latest Release ---
echo_info "Fetching latest release from GitHub..."
API_URL="https://api.github.com/repos/$REPO/releases/latest"
RELEASE_INFO=$(curl -s "$API_URL")

if [[ $(echo "$RELEASE_INFO" | jq -r '.message') == "Not Found" ]]; then
    echo_error "Repository or release not found at $REPO. Please check the repository name."
fi

ASSET_NAME="emosup-Linux-${ARCH}"
DOWNLOAD_URL=$(echo "$RELEASE_INFO" | jq -r ".assets[] | select(.name == \"$ASSET_NAME.zip\") | .browser_download_url")

if [ -z "$DOWNLOAD_URL" ]; then
    echo_error "Could not find a release asset named '$ASSET_NAME.zip' for the latest release."
fi

echo_info "Found asset: $ASSET_NAME.zip"

# --- Download and Install ---
TEMP_DIR=$(mktemp -d)
TEMP_ZIP_PATH="$TEMP_DIR/$ASSET_NAME.zip"
TEMP_EXTRACT_PATH="$TEMP_DIR/extracted"

echo_info "Downloading to a temporary file..."
curl -L -o "$TEMP_ZIP_PATH" "$DOWNLOAD_URL"

echo_info "Unzipping the executable..."
unzip -o "$TEMP_ZIP_PATH" -d "$TEMP_EXTRACT_PATH" # -o to overwrite
EXECUTABLE_PATH=$(find "$TEMP_EXTRACT_PATH" -type f -name "emosup-*" | head -n 1)

if [ -z "$EXECUTABLE_PATH" ]; then
    echo_error "Could not find the executable in the downloaded zip file."
fi

echo_info "Installing to $INSTALL_PATH (requires sudo)..."
chmod +x "$EXECUTABLE_PATH"
sudo mv "$EXECUTABLE_PATH" "$INSTALL_PATH"
rm -rf "$TEMP_DIR"

echo_info "Installation/Update complete!"

# --- Post-install Instructions ---
if [ -f "$SERVICE_PATH" ]; then
    echo_info "Systemd service file already exists. Restarting the service..."
    sudo systemctl daemon-reload
    sudo systemctl restart "$SERVICE_NAME"
    echo_info "Service restarted. Check status with: sudo systemctl status $SERVICE_NAME"
else
    echo
    echo_info "--- Next Steps: Create a systemd service ---"
    read -p "Enter the port you want the panel to run on [default: 12345]: " -r PORT
    PORT=${PORT:-12345}
    USER=$(whoami)

    echo_info "Below is a recommended systemd service configuration."
    echo_info "Run 'sudo nano $SERVICE_PATH', paste the content, and save."
    echo
    echo -e "\033[33m"
    cat << EOF
[Unit]
Description=EMOS Upload Panel Service
After=network.target

[Service]
Type=simple
User=$USER
WorkingDirectory=$(dirname "$INSTALL_PATH")
ExecStart=$INSTALL_PATH --port $PORT
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF
    echo -e "\033[0m"
    echo
    echo_info "After creating the file, run these commands to enable and start the service:"
    echo "sudo systemctl daemon-reload"
    echo "sudo systemctl enable $SERVICE_NAME"
    echo "sudo systemctl start $SERVICE_NAME"
    echo "sudo systemctl status $SERVICE_NAME"
fi