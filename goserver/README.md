# Go Rewrite Skeleton

这个目录用于在新分支中逐步重建一个更稳定的 Go 后端。

## 当前已完成
- 基础 Go module
- HTTP 服务入口
- 配置加载
- 任务领域模型
- 内存任务仓库
- 事件总线
- 单 worker 队列管理器骨架
- SSE 事件推送接口
- 基础任务面板首页

## 当前接口
- `GET /`
- `GET /healthz`
- `GET /api/tasks`
- `POST /api/queue/add`
- `GET /api/events`

## 当前页面能力
访问根路径 [`/`](goserver/internal/api/http.go) 可看到一个基础任务面板，支持：
- 输入任务名称
- 加入队列
- 查看当前队列长度
- 查看 worker 状态
- 通过 SSE 实时刷新任务列表

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
- 将当前基础面板升级为完整业务页面
