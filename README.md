# emosup

OpenList -> aria2 -> Emos 的异步上传面板。

当前仓库已经完成到第 6 阶段：

- 扫描 OpenList 目录并识别视频文件
- 调用 Emos 视频树并自动匹配目标条目
- 从扫描结果创建任务
- scheduler 自动消费任务
- 通过 aria2 下载文件
- 下载完成后自动进入上传阶段
- 调用 Emos 上传 token、分片上传、save
- save 等待型错误自动重试
- 服务重启后的基础恢复、失败收敛、结构化错误展示

可以运行。

要注意两点：

1. 现在的前后端应用本身已经能启动和联调。
2. 如果你要完整测试“下载 -> 上传 -> save -> completed”主链路，仍然需要你自己准备可用的 `OpenList`、`aria2`、`Emos` 环境和对应凭据。

## 目录结构

```text
docs/                 设计文档
backend/              Go 后端
frontend/             Vue 3 前端
upload.md             Emos 上传接口补充说明
.github/workflows/    GitHub Actions 自动构建
```

## 运行环境

建议在 Linux 上准备：

- `Go 1.25.x`
- `Node.js 20+`
- `npm 10+`
- `aria2c`

说明：

- 后端是 Go 项目，Linux 直接构建运行即可。
- 前端是 Vite + Vue 3 项目，开发模式需要 Node。
- 真正执行下载任务时，需要 aria2 的 JSON-RPC 服务。

## 快速开始

### 1. 安装前端依赖

```bash
cd frontend
npm install
```

### 2. 启动 aria2 RPC

最简单的本地测试命令：

```bash
mkdir -p ~/aria2-downloads

aria2c \
  --enable-rpc=true \
  --rpc-listen-port=6800 \
  --rpc-allow-origin-all=true \
  --rpc-listen-all=false \
  --continue=true \
  --max-concurrent-downloads=1 \
  --dir="$HOME/aria2-downloads"
```

如果你要设置密钥，可以加上：

```bash
--rpc-secret=your-secret
```

然后把它填到配置页或 `backend/data/config.json` 里。

### 3. 启动后端

请从 `backend` 目录运行，因为后端默认把数据存到当前目录下的 `./data`：

```bash
cd backend
go mod tidy
go run ./cmd/server
```

默认地址：

- 后端 API: `http://127.0.0.1:8080`

如果你只想先验证后端能不能起，也可以直接运行这一步。即使没配 OpenList/Emos，服务本身也能先启动。

### 4. 启动前端

新开一个终端：

```bash
cd frontend
npm run dev
```

默认地址：

- 前端页面: `http://127.0.0.1:5173`

## Linux 上的推荐启动顺序

```bash
# 终端 1
aria2c --enable-rpc=true --rpc-listen-port=6800 --rpc-allow-origin-all=true --continue=true --dir="$HOME/aria2-downloads"

# 终端 2
cd backend
go run ./cmd/server

# 终端 3
cd frontend
npm run dev
```

然后浏览器打开：

```text
http://127.0.0.1:5173
```

## 首次配置

后端配置文件位置：

```text
backend/data/config.json
```

仓库里提供了示例文件：

```text
backend/data/config.example.json
```

你可以：

1. 直接先启动后端和前端
2. 在前端“配置页”里填写配置并保存

也可以先手动写一个 Linux 用的配置文件，例如：

```json
{
  "server": {
    "host": "127.0.0.1",
    "port": 8080
  },
  "openlist": {
    "base_url": "http://127.0.0.1:5244",
    "username": "",
    "password": "",
    "token": ""
  },
  "aria2": {
    "rpc_url": "http://127.0.0.1:6800/jsonrpc",
    "secret": "",
    "download_dir": "/home/your-user/aria2-downloads",
    "poll_interval_seconds": 3,
    "connect_timeout_seconds": 10
  },
  "emos": {
    "base_url": "https://emos.best",
    "token": "your-emos-token",
    "storage": "default"
  },
  "worker": {
    "poll_interval_seconds": 5,
    "upload_chunk_size_mb": 8,
    "save_retry_interval_seconds": 25,
    "save_retry_max_attempts": 8
  }
}
```

把 `/home/your-user/aria2-downloads` 改成你自己的 Linux 目录。

## 如何确认服务已经跑起来

### 后端健康检查

浏览器或命令行访问：

```bash
curl http://127.0.0.1:8080/api/health
```

### scheduler 运行状态

```bash
curl http://127.0.0.1:8080/api/system/runtime
```

### recovery 摘要

```bash
curl http://127.0.0.1:8080/api/system/recovery
```

## 完整链路测试前提

如果你想真正测通：

```text
扫描 -> 创建任务 -> 下载 -> 上传 -> save -> completed
```

你至少需要：

- 可访问的 OpenList 服务
- 可用的 OpenList 文件直链
- 正常工作的 aria2 RPC
- 可用的 Emos `base_url`
- 可用的 Emos `token`
- Emos 侧存在能匹配/上传的目标视频条目

如果这些外部依赖没配好，应用仍然能启动，但任务执行会失败，并在任务详情里显示结构化错误信息。

## 当前运行方式

### 开发模式

前后端分开跑：

- 前端：Vite dev server
- 后端：Go API server

### 前端构建

```bash
cd frontend
npm run build
```

### 统一访问模式

前端执行过构建后，后端会自动尝试托管静态页面：

- 在源码目录开发时，会优先识别 `../frontend/dist`
- 在发布压缩包里，会优先识别与 `backend/` 同级的 `frontend/`

也就是说，打包好的 Linux 产物解压后，通常只要启动后端，就能通过同一个端口访问界面和 API。

### 后端测试

```bash
cd backend
go test ./...
```

## GitHub Beta 自动构建

仓库内置了 GitHub Actions 工作流：

- 推送 `beta` 或 `beta-*` 标签时自动触发
- 构建前端静态文件
- 运行后端测试
- 交叉编译 `linux-x64` 后端二进制
- 打包成 `emosup-linux-x64.tar.gz`
- 自动附加到 GitHub Release（预发布）

常用命令：

```bash
git tag beta
git push origin main --follow-tags
```

## 常见问题

### 1. 后端启动了，但创建任务后下载失败

优先检查：

- `aria2c` 是否真的已启动
- `rpc_url` 是否正确
- `secret` 是否一致
- OpenList 的 `raw_url` 是否可访问

### 2. 上传阶段失败

优先检查：

- `emos.base_url`
- `emos.token`
- 目标条目的 `item_type/item_id` 是否有效
- Emos save 是否返回等待型错误或致命错误

### 3. Linux 路径不生效

请确认你填写的是 Linux 实际路径，例如：

```text
/home/ubuntu/aria2-downloads
```

不要填 Windows 路径。

### 4. 服务重启后任务状态不对

当前实现下：

- `saving` 会尽量恢复继续 save
- `uploading` 会收敛为 `upload_failed`
- `downloading` 会尝试和 aria2 / 本地文件状态对账

这是第 6 阶段的预期行为。

## 一句话结论

现在这个项目已经可以跑。

如果你只是想在 Linux 上把界面和后端服务启动起来，按上面的步骤即可。
如果你要验证完整业务链路，还需要你准备真实的 OpenList、aria2、Emos 配置。
