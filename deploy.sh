#!/bin/bash

# This script downloads and installs/updates the latest release of emosup for Linux.
# It requires curl, jq, and unzip to be installed.

set -e

# --- Configuration ---
REPO="binaryu/emosup"
INSTALL_PATH="/usr/local/bin/emosup"
SERVICE_NAME="emosup.service"
SERVICE_PATH="/etc/systemd/system/$SERVICE_NAME"
CONFIG_DIR="$HOME/.config/emosup"

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
echo_info "此脚本将安装或更新 emosup 面板。"
read -p "您是否已在此服务器上安装 OpenList 和 Aria2？[y/N] " -n 1 -r
echo ""
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo_error "请先安装 OpenList 和 Aria2 再继续。详见 README。"
fi

for cmd in curl jq unzip; do
    if ! command -v $cmd &> /dev/null; then
        echo_error "$cmd 未安装。请先安装（例如：'sudo apt-get install $cmd'）。"
    fi
done

# --- Update or Install ---
if [ -f "$INSTALL_PATH" ]; then
    echo_info "在 $INSTALL_PATH 发现现有安装。"
    read -p "是否更新到最新版本？[y/N] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo_info "更新已取消。"
        exit 0
    fi
    
    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        echo_info "正在停止现有的 emosup 服务..."
        sudo systemctl stop "$SERVICE_NAME"
    fi
else
    echo_info "未发现现有安装。开始新安装。"
fi

# --- Architecture Detection ---
ARCH=$(uname -m)
case $ARCH in
    x86_64) ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    *) echo_error "不支持的架构：$ARCH。仅支持 amd64 和 arm64。" ;;
esac
echo_info "检测到架构：$ARCH"

# --- Fetch Latest Release ---
echo_info "从 GitHub 获取最新版本..."
API_URL="https://api.github.com/repos/$REPO/releases/latest"
RELEASE_INFO=$(curl -s "$API_URL")

if [[ $(echo "$RELEASE_INFO" | jq -r '.message // empty') == "Not Found" ]]; then
    echo_error "在 $REPO 未找到仓库或发布版本。请检查仓库名称。"
fi

TAG_NAME=$(echo "$RELEASE_INFO" | jq -r '.tag_name // empty')
if [ -z "$TAG_NAME" ]; then
    echo_error "无法获取最新版本标签。"
fi
echo_info "最新版本：$TAG_NAME"

ASSET_NAME="emosup-Linux-${ARCH}"
DOWNLOAD_URL=$(echo "$RELEASE_INFO" | jq -r ".assets[] | select(.name == \"$ASSET_NAME.zip\") | .browser_download_url // empty")

if [ -z "$DOWNLOAD_URL" ]; then
    echo_warn "未找到名为 '$ASSET_NAME.zip' 的发布资产。"
    echo_info "可用资产："
    echo "$RELEASE_INFO" | jq -r '.assets[].name'
    echo_error "请检查发布版本是否包含适合您架构的资产。"
fi

echo_info "找到资产：$ASSET_NAME.zip"

# --- Download and Install ---
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT  # 确保临时目录被清理

TEMP_ZIP_PATH="$TEMP_DIR/$ASSET_NAME.zip"
TEMP_EXTRACT_PATH="$TEMP_DIR/extracted"

echo_info "下载到临时文件..."
if ! curl -L -o "$TEMP_ZIP_PATH" "$DOWNLOAD_URL"; then
    echo_error "下载失败。请检查网络连接。"
fi

echo_info "解压可执行文件..."
mkdir -p "$TEMP_EXTRACT_PATH"
if ! unzip -q -o "$TEMP_ZIP_PATH" -d "$TEMP_EXTRACT_PATH"; then
    echo_error "解压失败。zip 文件可能已损坏。"
fi

# 查找可执行文件（应该是 emosup-Linux-amd64 或 emosup-Linux-arm64）
EXECUTABLE_PATH=$(find "$TEMP_EXTRACT_PATH" -type f -name "$ASSET_NAME" | head -n 1)

if [ -z "$EXECUTABLE_PATH" ]; then
    echo_warn "在下载的 zip 文件中未找到可执行文件 '$ASSET_NAME'。"
    echo_info "尝试查找任何可执行文件..."
    EXECUTABLE_PATH=$(find "$TEMP_EXTRACT_PATH" -type f -executable | head -n 1)
    
    if [ -z "$EXECUTABLE_PATH" ]; then
        echo_error "未找到任何可执行文件。zip 内容："
        ls -la "$TEMP_EXTRACT_PATH"
        exit 1
    fi
    echo_info "找到可执行文件：$(basename "$EXECUTABLE_PATH")"
fi

echo_info "安装到 $INSTALL_PATH（需要 sudo）..."
chmod +x "$EXECUTABLE_PATH"
if ! sudo mv "$EXECUTABLE_PATH" "$INSTALL_PATH"; then
    echo_error "安装失败。请检查权限。"
fi

echo_info "安装/更新完成！"

# --- Create config directory ---
if [ ! -d "$CONFIG_DIR" ]; then
    echo_info "创建配置目录：$CONFIG_DIR"
    mkdir -p "$CONFIG_DIR"
fi

# --- Post-install Instructions ---
if [ -f "$SERVICE_PATH" ]; then
    echo_info "Systemd 服务文件已存在。重启服务..."
    sudo systemctl daemon-reload
    sudo systemctl restart "$SERVICE_NAME"
    echo_info "服务已重启。查看状态：sudo systemctl status $SERVICE_NAME"
else
    echo
    echo_info "--- 下一步：创建 systemd 服务 ---"
    read -p "输入面板运行端口 [默认: 12345]: " -r PORT
    PORT=${PORT:-12345}
    USER=$(whoami)

    echo_info "以下是推荐的 systemd 服务配置。"
    echo_info "运行 'sudo nano $SERVICE_PATH'，粘贴内容并保存。"
    echo
    echo -e "\033[33m"
    cat << EOF
[Unit]
Description=EMOS Upload Panel Service
After=network.target

[Service]
Type=simple
User=$USER
WorkingDirectory=$HOME
ExecStart=$INSTALL_PATH --port $PORT
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF
    echo -e "\033[0m"
    echo
    echo_info "创建文件后，运行以下命令启用并启动服务："
    echo "  sudo systemctl daemon-reload"
    echo "  sudo systemctl enable $SERVICE_NAME"
    echo "  sudo systemctl start $SERVICE_NAME"
    echo "  sudo systemctl status $SERVICE_NAME"
fi