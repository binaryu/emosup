# -*- coding: utf-8 -*-
import subprocess
from pathlib import Path
from typing import Dict, Any, List, Tuple
from urllib.parse import quote
import time
import asyncio
import re
import requests

from .config import state, ARIA2_BIN, DEFAULT_ARIA2_RPC_URL, DEFAULT_ARIA2_RPC_SECRET, DEFAULT_CHUNK_SIZE_MB, DEFAULT_PARALLEL_TASKS, DEFAULT_DOWNLOAD_THREADS
from .utils import log, ensure_dir, safe_unlink, bytes_to_speed, RateMeter, guess_season_episode
from .clients import EmosClient
from .aria2 import Aria2RpcClient
from .upload import uploader, UploadItem
from pydantic import BaseModel, Field
from typing import Optional


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
            if sn is None: continue
            sn = int(sn)
            for ep in sea.get("episodes") or []:
                en = ep.get("episode_number")
                ve_id = ep.get("item_id")
                if en is None or ve_id is None: continue
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

        if video_type == "tv" and match_mode == "single_season_autofill" and default_season is not None:
            used = {int(f.episode) for f in files if f.episode is not None}
            missing = sorted([f for f in files if f.episode is None], key=lambda x: x.name)
            ep = 1
            for f in missing:
                while ep in used: ep += 1
                if (default_season, ep) in ep_map:
                    autofill_map[f.ol_path] = (default_season, ep)
                    used.add(ep)
                    ep += 1

        for f in files:
            s, e = f.season, f.episode
            if s is None or e is None:
                ps, pe = guess_season_episode(f.name, f.ol_path)
                if s is None: s = ps
                if e is None: e = pe
            if video_type == "tv" and (s is None or e is None) and f.ol_path in autofill_map:
                s, e = autofill_map[f.ol_path]

            row: Dict[str, Any] = {
                "name": f.name, "ol_path": f.ol_path, "size_bytes": f.size_bytes,
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
            matched_keys.setdefault(key, []).append(f.ol_path)
            enriched.append(row)

        for key, paths in matched_keys.items():
            if len(paths) > 1:
                conflicts.append(f"冲突：S{key[0]}E{key[1]} 被 {len(paths)} 个文件匹配")
                for row in enriched:
                    if row.get("match_status") == "ok" and row.get("season") == key[0] and row.get("episode") == key[1]:
                        row["match_status"] = "conflict"
                        row["match_text"] = "冲突：" + row.get("match_text", "")
        return enriched, conflicts

    async def _process_file(self, f: UploadItem, req: UploadRequest, idx: Dict, enrich_map: Dict, aria2_client: Aria2RpcClient, sem: asyncio.Semaphore):
        async with sem:
            if state.task["cancel"]:
                return

            log(f"处理：{f.name}", "INFO")
            ef = enrich_map.get(f.ol_path) or {}
            mstatus = ef.get("match_status")

            if mstatus != "ok":
                log(f"跳过：匹配失败/冲突 status={mstatus} msg={ef.get('match_text')}", "ERROR")
                with state.lock:
                    state.task["completed_files"] += 1
                return

            item_type, item_id, s, e = ef.get("server_item_type"), ef.get("server_item_id"), ef.get("season"), ef.get("episode")
            if idx.get("video_type") == "tv" and not req.force_upload and bool(ef.get("server_has_media")):
                log(f"预检查：S{s}E{e} 已有资源 ve-{item_id}，跳过", "WARN")
                with state.lock:
                    state.task["completed_files"] += 1
                return

            direct_url = build_direct_url(req.openlist_base_url, f.ol_path)
            cache_path = str(Path(req.cache_dir).resolve() / Path(f.name).name)
            if not aria2_client.download_and_monitor(direct_url, cache_path, req.download_threads):
                log(f"下载失败：请检查 aria2 日志 -> {cache_path}", "ERROR")
                with state.lock:
                    state.task["completed_files"] += 1
                return

            upload_ok, save_ok = False, False
            try:
                token = uploader.get_token(cache_path, "video", req.storage)
                if not token or "data" not in token or "upload_url" not in token["data"]:
                    raise RuntimeError("getUploadToken failed")
                upload_url, file_id = token["data"]["upload_url"], token["file_id"]
                uploader.upl_meter = RateMeter(interval=1.0, alpha=0.35)
                upload_ok = uploader.upload_stream_chunked(cache_path, upload_url, req.chunk_size_mb)
                if not upload_ok: raise RuntimeError("upload failed")
                save_ok = uploader.save_upload(item_type, int(item_id), file_id)
                if save_ok:
                    log(f"✅ 保存成功：{f.name} -> {item_type}-{item_id} (S{s}E{e}) | {ef.get('server_episode_title')}" if idx.get('video_type') == "tv" else f"✅ 保存成功：{f.name} -> {item_type}-{item_id}", "SUCCESS")
                else:
                    log("上传成功但保存失败（缓存保留）", "ERROR")
            except Exception as ex:
                log(f"处理异常：{f.name} | {ex}", "ERROR")

            if upload_ok and save_ok:
                safe_unlink(Path(cache_path))
                safe_unlink(Path(cache_path + ".aria2"))
                log("已删除缓存文件(.aria2 也清理)", "INFO")
            else:
                log(f"未完全成功：保留缓存用于续传/重试 -> {cache_path}", "WARN")
            
            with state.lock:
                state.task["completed_files"] += 1

    def process(self, req: UploadRequest):
        asyncio.run(self.async_process(req))

    async def async_process(self, req: UploadRequest):
        selected = [f for f in req.files if f.selected]
        state.task.update({"is_running": True, "cancel": False, "total_files": len(selected), "completed_files": 0, "stage": "precheck"})
        
        try:
            state.emos_token, state.emos_api_base, state.openlist_base, state.openlist_token, state.cache_dir, state.aria2_rpc_url, state.aria2_rpc_secret, state.chunk_size_mb, state.parallel_tasks, state.download_threads = \
                req.emos_token, req.emos_api_base, req.openlist_base_url, req.openlist_token, req.cache_dir, req.aria2_rpc_url, req.aria2_rpc_secret, req.chunk_size_mb, req.parallel_tasks, req.download_threads
            ensure_dir(req.cache_dir)

            aria2_client = Aria2RpcClient(req.aria2_rpc_url, req.aria2_rpc_secret)
            if not aria2_client.check_version():
                log("Aria2 RPC 连接失败，请检查 URL 和密钥", "ERROR")
                return

            tree = self.client.get_tree_by_tmdb(req.tmdb_id)
            if not tree:
                log("无法获取 video/tree，请确认 tmdb_id 是否存在且已同步", "ERROR")
                return

            idx = self.build_tree_index(tree)
            log(f"Tree加载完成：video_type={idx.get('video_type')} vl_id={idx.get('vl_id')} title={idx.get('title')} episodes={len(idx.get('episodes') or {})} default_season={idx.get('default_season')}", "INFO")
            enriched, conflicts = self.precheck_files(idx, selected, req.match_mode)
            if conflicts:
                for c in conflicts: log(c, "WARN")
            enrich_map = {x["ol_path"]: x for x in enriched}
            log(f"=== 任务开始：files={len(selected)} tmdb={req.tmdb_id} match_mode={req.match_mode} parallel={req.parallel_tasks} ===", "INFO")
            
            sem = asyncio.Semaphore(req.parallel_tasks)
            tasks = [self._process_file(f, req, idx, enrich_map, aria2_client, sem) for f in selected]
            await asyncio.gather(*tasks)

            log("=== 所有任务结束 ===", "SUCCESS")
        finally:
            state.task.update({"stage": "idle", "is_running": False})

worker = BatchWorker()