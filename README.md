# emosup

OpenList / 本地文件 → Emos 异步转存面板。

## 特性

- **多源扫描** — 支持 OpenList 网盘和本地下载目录，递归子目录、单文件/多文件扫描
- **TMDB 智能匹配** — 搜索影片名自动获取 ID，支持剧集+电影同时搜索，进入目录自动提取名称
- **内建下载器** — 多线程分段下载、断点续传、3 次自动重试，无需 aria2
- **文件名解析** — 支持 `S01E02`、`[01]`、`EP03`、`第04集`、`01.mp4`、`第一季` 等格式
- **实时监控** — SSE 推送任务进度，磁盘空间显示，批量暂停/恢复/删除
- **一键部署** — Docker 镜像开箱即用，零依赖

## 快速开始

```bash
# 1. 下载 docker-compose.yaml
wget https://raw.githubusercontent.com/binaryu/emosup/main/docker-compose.yaml

# 2. 创建 .env（可选，自定义端口/目录）
cat > .env << EOF
EMOSUP_PORT=8080
EMOSUP_DATA_DIR=./data
# 本机已有的媒体库（可选，用于「本地媒体」扫描）
EMOSUP_MEDIA_DIR=/path/to/your/media
TZ=Asia/Shanghai
EOF

# 3. 启动
docker compose up -d
```

打开 `http://localhost:8080`，使用默认账号 `admin` / `admin` 登录，在配置页修改密码并填入外部服务凭证即可使用。

## 目录挂载

| 用途 | 宿主机 (`.env`) | 容器内 | 说明 |
|------|-----------------|--------|------|
| 应用数据 | `EMOSUP_DATA_DIR`（默认 `./data`） | `/app/backend/data` | 配置、任务、扫描记录 |
| 下载缓存 | （在 data 卷内） | `/app/backend/data/downloads` | OpenList 下载临时文件，无需单独挂 |
| 本地媒体 | `EMOSUP_MEDIA_DIR`（默认 `./media`） | `/media` | 面板「本地媒体」浏览/扫描的根目录 |

```
宿主机 media 库          容器
/path/to/your/media  ←→  /media          ← 本地扫描
./data               ←→  /app/backend/data
                         └── downloads/  ← 网盘下载缓存
```

> 不要再把宿主机下载目录嵌套挂到 `data/downloads` 上——下载缓存随 data 卷一起持久化即可。  
> 若要扫描本机已有影片，把目录映射到 `EMOSUP_MEDIA_DIR`，在面板选择「本地媒体」。

## 配置项

| 分类 | 字段 | 说明 |
|------|------|------|
| 服务 | 监听地址 / 端口 | HTTP 服务绑定 |
| 登录认证 | 用户名 / 密码 / Token 有效期 | 面板 JWT 登录（默认 admin/admin） |
| OpenList | 接口地址 / 用户名 / 密码 / Token | 填写网盘地址和登录凭证 |
| Emos | 接口地址 / Token / 存储位置 | Emos API 连接 |
| 任务调度 | 轮询间隔 / 最大并发 / 下载线程 / 分片大小 / 重试参数 | 调度策略 |

> 配置页右上角保存后立即生效，并发和线程等参数无需重启。

## 外部依赖

| 服务 | 用途 | 必需 |
|------|------|------|
| OpenList | 文件浏览与下载 | 网盘扫描时必需 |
| Emos API | 视频树匹配 + 上传 | 必需 |
| TMDB API | 影片搜索自动匹配 | 推荐（免费申请：themoviedb.org/settings/api） |

## 开发

```bash
git clone git@github.com:binaryu/emosup.git
cd emosup

# 后端（在仓库根或 backend 下均可）
cd backend && go run ./cmd/server

# 前端
cd frontend && npm install && npx vite
```

前端 `http://localhost:5173`，后端 `http://localhost:8080`。

直接跑二进制时：

| 路径 | 默认位置 |
|------|----------|
| 数据根 | `backend/data`（或 `EMOSUP_DATA_DIR`） |
| 数据库 | `{数据根}/emosup.db`（SQLite WAL） |
| 下载缓存 / 本地浏览 | `{数据根}/downloads`（启动时自动创建） |

可用 `EMOSUP_LOCAL_ROOT` 覆盖本地浏览根目录（例如指向你的媒体库）。

## 目录结构

```
emosup/
├── backend/
│   ├── cmd/server/
│   ├── internal/
│   │   ├── app/        # 启动
│   │   ├── client/     # 外部 API（OpenList/Emos/TMDB）
│   │   ├── eventbus/   # SSE 事件总线
│   │   ├── handler/    # HTTP 路由
│   │   ├── model/      # 数据模型
│   │   ├── scheduler/  # 任务调度
│   │   ├── service/    # 业务逻辑
│   │   ├── store/      # SQLite 存储
│   │   └── utils/      # 工具（解析器等）
│   └── data/           # 运行时数据 (emosup.db / downloads)
├── frontend/
│   └── src/
│       ├── components/
│       ├── layouts/
│       ├── stores/
│       ├── views/
│       └── utils/
├── docker-compose.yaml
└── Dockerfile
```
