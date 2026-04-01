# emosup

OpenList -> aria2 -> Emos 的异步上传面板。

提供：

- OpenList 扫描与视频识别
- Emos 条目匹配
- aria2 下载
- Emos 分片上传与 save
- 任务队列、日志、失败重试

## 运行方式

### 1. Docker Compose

如果只是想尽快把服务跑起来，优先用这一种。

前置条件：

- 已安装 `Docker` 和 `Docker Compose`
- 已准备好可访问的 `OpenList`
- 已准备好可访问的 `aria2 RPC`
- 已准备好可访问的 `Emos`

启动：

```bash
docker compose up -d
```

查看状态：

```bash
docker compose ps
docker compose logs -f emosup
```

访问：

```text
http://127.0.0.1:8080
```

说明：

- `docker-compose.yaml` 默认使用远端镜像 `ghcr.io/binaryu/emosup:beta-20260323-214646`
- 首次启动会自动初始化配置文件
- 容器内前端已经构建完成，由后端直接托管
- 如需更换镜像标签，可在启动前设置 `EMOSUP_IMAGE`
- 如需改对外端口，可在启动前设置 `EMOSUP_PORT`
- 默认使用宿主机目录 `./data` 与 `./downloads` 作为挂载（可用环境变量修改）
- `EMOSUP_DATA_DIR`：配置/任务/日志等数据目录
- `EMOSUP_DOWNLOADS_DIR`：aria2 下载目录（必须与 aria2 实际下载目录一致）
- `EMOSUP_UID` / `EMOSUP_GID`：容器内运行用户 ID（建议设为宿主机目录所属用户）

### 1.2 Docker Compose（全量栈：OpenList + aria2 + emosup）

如果你希望一个 `yml` 把 OpenList、aria2 与 emosup 全部跑起来，使用 `docker-compose.full.yaml`。

启动：

```bash
docker compose -f docker-compose.full.yaml up -d
```

首次启动后，请在配置页把以下字段改成容器内可访问的地址：

- `openlist.base_url`：`http://openlist:5244`
- `aria2.rpc_url`：`http://aria2:6800/jsonrpc`
- `aria2.secret`：与 `ARIA2_RPC_SECRET` 保持一致（默认 `P3TERX`，建议改掉）

可选环境变量：

- `OPENLIST_DATA_DIR`：OpenList 数据目录（默认 `./openlist-data`）
- `OPENLIST_PORT`：OpenList 对外端口（默认 `5244`）
- `OPENLIST_UID` / `OPENLIST_GID`：OpenList 运行用户
- `ARIA2_CONFIG_DIR`：aria2 配置目录（默认 `./aria2-config`）
- `ARIA2_RPC_SECRET`：aria2 RPC 密钥
- `ARIA2_RPC_PORT`：aria2 RPC 端口（默认 `6800`）
- `ARIA2_LISTEN_PORT`：aria2 BT 监听端口（默认 `6888`）

### 1.3 一键部署脚本

为了避免每次手动处理权限、配置与端口，你可以直接用脚本完成“全量栈”或“单服务”部署。

全量栈（OpenList + aria2 + emosup）：

```bash
./deploy.sh full
```

仅 emosup：

```bash
./deploy.sh lite
```

脚本会自动：

- 生成 `.env`（首次运行）
- 创建并修正数据目录权限
- 生成 `data/config.json`（首次运行）
- 启动容器并输出连通性检查结果

### 1.1 Docker Compose（本地源码构建）

如果你在本地做过重构或修改了前端/后端代码，建议用本地构建，避免“前后端版本不一致”的问题。

启动：

```bash
docker compose -f docker-compose.local.yaml up -d --build
```

查看日志：

```bash
docker compose -f docker-compose.local.yaml logs -f emosup
```

说明：

- `docker-compose.local.yaml` 会从本地源码构建镜像
- 适合验证当前工作区的改动

### 2. 源码直接运行

如果你不想用 Docker，可以直接从仓库根目录启动：

```bash
go run ./backend/cmd/server
```

访问：

```text
http://127.0.0.1:8080
```

说明：

- 第一次启动会自动创建 `backend/data/config.json`
- 后端会自动查找并托管前端静态文件
- 如果仓库里已有 `frontend/dist`，只启动后端就够了
- 如果你改过前端页面，请先 `npm run build` 重新生成 `frontend/dist`

### 3. 发布包运行

下载 GitHub Release 的发布包后，解压并直接运行：

```bash
./emosup-server
```

访问：

```text
http://127.0.0.1:8080
```

说明：

- 第一次启动会自动创建 `data/config.json`
- 发布包已经包含前端静态文件，不需要额外构建前端

## 配置

配置文件会在第一次启动时自动生成。

常见位置：

- 源码运行：`backend/data/config.json`
- 发布包运行：`data/config.json`
- 参考模板：`backend/data/config.example.json`

至少需要确认这些字段：

- `openlist.base_url`
- `openlist.username/password` 或 `openlist.token`
- `aria2.rpc_url`
- `aria2.secret`
- `aria2.download_dir`
- `emos.base_url`
- `emos.token`
- `emos.storage`

推荐做法：

- 先把服务启动起来
- 打开 Web 界面的“配置页”
- 填好参数后点击保存

## 开发

开发依赖：

- `Go 1.25+`
- `Node.js 20+`
- `aria2` RPC
- 可用的 `OpenList` 和 `Emos`

后端：

```bash
go run ./backend/cmd/server
```

前端开发服务器：

```bash
cd frontend
npm install
npm run dev
```

访问：

```text
http://127.0.0.1:5173
```

说明：

- 前端请求统一走 `/api`
- Vite 会把 `/api` 代理到 `http://127.0.0.1:8080`

构建与测试：

```bash
cd frontend
npm run build

cd ../backend
go test ./...
```

## 发布

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

发布包解压后，直接运行根目录下的 `./emosup-server` 即可。

## 说明

这个仓库只包含应用本身。

完整跑通链路仍然需要你自己准备：

- OpenList
- aria2 RPC
- Emos
