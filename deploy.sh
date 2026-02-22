#!/bin/bash

# This script downloads and installs the latest release of emosup for Linux.
# It requires curl and jq to be installed.

set -e

# --- Configuration ---
# GitHub repository
REPO="binaryu/emosup"

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

# --- Prerequisite Check ---
echo_info "This script will install the emosup panel."
read -p "Have you already installed OpenList and Aria2 on this server? [y/N] " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo_error "Please install OpenList and Aria2 before proceeding. See README for details."
fi

# --- Dependency Check ---
for cmd in curl jq; do
    if ! command -v $cmd &> /dev/null; then
        echo_error "$cmd is not installed. Please install it first (e.g., 'sudo apt-get install $cmd')."
    fi
done

# --- Architecture Detection ---
ARCH=$(uname -m)
case $ARCH in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64)
        ARCH="arm64"
        ;;
    *)
        echo_error "Unsupported architecture: $ARCH. Only amd64 and arm64 are supported."
        ;;
esac
echo_info "Detected architecture: $ARCH"

# --- Fetch Latest Release ---
echo_info "Fetching latest release from GitHub..."
API_URL="https://api.github.com/repos/$REPO/releases/latest"
RELEASE_INFO=$(curl -s $API_URL)

if [[ $(echo "$RELEASE_INFO" | jq -r '.message') == "Not Found" ]]; then
    echo_error "Repository or release not found at $REPO. Please check the repository name."
fi

ASSET_NAME="emosup-Linux-${ARCH}"
DOWNLOAD_URL=$(echo "$RELEASE_INFO" | jq -r ".assets[] | select(.name == \"$ASSET_NAME\") | .browser_download_url")

if [ -z "$DOWNLOAD_URL" ]; then
    echo_error "Could not find a release asset named '$ASSET_NAME' for the latest release."
fi

echo_info "Found asset: $ASSET_NAME"
echo_info "Download URL: $DOWNLOAD_URL"

# --- Download and Install ---
INSTALL_PATH="/usr/local/bin/emosup"
TEMP_FILE=$(mktemp)

echo_info "Downloading to a temporary file..."
curl -L -o "$TEMP_FILE" "$DOWNLOAD_URL"

echo_info "Installing to $INSTALL_PATH (requires sudo)..."
chmod +x "$TEMP_FILE"
sudo mv "$TEMP_FILE" "$INSTALL_PATH"

echo_info "Installation complete!"
echo_info "Executable is now available at $INSTALL_PATH"

# --- Post-install Instructions ---
echo
echo_info "--- Next Steps: Create a systemd service ---"

read -p "Enter the port you want the panel to run on [default: 12345]: " -r PORT
PORT=${PORT:-12345}

USER=$(whoami)

echo_info "Below is a recommended systemd service configuration."
echo_info "Run 'sudo nano /etc/systemd/system/emosup.service', paste the content, and save."
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
# Optional: If you want to set environment variables for the service
# Environment="EMOS_API_BASE=https://your.emos.site"
# Environment="EMOS_TOKEN=your_token"

[Install]
WantedBy=multi-user.target
EOF
echo -e "\033[0m"
echo
echo_info "After creating the file, run these commands to enable and start the service:"
echo "sudo systemctl daemon-reload"
echo "sudo systemctl enable emosup.service"
echo "sudo systemctl start emosup.service"
echo "sudo systemctl status emosup.service"