# -*- coding: utf-8 -*-
import os
import sys
import asyncio
import argparse
import uvicorn
from fastapi import FastAPI, BackgroundTasks, WebSocket, WebSocketDisconnect, Request
from fastapi.responses import HTMLResponse
from fastapi.templating import Jinja2Templates
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel
from typing import List, Optional

from .config import state, DEFAULT_EMOS_API_BASE, DEFAULT_CACHE_DIR, DEFAULT_OPENLIST_BASE, DEFAULT_OPENLIST_TOKEN, DEFAULT_ARIA2_RPC_URL, DEFAULT_ARIA2_RPC_SECRET, DEFAULT_CHUNK_SIZE_MB, DEFAULT_PARALLEL_TASKS, DEFAULT_DOWNLOAD_THREADS
from .utils import ensure_dir
from .tasks import worker
from .upload import register_upload_routes, UploadItem
from .openlist import register_openlist_routes


class ScanRemoteRequest(BaseModel):
    root_path: str
    openlist_base_url: str = DEFAULT_OPENLIST_BASE
    openlist_token: str = DEFAULT_OPENLIST_TOKEN


class PrecheckRequest(BaseModel):
    emos_token: str
    emos_api_base: str = DEFAULT_EMOS_API_BASE
    tmdb_id: int
    match_mode: str = "strict"
    files: List[UploadItem]


app = FastAPI(title="EMOS PRO Panel")
app.add_middleware(CORSMiddleware, allow_origins=["*"], allow_methods=["*"], allow_headers=["*"])

def resource_path(relative_path):
    """ Get absolute path to resource, works for dev and for PyInstaller """
    try:
        # PyInstaller creates a temp folder and stores path in _MEIPASS
        base_path = sys._MEIPASS
    except Exception:
        base_path = os.path.abspath(".")
    return os.path.join(base_path, relative_path)

templates = Jinja2Templates(directory=resource_path("app/templates"))
register_upload_routes(app)
register_openlist_routes(app)


@app.get("/api/config")
async def api_config():
    return {
        "emos_api_base": state.emos_api_base,
        "cache_dir": state.cache_dir,
        "openlist_base_url": state.openlist_base,
        "aria2_rpc_url": state.aria2_rpc_url,
        "chunk_size_mb": state.chunk_size_mb,
        "parallel_tasks": state.parallel_tasks,
        "download_threads": state.download_threads,
    }


@app.post("/api/precheck")
async def precheck(req: PrecheckRequest):
    try:
        state.task["stage"] = "precheck"
        state.emos_token = req.emos_token
        state.emos_api_base = req.emos_api_base
        tree = worker.client.get_tree_by_tmdb(req.tmdb_id)
        if not tree:
            state.task["stage"] = "idle"
            return {"error": "无法获取 video/tree，请确认 tmdb_id / token"}
        idx = worker.build_tree_index(tree)
        enriched, conflicts = worker.precheck_files(idx, req.files, req.match_mode)
        state.task["stage"] = "idle"
        return {
            "video_type": idx.get("video_type"),
            "vl_id": idx.get("vl_id"),
            "title": idx.get("title"),
            "default_season": idx.get("default_season"),
            "conflicts": conflicts,
            "files": enriched,
        }
    except Exception as e:
        state.task["stage"] = "idle"
        return {"error": str(e)}


@app.post("/api/cancel")
async def cancel_task():
    if not state.task["is_running"]:
        return {"status": "no_task"}
    state.task["cancel"] = True
    return {"status": "cancelling"}


@app.get("/api/status")
async def status():
    return {"logs": state.logs[-80:], "task": state.task}


@app.websocket("/ws")
async def ws(ws: WebSocket):
    await ws.accept()
    last_len = 0
    try:
        while True:
            new_logs = state.logs[last_len:]
            last_len = len(state.logs)
            # Reduce payload size when task is running
            log_payload = new_logs if not state.task.get("is_running") else new_logs[-10:]
            await ws.send_json({"logs": log_payload, "task": state.task})
            await asyncio.sleep(2.0) # Reduce update frequency
    except WebSocketDisconnect:
        return

@app.get("/")
async def index(request: Request):
    return templates.TemplateResponse("index.html", {"request": request})


def main():
    ensure_dir(state.cache_dir)
    parser = argparse.ArgumentParser()
    parser.add_argument("--host", default=os.environ.get("HOST", "0.0.0.0"))
    parser.add_argument("--port", type=int, default=int(os.environ.get("PORT", "12345")))
    args = parser.parse_args()
    print(f"EMOS PRO Panel running on http://{args.host}:{args.port}")
    uvicorn.run(app, host=args.host, port=args.port)

if __name__ == "__main__":
    main()