# -*- coding: utf-8 -*-
from pathlib import Path
from typing import List, Dict, Any

from pydantic import BaseModel

from .config import VIDEO_EXTS, state
from .utils import guess_season_episode


class LocalFileScanner:
    @staticmethod
    def scan(root_path: str) -> List[Dict[str, Any]]:
        p = Path(root_path).expanduser()
        if not p.exists():
            raise RuntimeError(f"本地路径不存在: {root_path}")
        if not p.is_dir():
            raise RuntimeError(f"本地路径不是目录: {root_path}")

        out: List[Dict[str, Any]] = []
        for f in p.rglob('*'):
            if not f.is_file():
                continue
            if f.suffix.lower() not in VIDEO_EXTS:
                continue
            rel_name = f.name
            full = str(f.resolve())
            s, e = guess_season_episode(rel_name, full)
            size = f.stat().st_size
            out.append({
                "name": rel_name,
                "source": "local",
                "ol_path": "",
                "local_path": full,
                "size_bytes": size,
                "season": s,
                "episode": e,
                "selected": True,
                "match_status": "unchecked",
                "match_text": "",
                "server_item_type": "",
                "server_item_id": None,
                "server_has_media": None,
                "server_episode_title": "",
                "server_date_air": "",
            })

        out.sort(key=lambda x: (x.get("season") or 0, x.get("episode") or 0, x["name"]))
        return out


class ScanLocalRequest(BaseModel):
    local_path: str


def register_local_routes(app):
    @app.post("/api/scan_local")
    async def scan_local(req: ScanLocalRequest):
        try:
            state.task["stage"] = "scan"
            files = LocalFileScanner.scan(req.local_path)
            for x in files:
                sz = int(x.get("size_bytes") or 0)
                x["size"] = f"{sz / 1024 / 1024:.1f} MB" if sz else "unknown"
            state.task["stage"] = "idle"
            return {"files": files}
        except Exception as e:
            state.task["stage"] = "idle"
            return {"error": str(e)}
