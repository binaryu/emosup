# -*- coding: utf-8 -*-
import re
import time
import random
from pathlib import Path
from typing import Optional, Tuple

from .config import state


def log(msg: str, level: str = "INFO"):
    ts = time.strftime("%H:%M:%S")
    line = f"[{ts}] [{level}] {msg}"
    with state.lock:
        if len(state.logs) > 1500:
            state.logs = state.logs[-1100:]
        state.logs.append(line)
    print(line, flush=True)


def ensure_dir(p: str):
    Path(p).mkdir(parents=True, exist_ok=True)


def safe_unlink(p: Path):
    try:
        if p.exists():
            p.unlink()
    except Exception:
        pass


def bytes_to_speed(bps: float) -> str:
    return f"{bps / 1024 / 1024:.2f} MB/s"


class RateMeter:
    """固定刷新周期计算速度，避免回调太碎导致速度虚低"""
    def __init__(self, interval: float = 1.0, alpha: float = 0.35):
        self.interval = interval
        self.alpha = alpha
        self.last_t = 0.0
        self.last_b = 0
        self.speed_bps = 0.0

    def update(self, total_bytes: int) -> float:
        now = time.time()
        if self.last_t == 0.0:
            self.last_t = now
            self.last_b = total_bytes
            return self.speed_bps

        dt = now - self.last_t
        if dt < self.interval:
            return self.speed_bps

        delta = total_bytes - self.last_b
        inst = (delta / dt) if dt > 0 else 0.0
        self.speed_bps = inst if self.speed_bps == 0 else (self.alpha * inst + (1 - self.alpha) * self.speed_bps)
        self.last_t = now
        self.last_b = total_bytes
        return self.speed_bps


# ==============================
# Episode parser
# ==============================
def cn_season_to_int(s: str) -> Optional[int]:
    if s.isdigit():
        return int(s)
    
    s = s.strip()
    if not s:
        return None

    num_map = {'一': 1, '二': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9, '十': 10}
    
    if len(s) == 1:
        return num_map.get(s)
    
    if len(s) == 2:
        if s.startswith('十'): # 11-19
            return 10 + num_map.get(s[1], 0)
        if s.endswith('十'): # 20, 30, ...
            return num_map.get(s[0], 0) * 10
    
    if len(s) == 3 and s[1] == '十': # 21-99
        return num_map.get(s[0], 0) * 10 + num_map.get(s[2], 0)
        
    return None # Fallback for more complex numbers

_PATTERNS = [
    re.compile(r"[Ss](\d{1,2})[ ._-]*[Ee](\d{1,3})"),      # S01E02
    re.compile(r"(\d{1,2})x(\d{1,3})"),                   # 1x02
    re.compile(r"PL(\d{1,2})\..*?E(\d{1,3})", re.IGNORECASE), # PL01...E01
    re.compile(r"第[ _.-]*?(\d{1,3})[ _.-]*?集"),          # 第12集（没有季）
    re.compile(r"EP[ _.-]?(\d{1,3})", re.IGNORECASE),     # EP12
]

_SEASON_DIR = re.compile(r"(Season|S)[ _.-]?(\d{1,2})", re.IGNORECASE)
_SEASON_CN_DIR = re.compile(r"第([一二三四五六七八九十]+|\d+)季")


def guess_season_episode(name: str, full_path: str = "") -> Tuple[Optional[int], Optional[int]]:
    for pat in _PATTERNS:
        m = pat.search(name)
        if m:
            if "第" in pat.pattern:
                return None, int(m.group(1))
            if "EP" in pat.pattern.upper():
                return None, int(m.group(1))
            return int(m.group(1)), int(m.group(2))

    season = None
    if full_path:
        for p in Path(full_path).parts:
            m1 = _SEASON_DIR.search(p)
            if m1:
                season = int(m1.group(2))
            m2 = _SEASON_CN_DIR.search(p)
            if m2:
                season_str = m2.group(1)
                num = cn_season_to_int(season_str)
                if num is not None:
                    season = num

    m = re.search(r"第[ _.-]*?(\d{1,3})[ _.-]*?集", name)
    if m:
        return season, int(m.group(1))

    # Match standalone episode numbers like "01.mp4"
    m = re.match(r"^(\d{1,3})\.\w+$", name)
    if m:
        return season, int(m.group(1))

    return season, None


def _backoff(attempt: int, cap: float = 60.0) -> float:
    return min(cap, 2 ** attempt) + random.uniform(0, 0.8)