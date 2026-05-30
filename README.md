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
EMOSUP_DOWNLOADS_DIR=/home/user/downloads
TZ=Asia/Shanghai
EOF

# 3. 启动
docker compose up -d
```

打开 `http://localhost:8080`，在配置页填入凭证即可使用。

## 目录挂载

Docker 容器内下载路径为 `/app/backend/data/downloads`，通过 docker-compose 映射到宿主机：

```
宿主机                          容器内
/home/user/downloads    ←→    /app/backend/data/downloads
```

`.env` 中设置 `EMOSUP_DOWNLOADS_DIR` 指向你的下载目录即可。容器内本地浏览功能使用 `EMOSUP_LOCAL_ROOT` 环境变量（默认即上述容器路径）。

## 配置项

| 分类 | 字段 | 说明 |
|------|------|------|
| 服务 | 监听地址 / 端口 | HTTP 服务绑定 |
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

# 后端
cd backend && go run ./cmd/server

# 前端
cd frontend && npm install && npx vite
```

前端 `http://localhost:5173`，后端 `http://localhost:8080`。

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
│   │   ├── store/      # JSON 存储
│   │   └── utils/      # 工具（解析器等）
│   └── data/           # 运行时数据
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
