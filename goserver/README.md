# Go Rewrite Skeleton

这个目录用于在不删除现有 Python 项目的前提下，逐步重建一个更稳定的 Go 后端。

## 当前已完成
- 基础 Go module
- HTTP 服务入口
- 配置加载
- 任务领域模型
- 内存任务仓库
- 事件总线
- 单 worker 队列管理器骨架
- SSE 事件推送接口

## 当前接口
- `GET /healthz`
- `GET /api/tasks`
- `POST /api/queue/add`
- `GET /api/events`

## 运行方式
在 [`goserver/go.mod`](goserver/go.mod) 所在目录执行：

```bash
go run ./cmd/server
```

默认监听地址：`127.0.0.1:8081` 对应配置环境变量 `GO_EMOS_HTTP_ADDR`，默认值为 `:8081`。

## 下一步
- 接入真实任务执行器，替换当前模拟完成逻辑
- 接入 OpenList 扫描和本地扫描
- 接入 EMOS 预检查
- 接入 aria2 下载与上传流程
- 增加取消任务与日志流
- 迁移前端状态面板到新接口
