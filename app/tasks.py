# -*- coding: utf-8 -*-
from pathlib import Path
from typing import Dict, Any, List, Tuple
from urllib.parse import quote
import asyncio
import re
import time
import uuid

from pydantic import BaseModel

from .config import state, DEFAULT_ARIA2_RPC_URL, DEFAULT_ARIA2_RPC_SECRET, DEFAULT_CHUNK_SIZE_MB, DEFAULT_PARALLEL_TASKS, DEFAULT_DOWNLOAD_THREADS, QUEUE_RECENT_LIMIT, QUEUE_STATUS_TASK_LIMIT
from .utils import log, ensure_dir, safe_unlink, RateMeter, guess_season_episode
from .clients import EmosClient
from .aria2 import Aria2RpcClient
from .upload import uploader, UploadItem


class UploadRequest(BaseModel):
    emos_token: str
    emos_api_base: str
    tmdb_id: int
    storage: str = "global"
    force_upload: bool = False
    match_mode: str = "strict"
    openlist_base_url: str
    openlist_token: str
    cache_dir: str
    aria2_rpc_url: str = DEFAULT_ARIA2_RPC_URL
    aria2_rpc_secret: str = DEFAULT_ARIA2_RPC_SECRET
    chunk_size_mb: int = DEFAULT_CHUNK_SIZE_MB
    parallel_tasks: int = DEFAULT_PARALLEL_TASKS
    download_threads: int = DEFAULT_DOWNLOAD_THREADS
    files: List[UploadItem]


class QueueAddResult(BaseModel):
    added: int
    skipped: int
    queued_ids: List[str]


def build_direct_url(openlist_base: str, ol_path: str) -> str:
    base = openlist_base.rstrip("/")
    prefix = "/d/"
    p = ol_path.replace("\\", "/").lstrip("/")
    return base + prefix + quote(p, safe="/")


class BatchWorker:
    def __init__(self):
        self.client = EmosClient()

    @staticmethod
    def build_tree_index(tree: Dict[str, Any]) -> Dict[str, Any]:
        video_type = tree.get("video_type")
        vl_id = tree.get("item_id")
        title = tree.get("title")
        episodes: Dict[Tuple[int, int], Dict[str, Any]] = {}
        seasons = tree.get("seasons") or []
        normal_seasons = [int(s["season_number"]) for s in seasons if s.get("season_number") is not None and int(s["season_number"]) != 0 and (s.get("episodes") or [])]
        default_season = list(set(normal_seasons))[0] if len(set(normal_seasons)) == 1 else None

        for sea in seasons:
            sn = sea.get("season_number")
            if sn is None:
                continue
            sn = int(sn)
            for ep in sea.get("episodes") or []:
                en = ep.get("episode_number")
                ve_id = ep.get("item_id")
                if en is None or ve_id is None:
                    continue
                key = (sn, int(en))
                episodes[key] = {
                    "item_type": "ve", "item_id": int(ve_id),
                    "has_media": bool(ep.get("has_media")),
                    "episode_title": ep.get("episode_title") or "",
                    "date_air": ep.get("date_air") or "",
                }
        return {
            "video_type": video_type, "vl_id": int(vl_id) if isinstance(vl_id, (int, str)) and str(vl_id).isdigit() else vl_id,
            "title": title, "default_season": default_season, "episodes": episodes,
        }

    @staticmethod
    def precheck_files(tree_index: Dict[str, Any], files: List[UploadItem], match_mode: str) -> Tuple[List[Dict[str, Any]], List[str]]:
        video_type = tree_index.get("video_type")
        default_season = tree_index.get("default_season")
        ep_map: Dict[Tuple[int, int], Dict[str, Any]] = tree_index.get("episodes") or {}
        enriched: List[Dict[str, Any]] = []
        matched_keys: Dict[Tuple[int, int], List[str]] = {}
        conflicts: List[str] = []
        autofill_map: Dict[str, Tuple[int, int]] = {}

        def item_key(file_item: UploadItem) -> str:
            return file_item.ol_path or file_item.local_path or file_item.name

        if video_type == "tv" and match_mode == "single_season_autofill" and default_season is not None:
            used = {int(f.episode) for f in files if f.episode is not None}
            missing = sorted([f for f in files if f.episode is None], key=lambda x: x.name)
            ep = 1
            for f in missing:
                while ep in used:
                    ep += 1
                if (default_season, ep) in ep_map:
                    autofill_map[item_key(f)] = (default_season, ep)
                    used.add(ep)
                    ep += 1

        for f in files:
            s, e = f.season, f.episode
            if s is None or e is None:
                source_path = f.local_path or f.ol_path or f.name
                ps, pe = guess_season_episode(f.name, source_path)
                if s is None:
                    s = ps
                if e is None:
                    e = pe
            current_key = item_key(f)
            if video_type == "tv" and (s is None or e is None) and current_key in autofill_map:
                s, e = autofill_map[current_key]

            row: Dict[str, Any] = {
                "name": f.name, "ol_path": f.ol_path, "local_path": f.local_path, "size_bytes": f.size_bytes,
                "season": s, "episode": e, "manual_id": f.manual_id,
                "match_status": "missing", "match_text": "",
                "server_item_type": "", "server_item_id": None, "server_has_media": None,
                "server_episode_title": "", "server_date_air": "",
            }

            if f.manual_id:
                item_type = "ve" if "ve" in f.manual_id else "vl"
                item_id = int(re.sub(r'\D', '', f.manual_id))
                row.update({"match_status": "ok", "server_item_type": item_type, "server_item_id": item_id, "match_text": f"手动指定: {f.manual_id}"})
                enriched.append(row)
                continue

            if video_type == "movie":
                if tree_index.get("vl_id"):
                    row.update({"match_status": "ok", "server_item_type": "vl", "server_item_id": tree_index["vl_id"], "match_text": f"vl-{tree_index['vl_id']} | {tree_index.get('title') or ''}"})
                else:
                    row.update({"match_status": "missing", "match_text": "movie 无 vl_id"})
                enriched.append(row)
                continue

            if e is None:
                row.update({"match_status": "missing", "match_text": "缺 episode"})
                enriched.append(row)
                continue
            if s is None:
                if default_season is not None and match_mode != "strict":
                    s = default_season
                    row["season"] = s
                else:
                    row.update({"match_status": "missing", "match_text": "缺 season"})
                    enriched.append(row)
                    continue

            key = (int(s), int(e))
            info = ep_map.get(key)
            if not info:
                row.update({"match_status": "not_in_tree", "match_text": f"tree 无 S{s}E{e}"})
                enriched.append(row)
                continue

            row.update({
                "match_status": "ok", "server_item_type": "ve", "server_item_id": info["item_id"],
                "server_has_media": info["has_media"], "server_episode_title": info.get("episode_title") or "",
                "server_date_air": info.get("date_air") or "",
                "match_text": f"S{s}E{e} -> ve-{info['item_id']} | {row['server_episode_title']} | has_media={info['has_media']}"
            })
            matched_keys.setdefault(key, []).append(current_key)
            enriched.append(row)

        for key, paths in matched_keys.items():
            if len(paths) > 1:
                conflicts.append(f"冲突：S{key[0]}E{key[1]} 被 {len(paths)} 个文件匹配")
                for row in enriched:
                    if row.get("match_status") == "ok" and row.get("season") == key[0] and row.get("episode") == key[1]:
                        row["match_status"] = "conflict"
                        row["match_text"] = "冲突：" + row.get("match_text", "")
        return enriched, conflicts

    @staticmethod
    def _empty_progress() -> Dict[str, Any]:
        return {"percent": 0.0, "speed": "0 MB/s", "eta": "N/A", "done": False}

    def _queue_summary(self) -> Dict[str, Any]:
        queue_list = list(state.queue)
        current = state.current_task_id
        recent_ids = state.task_order[-QUEUE_RECENT_LIMIT:]
        recent = [state.tasks_by_id[tid] for tid in recent_ids if tid in state.tasks_by_id]
        counts = {
            "pending": 0,
            "running": 0,
            "success": 0,
            "failed": 0,
            "skipped": 0,
            "cancelled": 0,
        }
        for task in state.tasks_by_id.values():
            status = task.get("status")
            if status in counts:
                counts[status] += 1
        return {
            "is_running": state.worker_running,
            "cancel": state.cancel_current,
            "stage": state.task.get("stage", "idle"),
            "total_files": state.queue_stats["total"],
            "completed_files": state.queue_stats["completed"],
            "current_file": state.task.get("current_file", ""),
            "download": dict(state.task.get("download") or self._empty_progress()),
            "upload": dict(state.task.get("upload") or self._empty_progress()),
            "queue_size": len(queue_list),
            "pending_ids": queue_list[:30],
            "running_task_id": current,
            "recent_tasks": recent,
            "counts": counts,
        }

    def get_public_status(self) -> Dict[str, Any]:
        with state.lock:
            return self._queue_summary()

    def _reset_runtime_progress(self):
        state.task.update({
            "current_file": "",
            "stage": "idle",
            "download": self._empty_progress(),
            "upload": self._empty_progress(),
        })

    def _update_task_status(self, task_id: str, status: str, error: str = ""):
        with state.lock:
            task = state.tasks_by_id.get(task_id)
            if not task:
                return
            task["status"] = status
            task["error"] = error
            if status == "running":
                task["started_at"] = time.time()
            if status in {"success", "failed", "skipped", "cancelled"}:
                task["finished_at"] = time.time()

    def _finish_task(self, task_id: str, status: str, error: str = ""):
        with state.lock:
            task = state.tasks_by_id.get(task_id)
            if not task:
                return
            task["status"] = status
            task["error"] = error
            task["finished_at"] = time.time()
            state.queue_stats["completed"] += 1
            state.current_task_id = None
            state.task["current_file"] = ""
            state.task["stage"] = "idle"
            state.task["download"] = self._empty_progress()
            state.task["upload"] = self._empty_progress()
            log(
                f"[queue-debug] finish task_id={task_id} status={status} cancel_current={state.cancel_current} queue_size={len(state.queue)} completed={state.queue_stats['completed']}/{state.queue_stats['total']}",
                "INFO",
            )

    def _build_runtime_request(self, req: UploadRequest, file_item: UploadItem) -> UploadRequest:
        return UploadRequest(
            emos_token=req.emos_token,
            emos_api_base=req.emos_api_base,
            tmdb_id=req.tmdb_id,
            storage=req.storage,
            force_upload=req.force_upload,
            match_mode=req.match_mode,
            openlist_base_url=req.openlist_base_url,
            openlist_token=req.openlist_token,
            cache_dir=req.cache_dir,
            aria2_rpc_url=req.aria2_rpc_url,
            aria2_rpc_secret=req.aria2_rpc_secret,
            chunk_size_mb=req.chunk_size_mb,
            parallel_tasks=1,
            download_threads=req.download_threads,
            files=[file_item],
        )

    def enqueue(self, req: UploadRequest) -> QueueAddResult:
        selected = [f for f in req.files if f.selected]
        if not selected:
            return QueueAddResult(added=0, skipped=0, queued_ids=[])

        state.emos_token = req.emos_token
        state.emos_api_base = req.emos_api_base
        state.openlist_base = req.openlist_base_url
        state.openlist_token = req.openlist_token
        state.cache_dir = req.cache_dir
        state.aria2_rpc_url = req.aria2_rpc_url
        state.aria2_rpc_secret = req.aria2_rpc_secret
        state.chunk_size_mb = req.chunk_size_mb
        state.parallel_tasks = 1
        state.download_threads = req.download_threads
        ensure_dir(req.cache_dir)

        tree = self.client.get_tree_by_tmdb(req.tmdb_id)
        if not tree:
            raise RuntimeError("无法获取 video/tree，请确认 tmdb_id 是否存在且已同步")
        idx = self.build_tree_index(tree)
        enriched, conflicts = self.precheck_files(idx, selected, req.match_mode)
        if conflicts:
            for c in conflicts:
                log(c, "WARN")

        enrich_map = {(x.get("ol_path") or x.get("local_path") or x.get("name")): x for x in enriched}
        added_ids: List[str] = []
        skipped = 0
        with state.lock:
            for f in selected:
                key = f.ol_path or f.local_path or f.name
                enrich = dict(enrich_map.get(key) or {})
                task_id = uuid.uuid4().hex[:12]
                queue_task = {
                    "task_id": task_id,
                    "name": f.name,
                    "source": f.source or ("local" if f.local_path else "openlist"),
                    "ol_path": f.ol_path,
                    "local_path": f.local_path,
                    "size_bytes": f.size_bytes,
                    "season": enrich.get("season", f.season),
                    "episode": enrich.get("episode", f.episode),
                    "status": "pending",
                    "error": "",
                    "match_status": enrich.get("match_status", "missing"),
                    "match_text": enrich.get("match_text", ""),
                    "server_item_type": enrich.get("server_item_type", ""),
                    "server_item_id": enrich.get("server_item_id"),
                    "server_has_media": enrich.get("server_has_media"),
                    "server_episode_title": enrich.get("server_episode_title", ""),
                    "server_date_air": enrich.get("server_date_air", ""),
                    "created_at": time.time(),
                    "started_at": None,
                    "finished_at": None,
                    "req": req.model_dump(),
                    "file": f.model_dump(),
                }
                state.tasks_by_id[task_id] = queue_task
                state.task_order.append(task_id)
                if len(state.task_order) > QUEUE_STATUS_TASK_LIMIT * 3:
                    expired_ids = state.task_order[:-QUEUE_STATUS_TASK_LIMIT * 2]
                    state.task_order = state.task_order[-QUEUE_STATUS_TASK_LIMIT * 2:]
                    for expired_id in expired_ids:
                        if expired_id != state.current_task_id and expired_id not in state.queue:
                            state.tasks_by_id.pop(expired_id, None)
                state.queue.append(task_id)
                state.queue_stats["total"] += 1
                added_ids.append(task_id)

            state.task["is_running"] = state.worker_running
            if not state.worker_running and state.queue:
                state.task["stage"] = "queued"

        log(f"已加入队列：{len(added_ids)} 个文件 | 当前排队 {self.get_public_status().get('queue_size')}", "INFO")
        return QueueAddResult(added=len(added_ids), skipped=skipped, queued_ids=added_ids)

    async def _process_queue_task(self, queue_task: Dict[str, Any]):
        task_id = queue_task["task_id"]
        req = UploadRequest(**queue_task["req"])
        file_item = UploadItem(**queue_task["file"])
        runtime_req = self._build_runtime_request(req, file_item)
        idx = {
            "video_type": "movie" if queue_task.get("server_item_type") == "vl" else "tv",
        }
        enrich_map = {
            (file_item.ol_path or file_item.local_path or file_item.name): {
                "match_status": queue_task.get("match_status"),
                "match_text": queue_task.get("match_text"),
                "server_item_type": queue_task.get("server_item_type"),
                "server_item_id": queue_task.get("server_item_id"),
                "server_has_media": queue_task.get("server_has_media"),
                "server_episode_title": queue_task.get("server_episode_title"),
                "server_date_air": queue_task.get("server_date_air"),
                "season": queue_task.get("season"),
                "episode": queue_task.get("episode"),
            }
        }

        needs_aria2 = (file_item.source or ("local" if file_item.local_path else "openlist")).lower() != "local"
        aria2_client = Aria2RpcClient(req.aria2_rpc_url, req.aria2_rpc_secret) if needs_aria2 else None
        if needs_aria2 and not aria2_client.check_version():
            raise RuntimeError("Aria2 RPC 连接失败，请检查 URL 和密钥")

        self._update_task_status(task_id, "running")
        with state.lock:
            log(
                f"[queue-debug] start task_id={task_id} name={file_item.name} before_reset cancel_current={state.cancel_current} queue_size={len(state.queue)} current_task_id={state.current_task_id}",
                "INFO",
            )
            state.current_task_id = task_id
            state.cancel_current = False
            state.worker_running = True
            state.task.update({
                "is_running": True,
                "cancel": False,
                "current_file": file_item.name,
                "stage": "running",
                "download": self._empty_progress(),
                "upload": self._empty_progress(),
            })
            log(
                f"[queue-debug] start task_id={task_id} name={file_item.name} after_reset cancel_current={state.cancel_current} queue_size={len(state.queue)} current_task_id={state.current_task_id}",
                "INFO",
            )

        log(f"开始处理队列任务：{file_item.name}", "INFO")
        await self._process_file(file_item, runtime_req, idx, enrich_map, aria2_client)

    async def _process_file(self, f: UploadItem, req: UploadRequest, idx: Dict, enrich_map: Dict, aria2_client: Aria2RpcClient):
        if state.cancel_current:
            raise RuntimeError("cancelled")

        file_key = f.ol_path or f.local_path or f.name
        ef = enrich_map.get(file_key) or {}
        mstatus = ef.get("match_status")

        if mstatus != "ok":
            raise RuntimeError(f"匹配失败/冲突 status={mstatus} msg={ef.get('match_text')}")

        item_type, item_id, s, e = ef.get("server_item_type"), ef.get("server_item_id"), ef.get("season"), ef.get("episode")
        if idx.get("video_type") == "tv" and not req.force_upload and bool(ef.get("server_has_media")):
            raise RuntimeError(f"预检查：S{s}E{e} 已有资源 ve-{item_id}，跳过")

        source = (f.source or ("local" if f.local_path else "openlist")).lower()
        if source == "local":
            cache_path = str(Path(f.local_path or "").resolve())
            if not f.local_path or not Path(cache_path).exists():
                raise RuntimeError(f"本地文件不存在：{cache_path or f.local_path}")
            log(f"使用本地文件直传：{cache_path}", "INFO")
        else:
            if not f.ol_path:
                raise RuntimeError(f"OpenList 路径缺失：{f.name}")
            direct_url = build_direct_url(req.openlist_base_url, f.ol_path)
            cache_path = str(Path(req.cache_dir).resolve() / Path(f.name).name)
            with state.lock:
                state.task["stage"] = "download"
            log(f"通过 OpenList + aria2 下载缓存：{f.ol_path}", "INFO")
            if not aria2_client.download_and_monitor(direct_url, cache_path, req.download_threads):
                raise RuntimeError(f"下载失败：请检查 aria2 日志 -> {cache_path}")

        upload_ok, save_ok = False, False
        try:
            with state.lock:
                state.task["stage"] = "upload"
            token = uploader.get_token(cache_path, "video", req.storage)
            if not token or "data" not in token or "upload_url" not in token["data"]:
                raise RuntimeError("getUploadToken failed")
            upload_url, file_id = token["data"]["upload_url"], token["file_id"]
            uploader.upl_meter = RateMeter(interval=1.0, alpha=0.35)
            upload_ok = uploader.upload_stream_chunked(cache_path, upload_url, req.chunk_size_mb)
            if not upload_ok:
                raise RuntimeError("upload failed")
            save_ok = uploader.save_upload(item_type, int(item_id), file_id)
            if not save_ok:
                raise RuntimeError("save upload failed")
            log(f"✅ 保存成功：{f.name} -> {item_type}-{item_id} (S{s}E{e}) | {ef.get('server_episode_title')}" if idx.get('video_type') == 'tv' else f"✅ 保存成功：{f.name} -> {item_type}-{item_id}", "SUCCESS")
        finally:
            if upload_ok and save_ok:
                if source == "local":
                    log("本地文件上传完成（保留源文件）", "INFO")
                else:
                    safe_unlink(Path(cache_path))
                    safe_unlink(Path(cache_path + ".aria2"))
                    log("已删除缓存文件(.aria2 也清理)", "INFO")
            else:
                log(f"未完全成功：保留缓存用于续传/重试 -> {cache_path}", "WARN")

    async def worker_loop(self):
        while True:
            with state.lock:
                log(
                    f"[queue-debug] loop_enter queue_size={len(state.queue)} current_task_id={state.current_task_id} cancel_current={state.cancel_current} worker_running={state.worker_running}",
                    "INFO",
                )
                if not state.queue:
                    state.worker_running = False
                    state.task["is_running"] = False
                    self._reset_runtime_progress()
                    log("[queue-debug] loop_exit queue empty, worker stopped", "INFO")
                    return
                task_id = state.queue.pop(0)
                queue_task = state.tasks_by_id.get(task_id)
                log(
                    f"[queue-debug] dequeue task_id={task_id} queue_size={len(state.queue)} cancel_current={state.cancel_current}",
                    "INFO",
                )

            if not queue_task:
                continue

            try:
                await self._process_queue_task(queue_task)
                self._finish_task(task_id, "success")
            except Exception as ex:
                msg = str(ex)
                status = "cancelled" if msg == "cancelled" else ("skipped" if "跳过" in msg or "预检查" in msg else "failed")
                log(f"任务结束：{queue_task.get('name')} | {msg}", "WARN" if status in {"skipped", "cancelled"} else "ERROR")
                self._finish_task(task_id, status, msg)

    def ensure_worker(self):
        with state.lock:
            if state.worker_running:
                return False
            state.worker_running = True
            state.task["is_running"] = True
        asyncio.run(self.worker_loop())
        return True

    def start_worker(self):
        self.ensure_worker()

    def cancel_current(self) -> Dict[str, str]:
        with state.lock:
            if not state.worker_running or not state.current_task_id:
                log(
                    f"[queue-debug] cancel ignored worker_running={state.worker_running} current_task_id={state.current_task_id} queue_size={len(state.queue)} cancel_current={state.cancel_current}",
                    "INFO",
                )
                return {"status": "no_task"}
            state.cancel_current = True
            state.task["cancel"] = True
            log(
                f"[queue-debug] cancel requested current_task_id={state.current_task_id} queue_size={len(state.queue)} cancel_current={state.cancel_current}",
                "INFO",
            )
            return {"status": "cancelling"}


worker = BatchWorker()
