# -*- coding: utf-8 -*-
import time
import mimetypes
import requests
from pathlib import Path
from typing import List, Dict, Any, Optional
from fastapi import BackgroundTasks
from pydantic import BaseModel
from requests.adapters import HTTPAdapter
from requests.exceptions import Timeout, ConnectionError, RequestException
from urllib3.util.retry import Retry

from .config import state, DEFAULT_EMOS_API_BASE, DEFAULT_CACHE_DIR, DEFAULT_OPENLIST_BASE, DEFAULT_OPENLIST_TOKEN, DEFAULT_ARIA2_RPC_URL, DEFAULT_ARIA2_RPC_SECRET, DEFAULT_CHUNK_SIZE_MB, DEFAULT_PARALLEL_TASKS, DEFAULT_DOWNLOAD_THREADS
from .utils import log, bytes_to_speed, RateMeter, _backoff
from pydantic import Field


class UploadItem(BaseModel):
    name: str
    ol_path: str
    size_bytes: int = 0
    season: Optional[int] = None
    episode: Optional[int] = None
    selected: bool = True
    manual_id: Optional[str] = None


class UploadRequest(BaseModel):
    emos_token: str
    emos_api_base: str = DEFAULT_EMOS_API_BASE
    tmdb_id: int
    storage: str = "global"
    force_upload: bool = False
    match_mode: str = "strict"
    openlist_base_url: str = DEFAULT_OPENLIST_BASE
    openlist_token: str = DEFAULT_OPENLIST_TOKEN
    cache_dir: str = DEFAULT_CACHE_DIR
    aria2_rpc_url: str = DEFAULT_ARIA2_RPC_URL
    aria2_rpc_secret: str = DEFAULT_ARIA2_RPC_SECRET
    chunk_size_mb: int = DEFAULT_CHUNK_SIZE_MB
    parallel_tasks: int = DEFAULT_PARALLEL_TASKS
    download_threads: int = DEFAULT_DOWNLOAD_THREADS
    files: List[UploadItem]


class Uploader:
    def __init__(self):
        self.session = requests.Session()
        retries = Retry(total=3, backoff_factor=1, status_forcelist=[500, 502, 503, 504])
        self.session.mount("http://", HTTPAdapter(max_retries=retries))
        self.session.mount("https://", HTTPAdapter(max_retries=retries))
        self.upl_meter = RateMeter(interval=1.0, alpha=0.35)

    def _headers(self):
        return {
            "Authorization": f"Bearer {state.emos_token}",
            "User-Agent": "EMOS-PRO-PANEL/5.1",
            "Content-Type": "application/json",
        }

    def get_token(self, path: str, type_: str, storage: str) -> Optional[Dict]:
        try:
            f = Path(path)
            mime, _ = mimetypes.guess_type(str(f))
            if not mime:
                mime = "application/octet-stream"

            payload = {
                "type": type_,
                "file_type": mime,
                "file_name": f.name,
                "file_size": f.stat().st_size,
                "file_storage": storage
            }
            r = self.session.post(
                f"{state.emos_api_base}/api/upload/getUploadToken",
                json=payload,
                headers=self._headers(),
                timeout=60
            )
            if r.status_code != 200:
                log(f"获取 UploadToken 失败 HTTP {r.status_code}: {r.text[:300]} | payload={payload}", "ERROR")
                return None
            return r.json()
        except Exception as e:
            log(f"获取 UploadToken 异常: {e}", "ERROR")
            return None

    def save_upload(self, item_type: str, item_id: int, file_id: str) -> bool:
        try:
            payload = {"item_type": item_type, "item_id": item_id, "file_id": file_id}
            r = self.session.post(
                f"{state.emos_api_base}/api/upload/video/save",
                json=payload,
                headers=self._headers(),
                timeout=60
            )
            if r.status_code != 200:
                log(f"保存上传结果失败 HTTP {r.status_code}: {r.text[:250]}", "ERROR")
                return False
            return True
        except Exception as e:
            log(f"保存上传结果异常: {e}", "ERROR")
            return False

    def _upload_cb(self, cur: int, total: int):
        bps = self.upl_meter.update(cur)
        percent = (cur / total * 100.0) if total > 0 else 0
        eta_seconds = (total - cur) / bps if bps > 0 else 0
        state.task["upload"].update({
            "percent": percent,
            "speed": bytes_to_speed(bps),
            "eta": f"{int(eta_seconds // 60)}m {int(eta_seconds % 60)}s" if eta_seconds > 0 else "N/A",
            "done": (cur >= total),
        })

    def upload_stream_chunked(self, file_path: str, upload_url: str, chunk_size_mb: int) -> bool:
        file_size = Path(file_path).stat().st_size
        self._upload_cb(0, file_size)

        adapter = HTTPAdapter(pool_connections=8, pool_maxsize=8, max_retries=0)
        self.session.mount("http://", adapter)
        self.session.mount("https://", adapter)

        CHUNK_SIZE = int(chunk_size_mb) * 1024 * 1024
        CHUNK_SIZE = (CHUNK_SIZE // (256 * 1024)) * (256 * 1024)
        if CHUNK_SIZE == 0:
            CHUNK_SIZE = 256 * 1024
        log(f"开始上传，分片大小: {CHUNK_SIZE / 1024 / 1024:.2f} MB", "INFO")

        MAX_RETRY = 10
        uploaded = 0
        buf = bytearray(CHUNK_SIZE)

        with open(file_path, "rb") as f:
            while uploaded < file_size:
                if state.task["cancel"]:
                    raise RuntimeError("cancelled")

                n = f.readinto(buf)
                if not n:
                    break

                start = uploaded
                end = start + n - 1
                mv = memoryview(buf)[:n]

                headers = {
                    "Content-Type": "application/octet-stream",
                    "Content-Range": f"bytes {start}-{end}/{file_size}",
                    "Content-Length": str(n),
                }

                for attempt in range(1, MAX_RETRY + 1):
                    if state.task["cancel"]:
                        raise RuntimeError("cancelled")

                    try:
                        with self.session.put(upload_url, data=mv, headers=headers, timeout=(20, 600)) as resp:
                            code = resp.status_code

                            if 200 <= code < 300:
                                uploaded += n
                                self._upload_cb(uploaded, file_size)
                                break

                            if code in (429, 500, 502, 503, 504):
                                ra = resp.headers.get("Retry-After")
                                sleep_s = int(ra) if ra and ra.isdigit() else _backoff(attempt, cap=60.0)
                                log(f"分片限流/波动({code})，第{attempt}/{MAX_RETRY}次重试，等待 {sleep_s:.1f}s", "WARN")
                                time.sleep(sleep_s)
                                continue

                            log(f"分片上传失败 HTTP {code}: {resp.text[:200]}", "ERROR")
                            return False

                    except (Timeout, ConnectionError) as e:
                        sleep_s = _backoff(attempt, cap=60.0)
                        log(f"分片上传网络异常，第{attempt}/{MAX_RETRY}次重试：{e}，等待 {sleep_s:.1f}s", "WARN")
                        time.sleep(sleep_s)
                        continue
                    except RequestException as e:
                        sleep_s = _backoff(attempt, cap=60.0)
                        log(f"分片上传请求异常，第{attempt}/{MAX_RETRY}次重试：{e}，等待 {sleep_s:.1f}s", "WARN")
                        time.sleep(sleep_s)
                        continue

                else:
                    log("分片重试次数耗尽，上传失败", "ERROR")
                    return False
        
        self._upload_cb(file_size, file_size)
        return True

uploader = Uploader()

def register_upload_routes(app):
    @app.post("/api/start_upload")
    async def start_upload(req: UploadRequest, background_tasks: BackgroundTasks):
        from . import tasks
        if state.task["is_running"]:
            return {"error": "已有任务正在运行"}
        background_tasks.add_task(tasks.worker.process, req)
        return {"status": "started"}