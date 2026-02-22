# -*- coding: utf-8 -*-
import os
import threading
import collections
from typing import List, Dict, Any

# ==============================
# Config (env defaults)
# ==============================
DEFAULT_EMOS_API_BASE = os.environ.get("EMOS_API_BASE", "https://emos.best")
DEFAULT_EMOS_TOKEN = os.environ.get("EMOS_TOKEN", "")

DEFAULT_OPENLIST_BASE = os.environ.get("OPENLIST_BASE_URL", "http://127.0.0.1:5244")
DEFAULT_OPENLIST_TOKEN = os.environ.get("OPENLIST_TOKEN", "")

ARIA2_BIN = os.environ.get("ARIA2C_BIN", "aria2c")
DEFAULT_CACHE_DIR = os.environ.get("CACHE_DIR", "data/cache")
DEFAULT_ARIA2_RPC_URL = os.environ.get("ARIA2_RPC_URL", "http://127.0.0.1:6800/jsonrpc")
DEFAULT_ARIA2_RPC_SECRET = os.environ.get("ARIA2_RPC_SECRET", "")

DEFAULT_CHUNK_SIZE_MB = int(os.environ.get("DEFAULT_CHUNK_SIZE_MB", "128"))
DEFAULT_PARALLEL_TASKS = int(os.environ.get("DEFAULT_PARALLEL_TASKS", "3"))
DEFAULT_DOWNLOAD_THREADS = int(os.environ.get("DEFAULT_DOWNLOAD_THREADS", "4"))

VIDEO_EXTS = {".mp4", ".mkv", ".avi", ".mov", ".ts", ".m4v", ".webm"}


# ==============================
# State
# ==============================
class AppState:
    def __init__(self):
        self.emos_token = DEFAULT_EMOS_TOKEN
        self.emos_api_base = DEFAULT_EMOS_API_BASE

        self.openlist_base = DEFAULT_OPENLIST_BASE
        self.openlist_token = DEFAULT_OPENLIST_TOKEN
        self.direct_prefix = "/d/"

        self.cache_dir = DEFAULT_CACHE_DIR
        self.aria2_rpc_url = DEFAULT_ARIA2_RPC_URL
        self.aria2_rpc_secret = DEFAULT_ARIA2_RPC_SECRET
        self.chunk_size_mb = DEFAULT_CHUNK_SIZE_MB
        self.parallel_tasks = DEFAULT_PARALLEL_TASKS
        self.download_threads = DEFAULT_DOWNLOAD_THREADS

        self.logs: collections.deque = collections.deque(maxlen=500)
        self.lock = threading.Lock()

        self.task: Dict[str, Any] = {
            "is_running": False,
            "cancel": False,
            "total_files": 0,
            "completed_files": 0,
            "current_file": "",
            "stage": "idle",  # idle | scan | precheck | download | upload | finalize
            "download": {"percent": 0.0, "speed": "0 MB/s", "eta": "N/A", "done": False},
            "upload": {"percent": 0.0, "speed": "0 MB/s", "eta": "N/A", "done": False},
        }


state = AppState()