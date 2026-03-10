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
- OpenList 远程扫描接口
- 本地扫描接口第一版
- scan_combined 合并扫描逻辑第一版
- EMOS tree 获取第一版
- 预检查逻辑第一版迁移
- 队列状态汇总第一版
- 日志事件流第一版
- 取消当前任务接口第一版
- 批量入队请求结构第一版
- 队列任务元数据承载第一版
- aria2 RPC 客户端骨架
- 下载进度更新与状态文案第一版
- 上传与保存服务骨架
- worker 下载 -> 上传 -> 保存主干第一版
- 基础页面状态展示增强
- legacy 参考代码目录

## 当前接口
- `GET /`
- `GET /healthz`
- `GET /api/tasks`
- `GET /api/status`
- `GET /api/queue/status`
- `POST /api/queue/add`
- `POST /api/cancel`
- `GET /api/events`
- `POST /api/scan_remote`
- `POST /api/scan_local`
- `POST /api/scan_combined`
- `POST /api/precheck`

## 当前页面能力
访问根路径 [`/`](goserver/internal/api/http.go) 可看到一个基础任务面板，支持：
- 输入任务名称
- 加入队列
- 取消当前任务
- 查看当前文件、状态文案、最近错误
- 查看下载/上传进度条
- 通过 SSE 实时刷新任务列表与日志

## legacy 参考代码
旧 Python 逻辑已经恢复到 [`legacy/`](legacy/)，仅作为迁移参考：
- [`legacy/app/tasks.py`](legacy/app/tasks.py)
- [`legacy/app/upload.py`](legacy/app/upload.py)
- [`legacy/app/aria2.py`](legacy/app/aria2.py)
- [`legacy/app/openlist.py`](legacy/app/openlist.py)
- [`legacy/app/clients.py`](legacy/app/clients.py)
- [`legacy/app/local_files.py`](legacy/app/local_files.py)
- [`legacy/app/main.py`](legacy/app/main.py)

## 运行方式
在 [`goserver/go.mod`](goserver/go.mod) 所在目录执行：

```bash
go run ./cmd/server
```

默认监听地址：`127.0.0.1:8081` 对应配置环境变量 `GO_EMOS_HTTP_ADDR`，默认值为 `:8081`。

## 下一步
- 继续把 worker 中的模拟逻辑替换为更完整的业务判断
- 完善本地直传、缓存清理、跳过策略
- 补齐更完整的业务表单页面
