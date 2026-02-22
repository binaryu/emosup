# -*- coding: utf-8 -*-
import requests
from pathlib import Path
from typing import List, Dict, Any
from requests.adapters import HTTPAdapter
from urllib3.util.retry import Retry

from .config import state, VIDEO_EXTS
from .utils import guess_season_episode

class OpenListClient:
    """
    OpenList/AList v3: POST /api/fs/list
    """
    def __init__(self, base_url: str, token: str):
        self.base = base_url.rstrip("/")
        self.token = token.strip()
        self.sess = requests.Session()
        retries = Retry(total=3, backoff_factor=1, status_forcelist=[500, 502, 503, 504])
        self.sess.mount("http://", HTTPAdapter(max_retries=retries))
        self.sess.mount("https://", HTTPAdapter(max_retries=retries))

    def _headers(self, bearer: bool = False) -> Dict[str, str]:
        h = {"Content-Type": "application/json"}
        if self.token:
            h["Authorization"] = f"Bearer {self.token}" if bearer else self.token
        return h

    def list_dir(self, path: str) -> List[Dict[str, Any]]:
        url = self.base + "/api/fs/list"
        payload = {"path": path, "password": ""}
        r = self.sess.post(url, headers=self._headers(bearer=False), json=payload, timeout=30)

        if r.status_code in (401, 403) and self.token:
            r = self.sess.post(url, headers=self._headers(bearer=True), json=payload, timeout=30)

        if r.status_code != 200:
            raise RuntimeError(f"OpenList list HTTP {r.status_code}: {r.text[:250]}")

        data = r.json()
        if isinstance(data, dict) and "data" in data:
            d = data.get("data") or {}
            content = d.get("content") or d.get("files") or []
        else:
            content = []
        if not isinstance(content, list):
            content = []
        return content

    def walk_videos(self, root_path: str) -> List[Dict[str, Any]]:
        out = []
        stack = [root_path]
        while stack:
            cur = stack.pop()
            items = self.list_dir(cur)
            for it in items:
                name = it.get("name") or it.get("Name") or ""
                is_dir = bool(it.get("is_dir") or it.get("isDir") or False)
                size = int(it.get("size") or it.get("Size") or 0)
                full = (cur.rstrip("/") + "/" + name).replace("//", "/")
                if is_dir:
                    stack.append(full)
                    continue
                if Path(name).suffix.lower() in VIDEO_EXTS:
                    s, e = guess_season_episode(name, full)
                    out.append({
                        "name": name,
                        "ol_path": full,
                        "size_bytes": size,
                        "season": s,
                        "episode": e,
                        "selected": True,

                        # 预检查填充字段
                        "match_status": "unchecked",  # unchecked|ok|missing|not_in_tree|conflict
                        "match_text": "",
                        "server_item_type": "",
                        "server_item_id": None,
                        "server_has_media": None,
                        "server_episode_title": "",
                        "server_date_air": "",
                    })
        out.sort(key=lambda x: (x.get("season") or 0, x.get("episode") or 0, x["name"]))
        return out

def register_openlist_routes(app):
    @app.post("/api/scan_remote")
    async def scan_remote(req: "ScanRemoteRequest"):
        try:
            state.task["stage"] = "scan"
            c = OpenListClient(req.openlist_base_url, req.openlist_token)
            files = c.walk_videos(req.root_path)
            for x in files:
                sz = int(x.get("size_bytes") or 0)
                x["size"] = f"{sz / 1024 / 1024:.1f} MB" if sz else "unknown"
            state.task["stage"] = "idle"
            return {"files": files}
        except Exception as e:
            state.task["stage"] = "idle"
            return {"error": str(e)}