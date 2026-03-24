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

第一次运行不需要手动复制配置文件，`docker-compose.yaml` 会在命名卷里自动初始化 `config.json`。

2. 修改配置文件

推荐做法：

- 先直接启动服务
- 打开 Web 界面的“配置页”
- 填好 OpenList、aria2、Emos 参数后点击保存

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
docker compose up -d
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
docker compose up -d
docker compose down
```

说明：

- 当前 `docker-compose.yaml` 直接使用远端镜像，默认值是 `ghcr.io/binaryu/emosup:beta-20260323-214646`
- 数据目录使用 Docker 命名卷 `emosup_data`，比直接挂宿主机目录更不容易遇到权限问题
- 首次启动会自动生成 `config.json`，并把 `server.host` 调整为 `0.0.0.0`
- 镜像内已经包含后端二进制和前端静态文件，不会在本地重新编译
- 生产访问入口只有后端服务，前端由后端直接托管
- 如需更换镜像标签，可在启动前设置 `EMOSUP_IMAGE`
- 如需改对外端口，可在启动前设置 `EMOSUP_PORT`

## 开发依赖

- `Go 1.25+`
- `Node.js 20+`
- `aria2` RPC
- 可用的 `OpenList` 和 `Emos`

## 本地直接运行

如果你不想用 Docker，现在可以直接从源码启动后端：

```bash
go run ./backend/cmd/server
```

说明：

- 第一次启动会自动创建 `backend/data/config.json`
- 后端会自动查找并托管前端静态文件
- 打开 `http://127.0.0.1:8080` 后，可以直接在“配置页”里补全 OpenList、aria2、Emos 参数并保存

## 发布包运行

下载 GitHub Release 里的发布包后，解压并直接运行二进制：

```bash
./emosup-server
```

说明：

- 第一次启动会自动创建 `data/config.json`
- 前端静态文件已经包含在发布包里，不需要再单独构建前端
- 默认访问地址是 `http://127.0.0.1:8080`

## 配置

配置文件会在第一次启动时自动生成。

源码运行时默认位置：

```text
backend/data/config.json
```

发布包运行时默认位置：

```text
data/config.json
```

如果你想在启动前手动准备配置，也可以参考：

```text
backend/data/config.example.json
```

至少需要按你的环境修改：

- `openlist.base_url`
- `openlist.username/password` 或 `token`
- `aria2.rpc_url`
- `aria2.download_dir`
- `emos.base_url`
- `emos.token`

## 开发启动

1. 启动后端

```bash
go run ./backend/cmd/server
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
- 如果只是想把应用跑起来，不需要这一步，后端会直接托管已构建的前端页面

## 生产启动

如果你是从源码构建，先打前端产物，然后直接启动后端：

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

如果你使用 GitHub Release 发布包，则只需要：

```bash
./emosup-server
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

发布包结构现在是扁平的，解压后直接运行根目录下的 `./emosup-server` 即可。

## 说明

这个仓库只包含应用本身。

完整跑通链路仍然需要你自己准备：

- OpenList
- aria2 RPC
- Emos
