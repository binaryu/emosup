# emosup

OpenList / 本地 → aria2 → Emos 异步转存面板。

## 特性

- **影片扫描** — 浏览 OpenList 或本地下载目录，支持目录、单文件、多文件三种扫描模式
- **TMDB 自动匹配** — 搜索影片名自动获取 TMDB ID，进入目录自动提取剧名填充搜索框
- **智能解析** — 支持 `S01E02`、`[01]`、`EP03`、`第04集`、`Name - 05.mkv` 等命名格式
- **任务队列** — SSE 实时推送进度，可配置并发数，支持取消/重试/删除/批量删除
- **一键部署** — Docker 镜像开箱即用，无需安装 Go/Node

## 快速开始

```bash
# 下载 docker-compose.yaml
wget https://raw.githubusercontent.com/binaryu/emosup/main/docker-compose.yaml

# 启动
docker compose up -d
```

打开 `http://localhost:8080`，在配置页填入外部服务凭证即可使用。

## 外部依赖

| 服务 | 用途 | 必需 |
|------|------|------|
| OpenList | 文件浏览与直链获取 | 可选（可用本地目录替代） |
| aria2 RPC | 下载管理 | 下载 OpenList 文件时需要 |
| Emos API | 视频树匹配 + 上传 | 必需 |
| TMDB API | 影片搜索自动匹配 | 推荐（免费申请：https://www.themoviedb.org/settings/api） |

## 开发

```bash
git clone git@github.com:binaryu/emosup.git
cd emosup

# 后端 (需要 Go 1.25+)
cd backend
go install github.com/air-verse/air@latest
air

# 前端 (需要 Node 20+)
cd frontend
npm install
npx vite --host 0.0.0.0
```

前端 `http://localhost:5173`，后端 `http://localhost:8080`。

## 配置项

| 分类 | 字段 | 说明 |
|------|------|------|
| 服务 | 监听地址 / 端口 | HTTP 服务绑定 |
| OpenList | 接口地址 / 用户名 / 密码 / Token | OpenList 连接 |
| Emos | 接口地址 / Token / 存储位置 | Emos API 连接 |
| aria2 | RPC 地址 / 密钥 / 下载目录 | aria2 连接 |
| 任务调度 | 轮询间隔 / 最大并发 / 分片大小 / 重试参数 | 调度策略 |
| TMDB API Key | API Key | 影片搜索 |

## 目录结构

```
emosup/
├── backend/           # Go 后端
│   ├── cmd/server/    # 入口
│   ├── internal/
│   │   ├── app/       # 应用启动
│   │   ├── client/    # 外部 API 客户端
│   │   ├── eventbus/  # SSE 事件总线
│   │   ├── handler/   # HTTP 路由
│   │   ├── model/     # 数据模型
│   │   ├── scheduler/ # 任务调度
│   │   ├── service/   # 业务逻辑
│   │   ├── store/     # JSON 文件存储
│   │   └── utils/     # 工具
│   └── data/          # 运行时数据
├── frontend/          # Vue3 前端
│   └── src/
│       ├── components/
│       ├── layouts/
│       ├── stores/
│       ├── types/
│       ├── utils/
│       └── views/
├── docker-compose.yaml
├── Dockerfile
└── docs/
```
