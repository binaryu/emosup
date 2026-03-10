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
- EMOS tree 获取第一版
- 预检查逻辑第一版迁移
- legacy 参考代码目录

## 当前接口
- `GET /`
- `GET /healthz`
- `GET /api/tasks`
- `POST /api/queue/add`
- `GET /api/events`
- `POST /api/scan_remote`
- `POST /api/scan_local`
- `POST /api/precheck`

## 当前页面能力
访问根路径 [`/`](goserver/internal/api/http.go) 可看到一个基础任务面板，支持：
- 输入任务名称
- 加入队列
- 查看当前队列长度
- 查看 worker 状态
- 通过 SSE 实时刷新任务列表

## legacy 参考代码
旧 Python 逻辑已经恢复到 [`legacy/`](legacy/)，仅作为迁移参考：
- [`legacy/app/tasks.py`](legacy/app/tasks.py)
- [`legacy/app/upload.py`](legacy/app/upload.py)
- [`legacy/app/aria2.py`](legacy/app/aria2.py)
- [`legacy/app/openlist.py`](legacy/app/openlist.py)
- [`legacy/app/clients.py`](legacy/app/clients.py)
- [`legacy/app/local_files.py`](legacy/app/local_files.py)

## 运行方式
在 [`goserver/go.mod`](goserver/go.mod) 所在目录执行：

```bash
go run ./cmd/server
```

默认监听地址：`127.0.0.1:8081` 对应配置环境变量 `GO_EMOS_HTTP_ADDR`，默认值为 `:8081`。

## 下一步
- 迁移 scan_combined 合并扫描逻辑
- 迁移队列执行、取消和日志事件
- 迁移 aria2 下载和上传链路
- 将当前基础面板升级为完整业务页面
