# -*- coding: utf-8 -*-
import requests
import time
from pathlib import Path
from typing import Dict, Optional

from .config import state
from .utils import log, ensure_dir, bytes_to_speed

class Aria2RpcClient:
    def __init__(self, rpc_url: str, secret: str):
        self.url = rpc_url
        self.secret = secret
        self.session = requests.Session()

    def _rpc_call(self, method: str, params: list) -> Dict:
        payload = {
            "jsonrpc": "2.0",
            "id": "emos",
            "method": method,
            "params": [f"token:{self.secret}"] + params,
        }
        try:
            r = self.session.post(self.url, json=payload, timeout=10)
            r.raise_for_status()
            return r.json()
        except requests.RequestException as e:
            log(f"Aria2 RPC call failed: {e}", "ERROR")
            return {"error": str(e)}

    def add_download(self, url: str, options: Dict) -> Optional[str]:
        res = self._rpc_call("aria2.addUri", [[url], options])
        return res.get("result")

    def get_status(self, gid: str) -> Optional[Dict]:
        res = self._rpc_call("aria2.tellStatus", [gid])
        return res.get("result")

    def remove_download(self, gid: str, force: bool = False):
        if force:
            self._rpc_call("aria2.forceRemove", [gid])
        else:
            self._rpc_call("aria2.removeDownloadResult", [gid])

    def check_version(self) -> bool:
        res = self._rpc_call("aria2.getVersion", [])
        if "result" in res:
            log(f"Aria2 connected: version {res['result']['version']}", "INFO")
            return True
        log(f"Aria2 connection failed: {res.get('error')}", "ERROR")
        return False

    def download_and_monitor(self, url: str, dst_path: str, threads: int) -> bool:
        ensure_dir(str(Path(dst_path).parent))
        options = {
            "dir": str(Path(dst_path).parent),
            "out": Path(dst_path).name,
            "allow-overwrite": "true",
            "auto-file-renaming": "false",
            "split": str(threads),
            "max-connection-per-server": str(threads),
        }
        gid = self.add_download(url, options)
        if not gid:
            log("Failed to add download task to aria2", "ERROR")
            return False

        log(f"Aria2 task added: GID={gid}", "INFO")

        while True:
            if state.task["cancel"]:
                log("Cancellation received, removing aria2 task", "WARN")
                self.remove_download(gid, force=True)
                return False

            status = self.get_status(gid)
            if not status:
                time.sleep(2)
                continue

            total = int(status.get("totalLength", 0))
            completed = int(status.get("completedLength", 0))
            speed = int(status.get("downloadSpeed", 0))
            percent = (completed / total * 100) if total > 0 else 0
            eta_seconds = (total - completed) / speed if speed > 0 else 0

            state.task["download"].update({
                "percent": percent,
                "speed": bytes_to_speed(speed),
                "eta": f"{int(eta_seconds // 60)}m {int(eta_seconds % 60)}s" if eta_seconds > 0 else "N/A",
            })

            if status["status"] == "complete":
                state.task["download"].update({"percent": 100.0, "speed": "done", "done": True})
                log("Aria2 download complete", "SUCCESS")
                self.remove_download(gid)
                return True
            elif status["status"] in ("error", "removed"):
                error_msg = status.get("errorMessage", "Unknown error")
                log(f"Aria2 download failed: {error_msg}", "ERROR")
                self.remove_download(gid, force=True)
                return False

            time.sleep(1)