#!/usr/bin/env bash
# emosup one-click installer
#
#   curl -fsSL https://raw.githubusercontent.com/binaryu/emosup/main/scripts/install.sh | sudo bash
#   sudo bash install.sh install|update|uninstall|status|restart
#
# Env: EMOSUP_REPO EMOSUP_INSTALL_DIR EMOSUP_VERSION EMOSUP_PORT EMOSUP_PROXY
set -euo pipefail

REPO="${EMOSUP_REPO:-binaryu/emosup}"
INSTALL_DIR="${EMOSUP_INSTALL_DIR:-/opt/emosup}"
VERSION="${EMOSUP_VERSION:-}"
PORT="${EMOSUP_PORT:-}"
BUNDLE="${EMOSUP_BUNDLE:-}"
SERVICE_NAME="emosup"
KEEP_DATA=0
NONINTERACTIVE="${EMOSUP_NONINTERACTIVE:-0}"
EMOSUP_PROXY="${EMOSUP_PROXY:-}"

GITHUB_API="https://api.github.com/repos/${REPO}"
GITHUB_RELEASES="https://github.com/${REPO}/releases"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log()  { echo -e "${GREEN}[emosup]${NC} $*"; }
warn() { echo -e "${YELLOW}[emosup]${NC} $*"; }
err()  { echo -e "${RED}[emosup]${NC} $*" >&2; }
die()  { err "$*"; exit 1; }

need_cmd() { command -v "$1" >/dev/null 2>&1 || die "缺少命令: $1"; }
is_root()  { [[ "$(id -u)" -eq 0 ]]; }

http_get() {
  local url="$1" out="${2:-}"
  if command -v curl >/dev/null 2>&1; then
    if [[ -n "$out" ]]; then curl -fsSL --retry 3 -o "$out" "$url"
    else curl -fsSL --retry 3 "$url"; fi
  elif command -v wget >/dev/null 2>&1; then
    if [[ -n "$out" ]]; then wget -qO "$out" "$url"
    else wget -qO- "$url"; fi
  else
    die "需要 curl 或 wget"
  fi
}

proxy_github_url() {
  local url="$1"
  if [[ -z "$EMOSUP_PROXY" || "$EMOSUP_PROXY" == "0" ]]; then
    echo "$url"
    return
  fi
  local proxy_prefix="${EMOSUP_PROXY}"
  if [[ "$EMOSUP_PROXY" == "1" ]]; then
    proxy_prefix="https://gh-proxy.com/"
  fi
  [[ "$proxy_prefix" == */ ]] || proxy_prefix="${proxy_prefix}/"
  echo "${proxy_prefix}${url}"
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64)  echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *) die "不支持的架构: $(uname -m)（仅 linux amd64/arm64）" ;;
  esac
}

detect_os() {
  local o
  o="$(uname -s | tr '[:upper:]' '[:lower:]')"
  [[ "$o" == "linux" ]] || die "当前仅支持 Linux"
  echo "$o"
}

resolve_version() {
  if [[ -n "$BUNDLE" ]]; then
    if [[ -n "$VERSION" ]]; then
      echo "$VERSION"
    else
      echo "local"
    fi
    return
  fi
  if [[ -n "$VERSION" ]]; then
    echo "$VERSION"
    return
  fi
  local tag
  tag="$(http_get "$(proxy_github_url "${GITHUB_API}/releases/latest")" 2>/dev/null \
    | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1 || true)"
  if [[ -z "$tag" ]]; then
    tag="$(http_get "$(proxy_github_url "${GITHUB_API}/releases?per_page=5")" 2>/dev/null \
      | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1 || true)"
  fi
  [[ -n "$tag" ]] || die "无法获取版本，请: --version vX.Y.Z 或 EMOSUP_VERSION=..."
  echo "$tag"
}

download_and_extract() {
  local version="$1" arch="$2" dest="$3"
  local asset="emosup-linux-${arch}.tar.gz"
  local tmp
  tmp="$(mktemp -d)"

  if [[ -n "$BUNDLE" ]]; then
    [[ -f "$BUNDLE" ]] || { rm -rf "$tmp"; die "找不到 bundle: ${BUNDLE}"; }
    log "使用本地 bundle ${BUNDLE}"
    cp -a "$BUNDLE" "${tmp}/${asset}"
  else
    local url="${GITHUB_RELEASES}/download/${version}/${asset}"
    local proxy_url
    proxy_url="$(proxy_github_url "$url")"
    log "下载 ${proxy_url}"
    if ! http_get "$proxy_url" "${tmp}/${asset}"; then
      rm -rf "$tmp"
      die "下载失败。请确认 Release ${version} 已发布且包含 ${asset}"
    fi

    if http_get "$(proxy_github_url "${url}.sha256")" "${tmp}/${asset}.sha256" 2>/dev/null; then
      log "校验 SHA256"
      (cd "$tmp" && sha256sum -c "${asset}.sha256") || { rm -rf "$tmp"; die "校验和失败"; }
    else
      warn "未找到 .sha256，跳过校验"
    fi
  fi

  tar -xzf "${tmp}/${asset}" -C "$tmp"
  local extracted
  extracted="$(find "$tmp" -maxdepth 1 -type d \( -name 'emosup-linux-*' -o -name 'emosup' \) | head -1)"
  [[ -n "$extracted" && -x "${extracted}/emosup-server" ]] || {
    rm -rf "$tmp"
    die "解压后未找到 emosup-server"
  }

  mkdir -p "$dest"
  # Rename data/ aside (same filesystem → instant) instead of copying it to
  # /tmp: the download cache can be tens of GB and /tmp may be a small tmpfs.
  local keep_data=""
  if [[ -d "${dest}/data" ]]; then
    keep_data="${dest}/.upgrade-data-bak"
    rm -rf "$keep_data"
    mv "${dest}/data" "$keep_data"
  fi
  if [[ -f "${dest}/emosup.env" ]]; then
    cp -a "${dest}/emosup.env" "${tmp}/_keep_env"
  fi

  # Replace program files, preserve data/env
  find "$dest" -mindepth 1 -maxdepth 1 ! -name .upgrade-data-bak ! -name emosup.env -exec rm -rf {} +
  cp -a "${extracted}/." "$dest/"
  chmod +x "${dest}/emosup-server"

  if [[ -n "$keep_data" && -d "$keep_data" ]]; then
    rm -rf "${dest}/data"
    mv "$keep_data" "${dest}/data"
  else
    mkdir -p "${dest}/data/downloads"
  fi
  if [[ -f "${tmp}/_keep_env" ]]; then
    mv "${tmp}/_keep_env" "${dest}/emosup.env"
  fi

  echo "$version" > "${dest}/VERSION"
  rm -rf "$tmp"
  log "已安装到 ${dest}（版本 ${version}）"
}

write_env_file() {
  local dest="$1"
  local envf="${dest}/emosup.env"
  if [[ -f "$envf" ]]; then
    log "保留已有环境文件 ${envf}"
    sync_env_port "$dest"
    return
  fi
  : "${PORT:=8080}"
  cat > "$envf" <<EOF
# emosup 运行环境（systemd EnvironmentFile）
EMOSUP_DATA_DIR=${dest}/data
EMOSUP_FRONTEND_DIST=${dest}/frontend
EMOSUP_HOST=0.0.0.0
EMOSUP_PORT=${PORT}
# 本地媒体根目录（可选；也可在面板「系统配置 → 本地媒体」设置）
# EMOSUP_LOCAL_ROOT=/path/to/your/media
EOF
  log "已写入 ${envf}"
}

install_systemd() {
  local dest="$1"
  if ! command -v systemctl >/dev/null 2>&1; then
    warn "无 systemd，跳过服务注册。手动: cd ${dest} && ./emosup-server"
    return 1
  fi
  if ! is_root; then
    warn "非 root，跳过 systemd。手动: cd ${dest} && ./emosup-server"
    return 1
  fi

  cat > "/etc/systemd/system/${SERVICE_NAME}.service" <<EOF
[Unit]
Description=emosup — OpenList / local media → Emos upload panel
Documentation=https://github.com/${REPO}
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=${dest}
ExecStart=${dest}/emosup-server
Restart=on-failure
RestartSec=3
LimitNOFILE=65535
# All EMOSUP_* variables come from emosup.env. Do NOT hardcode Environment=
# here: systemd gives it priority over EnvironmentFile, which would silently
# ignore edits to emosup.env.
EnvironmentFile=-${dest}/emosup.env

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable "${SERVICE_NAME}"
  systemctl restart "${SERVICE_NAME}"
  log "systemd 已启用: systemctl status ${SERVICE_NAME}"
  return 0
}

stop_service() {
  if command -v systemctl >/dev/null 2>&1 && is_root; then
    systemctl stop "${SERVICE_NAME}" 2>/dev/null || true
  fi
  pkill -f "${INSTALL_DIR}/emosup-server" 2>/dev/null || true
  # Wait for the process to actually exit before releasing the port.
  local waited=0
  while (( waited < 5 )); do
    if ! pgrep -f "${INSTALL_DIR}/emosup-server" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.5
    (( waited++ ))
  done
  # Force kill if still alive after 2.5s.
  pkill -9 -f "${INSTALL_DIR}/emosup-server" 2>/dev/null || true
  sleep 0.5
}

# Ask for listen port when interactive and not already set via --port / EMOSUP_PORT.
prompt_port() {
  if [[ -n "$PORT" ]]; then
    return
  fi
  # Preserve existing install port on update if emosup.env already has one.
  if [[ -f "${INSTALL_DIR}/emosup.env" ]]; then
    local existing
    existing="$(sed -n 's/^EMOSUP_PORT=//p' "${INSTALL_DIR}/emosup.env" | head -1 | tr -d '"' | tr -d "'")"
    if [[ "$existing" =~ ^[0-9]+$ ]] && (( existing >= 1 && existing <= 65535 )); then
      PORT="$existing"
      log "沿用已配置端口: ${PORT}"
      return
    fi
  fi

  local default_port=8080
  if [[ "$NONINTERACTIVE" == "1" ]] || [[ ! -t 0 ]]; then
    # Piped install (curl | bash): stdin is the script, not a TTY.
    # Try to read from /dev/tty so the user can still answer.
    if [[ -r /dev/tty ]] && [[ -w /dev/tty ]]; then
      local answer=""
      echo -en "${GREEN}[emosup]${NC} 请输入面板端口 [${default_port}]: " > /dev/tty || true
      read -r answer < /dev/tty || true
      answer="$(echo "${answer:-}" | tr -d '[:space:]')"
      if [[ -z "$answer" ]]; then
        PORT="$default_port"
      elif [[ "$answer" =~ ^[0-9]+$ ]] && (( answer >= 1 && answer <= 65535 )); then
        PORT="$answer"
      else
        warn "端口无效，使用默认 ${default_port}"
        PORT="$default_port"
      fi
      return
    fi
    PORT="$default_port"
    log "非交互环境，使用默认端口 ${PORT}"
    return
  fi

  local answer=""
  read -r -p "$(echo -e "${GREEN}[emosup]${NC} 请输入面板端口 [${default_port}]: ")" answer || true
  answer="$(echo "${answer:-}" | tr -d '[:space:]')"
  if [[ -z "$answer" ]]; then
    PORT="$default_port"
  elif [[ "$answer" =~ ^[0-9]+$ ]] && (( answer >= 1 && answer <= 65535 )); then
    PORT="$answer"
  else
    warn "端口无效，使用默认 ${default_port}"
    PORT="$default_port"
  fi
}

prompt_proxy() {
  if [[ -n "$EMOSUP_PROXY" ]]; then
    return
  fi
  if [[ "$NONINTERACTIVE" == "1" ]] || [[ ! -t 0 ]]; then
    if [[ -r /dev/tty ]] && [[ -w /dev/tty ]]; then
      local answer=""
      echo -en "${GREEN}[emosup]${NC} 是否使用国内代理加速 GitHub 下载？[y/N]: " > /dev/tty || true
      read -r answer < /dev/tty || true
      case "$(echo "${answer:-}" | tr '[:upper:]' '[:lower:]')" in
        y|yes|1) EMOSUP_PROXY="1" ;;
        *)       EMOSUP_PROXY="0" ;;
      esac
      return
    fi
    EMOSUP_PROXY="0"
    return
  fi
  local answer=""
  read -r -p "$(echo -e "${GREEN}[emosup]${NC} 是否使用国内代理加速 GitHub 下载？[y/N]: ")" answer || true
  case "$(echo "${answer:-}" | tr '[:upper:]' '[:lower:]')" in
    y|yes|1) EMOSUP_PROXY="1" ;;
    *)       EMOSUP_PROXY="0" ;;
  esac
}

# Update EMOSUP_PORT in env file if the file already existed with another port.
sync_env_port() {
  local dest="$1"
  local envf="${dest}/emosup.env"
  [[ -f "$envf" ]] || return 0
  if grep -q '^EMOSUP_PORT=' "$envf"; then
    sed -i "s|^EMOSUP_PORT=.*|EMOSUP_PORT=${PORT}|" "$envf"
  else
    echo "EMOSUP_PORT=${PORT}" >> "$envf"
  fi
}

cmd_install() {
  need_cmd tar
  need_cmd uname
  local arch version
  detect_os >/dev/null
  arch="$(detect_arch)"
  version="$(resolve_version)"
  prompt_port
  prompt_proxy
  log "架构=${arch} 版本=${version} 目录=${INSTALL_DIR} 端口=${PORT}${EMOSUP_PROXY:+ 代理=${EMOSUP_PROXY}}"

  if [[ -x "${INSTALL_DIR}/emosup-server" ]]; then
    warn "检测到已有安装，将覆盖程序文件并保留 data/"
    stop_service
  fi

  download_and_extract "$version" "$arch" "$INSTALL_DIR"
  write_env_file "$INSTALL_DIR"
  sync_env_port "$INSTALL_DIR"

  if install_systemd "$INSTALL_DIR"; then
    sleep 1
    if systemctl is-active --quiet "${SERVICE_NAME}"; then
      log "部署成功"
      log "  面板: http://0.0.0.0:${PORT}"
      log "  账号: admin / admin （请尽快修改密码）"
    else
      warn "服务未 active，日志: journalctl -u ${SERVICE_NAME} -n 50 --no-pager"
    fi
  else
    log "前台运行: cd ${INSTALL_DIR} && ./emosup-server"
    log "  面板: http://127.0.0.1:${PORT}  账号: admin / admin"
  fi
}

cmd_update() {
  [[ -d "$INSTALL_DIR" && -x "${INSTALL_DIR}/emosup-server" ]] \
    || die "未找到安装: ${INSTALL_DIR}，请先 install"
  log "更新 emosup …"
  stop_service
  cmd_install
}

cmd_uninstall() {
  log "卸载 emosup → ${INSTALL_DIR}"
  stop_service
  if is_root && [[ -f "/etc/systemd/system/${SERVICE_NAME}.service" ]]; then
    systemctl disable "${SERVICE_NAME}" 2>/dev/null || true
    rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
    systemctl daemon-reload 2>/dev/null || true
    log "已移除 systemd unit"
  fi

  if [[ ! -d "$INSTALL_DIR" ]]; then
    warn "安装目录不存在"
    return 0
  fi

  if [[ "$KEEP_DATA" -eq 1 && -d "${INSTALL_DIR}/data" ]]; then
    local bak
    bak="${INSTALL_DIR%/}.data.bak.$(date +%Y%m%d%H%M%S)"
    mv "${INSTALL_DIR}/data" "$bak"
    log "数据已保留: ${bak}"
  fi
  rm -rf "$INSTALL_DIR"
  log "卸载完成"
  [[ "$KEEP_DATA" -eq 0 ]] && warn "数据已删除；若要保留: uninstall --keep-data"
}

cmd_status() {
  if [[ -f "${INSTALL_DIR}/VERSION" ]]; then
    log "版本: $(cat "${INSTALL_DIR}/VERSION")"
  else
    warn "未安装或缺少 VERSION"
  fi
  log "目录: ${INSTALL_DIR}"
  if command -v systemctl >/dev/null 2>&1 && systemctl cat "${SERVICE_NAME}" &>/dev/null; then
    systemctl status "${SERVICE_NAME}" --no-pager || true
  elif pgrep -f "${INSTALL_DIR}/emosup-server" >/dev/null 2>&1; then
    log "进程: $(pgrep -af "${INSTALL_DIR}/emosup-server" | head -1)"
  else
    warn "未在运行"
  fi
}

cmd_restart() {
  if command -v systemctl >/dev/null 2>&1 && is_root && systemctl cat "${SERVICE_NAME}" &>/dev/null; then
    systemctl restart "${SERVICE_NAME}"
    systemctl status "${SERVICE_NAME}" --no-pager || true
  else
    die "需要已安装的 systemd 服务 + root"
  fi
}

usage() {
  cat <<EOF
emosup 安装脚本

用法:
  $0 install   [--dir DIR] [--version TAG] [--port PORT] [--proxy URL|1]
  $0 install   [--bundle /path/to/emosup-linux-amd64.tar.gz] [--dir DIR] [--port PORT]
  $0 update    [--dir DIR] [--version TAG] [--port PORT] [--bundle FILE] [--proxy URL|1]
  $0 uninstall [--dir DIR] [--keep-data]
  $0 status    [--dir DIR]
  $0 restart

一键安装最新版（会询问端口，默认 8080）:
  curl -fsSL https://raw.githubusercontent.com/${REPO}/main/scripts/install.sh | sudo bash

指定端口 / 版本:
  curl -fsSL .../install.sh | sudo bash -s -- install --port 9090 --version v1.0.0

非交互（跳过询问）:
  curl -fsSL .../install.sh | sudo EMOSUP_PORT=9090 EMOSUP_NONINTERACTIVE=1 bash

国内代理（加速 GitHub 下载）:
  curl -fsSL .../install.sh | sudo EMOSUP_PROXY=1 bash              # 使用 gh-proxy.com
  curl -fsSL .../install.sh | sudo EMOSUP_PROXY=https://gh-proxy.com/ bash
  curl -fsSL .../install.sh | sudo bash -s -- install --proxy 1

更新 / 卸载:
  sudo bash install.sh update
  sudo bash install.sh uninstall --keep-data
EOF
}

main() {
  local action="${1:-install}"
  shift || true

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --dir)      INSTALL_DIR="$2"; shift 2 ;;
      --version)  VERSION="$2"; shift 2 ;;
      --port)     PORT="$2"; shift 2 ;;
      --bundle)   BUNDLE="$2"; shift 2 ;;
      --proxy)    EMOSUP_PROXY="$2"; shift 2 ;;
      --keep-data) KEEP_DATA=1; shift ;;
      -y|--yes)   NONINTERACTIVE=1; shift ;;
      -h|--help)  usage; exit 0 ;;
      *)          shift ;;
    esac
  done
  # PORT already set from EMOSUP_PORT at startup, or from --port above.

  case "$action" in
    install|"")   cmd_install ;;
    update)       cmd_update ;;
    uninstall|remove) cmd_uninstall ;;
    status)       cmd_status ;;
    restart)      cmd_restart ;;
    help|-h|--help) usage ;;
    *) usage; die "未知命令: $action" ;;
  esac
}

main "$@"
