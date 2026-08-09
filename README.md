# emosup

OpenList / 本地文件 → Emos 异步转存面板。

## 特性

- **多源扫描** — OpenList 网盘与本机目录，递归子目录、多文件勾选
- **BT 下载** — 对接 qBittorrent WebUI 加磁力链接，下载完成后一键扫描转存；是否保留本地文件在做种/转存前按需勾选
- **缓存管理** — 下载缓存目录可视化，识别孤儿/临时分片/任务引用文件，批量清理
- **TMDB 匹配** — 搜片名取 ID，进目录自动猜剧名
- **内建下载器** — 多线程分段、断点续传、自动重试
- **Emos 上传** — onedrive/r2 分片、multipart 预签名分片、断点续传
- **文件名解析** — `S01E02`、`[Show][25][BIG5][1080P]`、`入青云01.mp4`、`EP03`、`第04集`、`Season II` 等
- **实时进度** — SSE 推送速度/进度
- **SQLite 存储** — 任务进度高频写入不再卡在 JSON 文件上
- **面板内一键升级** — 检查 GitHub 最新版，下载校验后自动替换并重启（保留 data/ 与 emosup.env）
- **一键二进制部署** — 安装 / 更新 / 卸载脚本 + systemd

## 快速开始

```bash

curl -fsSL https://gh-proxy.com/raw.githubusercontent.com/binaryu/emosup/main/scripts/install.sh | sudo bash
```

打开 `http://服务器IP:端口`，默认账号 **`admin` / `admin`**（请立刻改密码）。

### 常用命令

```bash
# 安装指定版本
curl -fsSL https://raw.githubusercontent.com/binaryu/emosup/main/scripts/install.sh \
  | sudo bash -s -- install --version v1.0.0

# 用本地打包产物安装（适合先部署测试版，不依赖已发布的 Release）
sudo bash scripts/install.sh install --bundle /path/to/emosup-linux-amd64.tar.gz

# 装到自定义目录
sudo bash install.sh install --dir /home/user/emosup --port 8080

# 更新到最新版（保留 data/）
sudo bash /opt/emosup/install.sh update
# 或
curl -fsSL https://raw.githubusercontent.com/binaryu/emosup/main/scripts/install.sh \
  | sudo bash -s -- update

# 状态 / 重启
sudo bash /opt/emosup/install.sh status
sudo systemctl restart emosup
sudo journalctl -u emosup -f

# 卸载（保留数据）
sudo bash /opt/emosup/install.sh uninstall --keep-data
# 卸载（连数据一起删）
sudo bash /opt/emosup/install.sh uninstall
```

### 安装后目录

```
/opt/emosup/
├── emosup-server      # 主程序
├── frontend/          # 面板静态资源
├── data/              # SQLite + 下载缓存（更新时保留）
│   ├── emosup.db
│   └── downloads/
├── emosup.env         # 环境变量（端口、目录等）
├── VERSION
└── install.sh
```

编辑 `/opt/emosup/emosup.env` 后执行 `sudo systemctl restart emosup`：

| 变量 | 说明 | 默认 |
|------|------|------|
| `EMOSUP_PORT` | 监听端口 | `8080` |
| `EMOSUP_HOST` | 监听地址 | `0.0.0.0` |
| `EMOSUP_DATA_DIR` | 数据目录 | `/opt/emosup/data` |
| `EMOSUP_FRONTEND_DIST` | 前端目录 | `/opt/emosup/frontend` |
| `EMOSUP_LOCAL_ROOT` | 本地浏览根目录（可选） | 面板配置 / downloads |
| `EMOSUP_DOWNLOADS_DIR` | 下载缓存目录（可选） | 面板配置 download_dir |
| `EMOSUP_DOWNLOADS_HOST` | 下载缓存宿主机路径（Docker 映射用） | 同容器内路径 |

本地媒体目录也可在面板 **系统配置 → 本地媒体** 里改，无需改环境变量。

### 面板内自动升级

「系统配置 → 关于与升级」可一键升级：自动检测 GitHub 最新 Release、下载并校验 SHA256、替换程序文件（保留 `data/` 与 `emosup.env`）后重启服务。

- 适用于 `install.sh` 部署的目录（systemd 或手动运行均可）
- Docker 部署请在宿主机执行 `docker compose pull && docker compose up -d`（面板内升级会给出提示并拒绝）
- 国内网络可在安装时通过 `EMOSUP_PROXY=1` 走 gh-proxy 加速（面板升级同样生效）

### 不装 systemd、手动跑

```bash
# 从 GitHub Releases 下载 emosup-linux-amd64.tar.gz 后：
tar -xzf emosup-linux-amd64.tar.gz
cd emosup-linux-amd64
./emosup-server
```

---

## Docker 部署（可选）

```bash
wget https://raw.githubusercontent.com/binaryu/emosup/main/docker-compose.yaml

cat > .env << EOF
EMOSUP_PORT=8080
EMOSUP_DATA_DIR=./data
EMOSUP_MEDIA_DIR=/path/to/your/media
TZ=Asia/Shanghai
EOF

docker compose up -d
```

| 用途 | 宿主机 | 容器 |
|------|--------|------|
| 应用数据 | `EMOSUP_DATA_DIR` | `/app/backend/data` |
| 下载缓存 | （在 data 卷内） | `.../downloads` |
| 本地媒体 | `EMOSUP_MEDIA_DIR` | `/media` |

Docker 的本地媒体根目录默认固定为容器内 `/media`（面板里改无效，因为环境变量优先）；如需换容器内路径，在 `.env` 中加一行 `EMOSUP_LOCAL_ROOT=/other/path` 并保证该路径已挂载进容器。

---

## 配置项（面板）

| 分类 | 字段 | 说明 |
|------|------|------|
| 服务 | 监听地址 / 端口 | HTTP 绑定 |
| 本地媒体 | 浏览根目录 / 下载缓存 / 上传后保留本地文件 | 二进制扫本机片库；BT/PT 或留档时开启保留 |
| 登录认证 | 用户名 / 密码 / Token 有效期 | JWT（默认 admin/admin） |
| OpenList | 地址 / 账密 / Token | 网盘 |
| Emos | 地址 / Token / 存储位置 | 上传目标 |
| qBittorrent | WebUI 地址 / 账密 / 保存目录 | 磁力/BT 下载（「BT 下载」页） |
| Worker | 并发 / 线程 / 分片 / 重试 / TMDB Key / 自动调优 | 调度 |

保存后热更新（并发等无需重启进程）。

**自动调优**（Worker → 自动调优，默认开启）：根据实测带宽（AIMD 拥塞控制式）与剩余磁盘空间，自动提高并行任务数、下载线程、上传分片与分片并发；只在用户设置值之上浮动，不会低于手动配置。

## 外部依赖

| 服务 | 用途 | 必需 |
|------|------|------|
| OpenList | 网盘浏览与下载 | 网盘扫描时 |
| Emos API | 视频树 + 上传 | 是 |
| TMDB API | 片名搜索 | 推荐 |
| qBittorrent | 磁力/BT 下载（WebUI API） | 使用 BT 下载时 |

## 开发

```bash
git clone git@github.com:binaryu/emosup.git
cd emosup

cd backend && go run ./cmd/server
cd frontend && npm install && npx vite
```

前端 `http://localhost:5173`，后端 `http://localhost:8080`。

## 目录结构

```
emosup/
├── backend/           # Go 服务
├── frontend/          # Vue 面板
├── scripts/
│   ├── install.sh     # 一键安装/更新/卸载
│   ├── emosup.service # systemd 模板
│   └── build-release.sh
├── docker-compose.yaml
└── Dockerfile
```
