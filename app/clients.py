# -*- coding: utf-8 -*-
import time
import mimetypes
from pathlib import Path
from typing import Optional, Dict, Any, List, Tuple

import requests
from requests.adapters import HTTPAdapter
from urllib3.util.retry import Retry
from requests.exceptions import Timeout, ConnectionError, RequestException

from .config import state, VIDEO_EXTS
from .utils import log, guess_season_episode, _backoff


# ==============================
# EMOS client
# ==============================
class EmosClient:
    def __init__(self):
        self.session = requests.Session()
        retries = Retry(total=3, backoff_factor=1, status_forcelist=[500, 502, 503, 504])
        self.session.mount("http://", HTTPAdapter(max_retries=retries))
        self.session.mount("https://", HTTPAdapter(max_retries=retries))
        self._tree_cache: Dict[int, Tuple[float, Optional[Dict[str, Any]]]] = {}

    def _headers(self):
        return {
            "Authorization": f"Bearer {state.emos_token}",
            "User-Agent": "EMOS-PRO-PANEL/5.1",
            "Content-Type": "application/json",
        }

    def get_tree_by_tmdb(self, tmdb_id: int, cache_ttl: int = 180) -> Optional[Dict[str, Any]]:
        now = time.time()
        if tmdb_id in self._tree_cache:
            ts, tree = self._tree_cache[tmdb_id]
            if now - ts < cache_ttl and tree:
                return tree

        try:
            r = self.session.get(
                f"{state.emos_api_base}/api/video/tree",
                params={"tmdb_id": tmdb_id},
                headers=self._headers(),
                timeout=60
            )
            if r.status_code != 200:
                log(f"获取 video/tree 失败 HTTP {r.status_code}: {r.text[:250]}", "ERROR")
                self._tree_cache[tmdb_id] = (now, None)
                return None
            data = r.json()
            if isinstance(data, list) and data:
                tree = data[0]
                self._tree_cache[tmdb_id] = (now, tree)
                return tree
            self._tree_cache[tmdb_id] = (now, None)
            return None
        except Exception as e:
            log(f"获取 video/tree 异常: {e}", "ERROR")
            self._tree_cache[tmdb_id] = (now, None)
            return None
