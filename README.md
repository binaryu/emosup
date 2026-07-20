# emosup

OpenList / 本地文件 → Emos 异步转存面板。

## 特性

- **多源扫描** — OpenList 网盘与本机目录，递归子目录、多文件勾选
- **TMDB 匹配** — 搜片名取 ID，进目录自动猜剧名
- **内建下载器** — 多线程分段、断点续传、自动重试
- **文件名解析** — `S01E02`、`[01]`、`EP03`、`第04集`、`01.mp4`、`第一季` 等
- **实时进度** — SSE 推送速度/进度
- **SQLite 存储** — 任务进度高频写入不再卡在 JSON 文件上
- **一键二进制部署** — 安装 / 更新 / 卸载脚本 + systemd

## 快速开始

```bash

curl -fsSL https://raw.githubusercontent.com/binaryu/emosup/main/scripts/install.sh | sudo bash
```

打开 `http://服务器IP:端口`，默认账号 **`admin` / `admin`**（请立刻改密码）。

### 常用命令

```bash
# 安装指定版本
curl -fsSL https://raw.githubusercontent.com/binaryu/emosup/main/scripts/install.sh \
  | sudo bash -s -- install --version v1.0.0

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
| `EMOSUP_DATA_DIR` | 数据目录 | `/opt/emosup/data` |
| `EMOSUP_FRONTEND_DIST` | 前端目录 | `/opt/emosup/frontend` |
| `EMOSUP_LOCAL_ROOT` | 本地浏览根目录（可选） | 面板配置 / downloads |

本地媒体目录也可在面板 **系统配置 → 本地媒体** 里改，无需改环境变量。

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

---

## 配置项（面板）

| 分类 | 字段 | 说明 |
|------|------|------|
| 服务 | 监听地址 / 端口 | HTTP 绑定 |
| 本地媒体 | 浏览根目录 / 下载缓存 | 二进制扫本机片库 |
| 登录认证 | 用户名 / 密码 / Token 有效期 | JWT（默认 admin/admin） |
| OpenList | 地址 / 账密 / Token | 网盘 |
| Emos | 地址 / Token / 存储位置 | 上传目标 |
| Worker | 并发 / 线程 / 分片 / 重试 / TMDB Key | 调度 |

保存后热更新（并发等无需重启进程）。

## 外部依赖

| 服务 | 用途 | 必需 |
|------|------|------|
| OpenList | 网盘浏览与下载 | 网盘扫描时 |
| Emos API | 视频树 + 上传 | 是 |
| TMDB API | 片名搜索 | 推荐 |

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
