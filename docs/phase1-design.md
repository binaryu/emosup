# 第一阶段设计说明

## 1. 系统架构说明

### 1.1 模块划分

系统按“任务队列 + 后台执行”原则拆分为以下模块：

- `frontend`
  - 配置管理
  - OpenList 目录浏览
  - 扫描结果与匹配确认
  - 任务列表与详情
- `handler`
  - Gin HTTP 接口层
  - 只负责参数校验、响应组装、错误码映射
- `service`
  - 核心业务编排
  - 承担扫描、匹配、任务创建、状态流转
- `store`
  - 基于 JSON 文件的持久化
  - 管理配置、扫描会话、任务、任务日志
- `client`
  - OpenList、aria2、Emos 三类外部系统客户端
- `scheduler`
  - 启动恢复、任务轮询、串行 worker、取消与重试
- `model`
  - 配置、扫描、任务、日志等领域模型
- `utils`
  - 原子写入、ID 生成、时间与文件辅助能力

### 1.2 执行流程

#### A. 扫描与匹配

1. 前端在 OpenList 页面浏览目录。
2. 用户选择目录并输入 `tmdb_id`。
3. 后端扫描目录中的视频文件。
4. 后端为每个文件获取 OpenList 直链。
5. 后端调用 Emos 视频树接口。
6. 后端解析文件名与路径中的季集信息。
7. 后端生成自动匹配建议。
8. 扫描结果保存为 `scan session`。
9. 前端进入匹配确认页，允许人工修正。
10. 用户勾选条目后批量创建任务。

#### B. 后台执行

1. 任务以单文件粒度写入 `data/tasks/task_*.json`。
2. 后端 scheduler 启动后恢复任务状态。
3. worker 轮询找到第一个 `queued` 任务。
4. 通过 aria2 下载 OpenList 直链文件。
5. 下载成功后准备 Emos 上传 token。
6. 执行分片上传并更新进度。
7. 上传完成后调用保存接口。
8. 若 422 且命中“视频正在合并中”则延迟重试保存。
9. 成功标记 `completed`，失败记录日志并转失败态。

### 1.3 数据流向

- 配置流
  - 前端配置页 -> `ConfigHandler` -> `ConfigService` -> `ConfigStore` -> `data/config.json`
- 扫描流
  - 前端扫描请求 -> `ScanHandler` -> `ScanService`
  - `ScanService` -> `OpenListClient` + `EmosClient`
  - 扫描结果 -> `ScanStore` -> `data/scans/scan_*.json`
- 任务流
  - 前端确认入队 -> `TaskHandler` -> `TaskService`
  - 任务快照 -> `TaskStore` -> `data/tasks/task_*.json`
- 执行流
  - `Scheduler` -> `TaskStore` -> `Aria2Client` -> `EmosClient`
  - 执行日志 -> `TaskLogStore` -> `data/logs/task_*.json`

### 1.4 外部系统关系

- OpenList
  - 提供目录浏览、文件元数据、下载直链
- aria2
  - 负责接收下载任务、查询下载状态
- Emos
  - 提供视频树、基础信息、上传 token、分片上传、保存接口

后端统一代理所有外部系统交互，前端不直连 OpenList、aria2、Emos。

## 2. 数据模型设计

### 2.1 AppConfig

```json
{
  "server": {
    "host": "127.0.0.1",
    "port": 8080
  },
  "openlist": {
    "base_url": "",
    "username": "",
    "password": "",
    "token": ""
  },
  "aria2": {
    "rpc_url": "http://127.0.0.1:6800/jsonrpc",
    "secret": "",
    "download_dir": "./downloads"
  },
  "emos": {
    "base_url": "https://emos.best",
    "token": "",
    "storage": "default"
  },
  "worker": {
    "poll_interval_seconds": 5,
    "save_retry_interval_seconds": 25,
    "save_retry_max_attempts": 8
  }
}
```

### 2.2 ScanSession

- `id`
- `directory`
- `tmdb_id`
- `video_type`
- `status`
- `total`
- `matched`
- `unmatched`
- `created_at`
- `updated_at`
- `items`

### 2.3 ScanItem

- `id`
- `file_name`
- `file_path`
- `size`
- `direct_url`
- `media_kind`
- `parsed_season`
- `parsed_episode`
- `match_status`
- `candidate_item_type`
- `candidate_item_id`
- `confirmed_item_type`
- `confirmed_item_id`
- `target_title`
- `selected`
- `reason`

### 2.4 Task

- `id`
- `scan_session_id`
- `source`
  - `openlist_path`
  - `direct_url`
  - `file_name`
  - `file_size`
- `parse_result`
  - `season`
  - `episode`
  - `video_type`
- `target`
  - `tmdb_id`
  - `item_type`
  - `item_id`
  - `target_title`
- `runtime`
  - `gid`
  - `download_path`
  - `uploaded_file_id`
  - `media_id`
  - `progress`
  - `error_message`
- `status`
- `created_at`
- `updated_at`
- `started_at`
- `completed_at`

### 2.5 TaskLog

- `task_id`
- `entries[]`
  - `id`
  - `level`
  - `stage`
  - `message`
  - `timestamp`

## 3. JSON 存储设计

### 3.1 目录结构

```text
backend/data/
  config.json
  scans/
    scan_{scan_id}.json
  tasks/
    task_{task_id}.json
  logs/
    task_{task_id}.json
```

### 3.2 文件命名

- 扫描会话：`scan_{yyyyMMddHHmmss}_{shortid}.json`
- 任务：`task_{yyyyMMddHHmmss}_{shortid}.json`
- 日志：`task_{task_id}.json`

### 3.3 原子写入方案

每次写入遵循以下顺序：

1. 序列化到内存
2. 写入同目录临时文件 `*.tmp`
3. `fsync`
4. 使用重命名替换正式文件

Windows 下重命名存在被占用风险，因此约定：

- 单进程内由 `store` 层串行控制写入
- 同一任务写入由任务级互斥保护
- 临时文件与正式文件位于同目录，降低跨卷风险

### 3.4 并发访问建议

- MVP 只允许单进程持有写权限
- `store` 内部使用互斥锁管理任务与日志文件写入
- `scheduler` 与 HTTP 接口统一通过 service/store 修改状态
- 不允许 handler 直接改文件

## 4. 任务状态机设计

状态集合：

- `draft`
- `queued`
- `downloading`
- `download_failed`
- `download_completed`
- `upload_pending`
- `uploading`
- `upload_failed`
- `completed`
- `canceled`

状态流转：

```text
draft -> queued
queued -> downloading
downloading -> download_completed
downloading -> download_failed
download_completed -> upload_pending
upload_pending -> uploading
uploading -> completed
uploading -> upload_failed
queued/downloading/upload_pending/uploading -> canceled
download_failed/upload_failed -> queued
```

补充约束：

- 创建任务后立即从 `draft` 写入为 `queued`
- 恢复时若发现中间态，可按安全策略回退
  - `downloading` -> `queued`
  - `uploading` -> `upload_pending`
- `canceled` 为终态
- `completed` 为终态

## 5. REST API 设计

### 5.1 配置接口

- `GET /api/config`
  - 获取当前配置
- `PUT /api/config`
  - 保存配置
- `POST /api/config/validate`
  - 校验 OpenList / aria2 / Emos 连通性

### 5.2 OpenList 接口

- `GET /api/openlist/tree?path=/`
  - 浏览目录
- `GET /api/openlist/file?path=...`
  - 获取单文件元数据

### 5.3 扫描接口

- `POST /api/scans`
  - 发起扫描
- `GET /api/scans`
  - 扫描列表
- `GET /api/scans/:id`
  - 扫描详情
- `POST /api/scans/:id/tasks`
  - 根据确认后的扫描条目批量创建任务

### 5.4 任务接口

- `GET /api/tasks`
  - 任务列表，支持按状态筛选
- `GET /api/tasks/:id`
  - 任务详情
- `GET /api/tasks/:id/logs`
  - 任务日志
- `POST /api/tasks/:id/retry`
  - 重试失败任务
- `POST /api/tasks/:id/cancel`
  - 取消任务

### 5.5 系统接口

- `GET /api/health`
  - 服务状态

## 6. 前端页面设计

### 6.1 配置页

- 编辑 OpenList / aria2 / Emos / Worker 配置
- 支持保存与基础校验

### 6.2 OpenList 浏览页

- 目录树浏览
- 当前路径面包屑
- 录入 `tmdb_id`
- 触发扫描

### 6.3 扫描结果页

- 展示扫描会话摘要
- 展示每个文件的解析结果、自动匹配结果
- 支持人工修改 `item_type` / `item_id`
- 支持勾选后批量入队

### 6.4 任务队列页

- 按状态筛选
- 进度与错误展示
- 重试 / 取消

### 6.5 任务详情页

- 来源文件快照
- 目标条目快照
- 执行进度
- `file_id` / `media_id`
- 日志列表

## 7. 后端骨架原则

- 只实现基础启动、依赖注入、示例接口、内存占位服务和 JSON store 雏形
- 业务客户端只提供接口定义和 stub，避免在第一阶段写满外部调用细节
- scheduler 先具备启动与轮询骨架，不直接接完整下载上传链路

## 8. 前端骨架原则

- 使用 Vue3 + TypeScript + Vite + Element Plus + Pinia
- 页面先完成布局、导航、占位数据绑定和 API 接口边界
- 不提前实现复杂交互，只把主流程页面串起来
