# 第二阶段设计说明

## 范围

第二阶段只覆盖以下能力：

- OpenList 目录浏览
- 指定目录视频文件扫描
- 获取视频文件下载直链
- 基于 `tmdb_id` 拉取 Emos 视频树
- 文件名与路径季集解析
- 自动匹配 `item_type` / `item_id`
- ScanSession / ScanItem 持久化
- 人工修正与确认保存

明确不进入以下链路：

- 不创建任务
- 不调用 aria2
- 不执行任务调度
- 不进入上传与保存媒体流程

## 数据模型调整

### ScanSession

- `path`
- `tmdb_id`
- `video_type`
- `status`: `processing` / `completed` / `failed`
- `total_count`
- `matched_count`
- `unmatched_count`
- `items[]`

### ScanItem

- `scan_session_id`
- `openlist_path`
- `file_name`
- `file_size`
- `raw_url`
- `is_video`
- `parsed`
  - `season`
  - `episode`
  - `is_special`
  - `raw_text`
- `match_status`
  - `matched`
  - `unmatched`
  - `ambiguous`
  - `invalid`
- `match_reason`
- `match_candidates[]`
- `selected_item_type`
- `selected_item_id`
- `selected_title`
- `confirmed`

## 服务划分

### OpenListService

- 读取配置
- 调用 OpenList client 列目录
- 获取文件 raw link
- 判断是否为视频文件

### EmosService

- 读取配置
- 获取视频树
- 预留获取视频基础信息接口

### Parser

- 独立解析 `S01E01`、`1x01`、`第01集`、`EP01`
- 从路径补充季信息
- 识别 `Specials` / `特别篇` 为 `season=0`

### MatchService

- 电影直接匹配根节点
- 电视剧按 `season` + `episode` 匹配
- 生成 `match_status`、`match_reason`、候选目标与最终默认选择结果

### ScanService

- 协调整次扫描流程
- 聚合 OpenList / Parser / Emos / Match
- 保存 ScanSession
- 支持单项人工修正保存

## API 约定

### `GET /api/openlist/list?path=...`

- 返回目录条目

### `POST /api/scans`

- 请求体：
  - `path`
  - `tmdb_id`
  - `video_type` 可选
- 返回完整 `ScanSession`

### `GET /api/scans`

- 返回扫描会话列表

### `GET /api/scans/:id`

- 返回扫描会话详情

### `PATCH /api/scans/:scanId/items/:itemId`

- 更新：
  - `selected_item_type`
  - `selected_item_id`
  - `selected_title`
  - `confirmed`

## OpenList 适配策略

基于 OpenList 文档，MVP 使用：

- `POST /api/fs/list`
- `POST /api/fs/get`

并在 client 层封装响应适配，避免业务层依赖第三方字段细节。

## 前端调整

- OpenList 页增加 `video_type` 输入
- 扫描结果页移除“添加任务队列”
- 改为展示匹配状态、解析结果、候选目标、确认状态
- 提供逐项保存人工修正
