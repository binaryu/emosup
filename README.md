# emosup

OpenList -> aria2 -> Emos 的异步上传面板。

提供：

- OpenList 扫描与视频识别
- Emos 条目匹配
- aria2 下载
- Emos 分片上传与 save
- 任务队列、日志、失败重试

## Docker Compose 快速启动

如果你只是想把服务跑起来，优先用这一节。

前置条件：

- 已安装 `Docker` 和 `Docker Compose`
- 你已经准备好了可访问的 `OpenList`
- 你已经准备好了可访问的 `aria2 RPC`
- 你已经准备好了可访问的 `Emos`

1. 初始化配置文件

```bash
cp backend/data/config.example.json backend/data/config.json
```

2. 修改配置文件

文件位置：

```text
backend/data/config.json
```

至少需要确认这些字段：

- `server.host`
- `server.port`
- `openlist.base_url`
- `openlist.username/password` 或 `openlist.token`
- `aria2.rpc_url`
- `aria2.secret`
- `aria2.download_dir`
- `emos.base_url`
- `emos.token`
- `emos.storage`

3. 启动服务

```bash
docker compose up -d --build
```

4. 查看状态

```bash
docker compose ps
docker compose logs -f emosup
```

5. 打开页面

```text
http://127.0.0.1:8080
```

常用命令：

```bash
docker compose restart emosup
docker compose pull
docker compose up -d --build
docker compose down
```

说明：

- 当前 `docker-compose.yml` 会把本地 `./backend/data` 挂载到容器内 `/app/backend/data`
- 镜像内已经包含后端二进制和前端静态文件
- 生产访问入口只有后端服务，前端由后端直接托管

## 开发依赖

- `Go 1.25+`
- `Node.js 20+`
- `aria2` RPC
- 可用的 `OpenList` 和 `Emos`

## 配置

配置文件位置：

```text
backend/data/config.json
```

初始化：

```bash
cp backend/data/config.example.json backend/data/config.json
```

然后按你的环境修改：

- `openlist.base_url`
- `openlist.username/password` 或 `token`
- `aria2.rpc_url`
- `aria2.download_dir`
- `emos.base_url`
- `emos.token`

## 开发启动

1. 启动后端

```bash
cd backend
go run ./cmd/server
```

2. 启动前端

```bash
cd frontend
npm install
npm run dev
```

3. 打开页面

```text
http://127.0.0.1:5173
```

说明：

- 前端请求统一走 `/api`
- Vite 会把 `/api` 代理到 `http://127.0.0.1:8080`

## 生产启动

前端构建后，后端会直接托管前端页面，所以生产模式只需要启动后端。

```bash
cd frontend
npm install
npm run build

cd ../backend
go run ./cmd/server
```

访问：

```text
http://127.0.0.1:8080
```

## 构建与测试

前端构建：

```bash
cd frontend
npm run build
```

后端测试：

```bash
cd backend
go test ./...
```

## Beta 发布

推送 `beta` 或 `beta-*` 标签时会自动：

- 构建前端
- 运行后端测试
- 生成 `linux-x64` 发布包
- 发布到 GitHub Release
- 推送 Docker 镜像到 `ghcr.io/binaryu/emosup`

常用命令：

```bash
git tag beta
git push origin main
git push origin refs/tags/beta --force
```

拉取 beta 镜像：

```bash
docker pull ghcr.io/binaryu/emosup:beta
```

## 说明

这个仓库只包含应用本身。

完整跑通链路仍然需要你自己准备：

- OpenList
- aria2 RPC
- Emos
