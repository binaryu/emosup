const VIDEO_EXTS = new Set([
  'mp4', 'mkv', 'avi', 'mov', 'wmv', 'flv', 'm4v', 'ts', 'mpg', 'mpeg', 'webm',
])

export function isVideoFile(name: string): boolean {
  const ext = name.split('.').pop()?.toLowerCase() || ''
  return VIDEO_EXTS.has(ext)
}

/**
 * Guess a show/movie title from a directory path for TMDB search.
 * Strips season folders, release tags, years, and codec noise.
 */
export function extractShowTitle(path: string): string {
  const parts = path.replace(/\/+$/, '').split('/').filter(Boolean)
  for (let i = parts.length - 1; i >= 0; i--) {
    const cleaned = cleanSegment(parts[i])
    if (!cleaned) continue
    if (isJunkSegment(cleaned) || isSeasonOnly(parts[i])) continue
    return cleaned
  }
  return ''
}

function cleanSegment(segment: string): string {
  let name = segment
  name = name.replace(/\[.*?\]/g, ' ')
  name = name.replace(/【.*?】/g, ' ')
  name = name.replace(/\(.*?\)/g, ' ')
  name = name.replace(/（.*?）/g, ' ')
  name = name.replace(/\bS\d{1,2}E\d{1,3}\b/gi, ' ')
  name = name.replace(/\b\d{1,2}x\d{1,3}\b/gi, ' ')
  name = name.replace(/第\s*\d+\s*[集话季]/g, ' ')
  name = name.replace(/第[一二三四五六七八九十]+\s*季/g, ' ')
  name = name.replace(/\bSeason\s*\d+\b/gi, ' ')
  name = name.replace(/\bS\d{1,2}\b/gi, ' ')
  name = name.replace(
    /\b(1080p|720p|2160p|4k|8k|bluray|web-?dl|webrip|hdtv|x264|x265|h\.?264|h\.?265|hevc|avc|flac|aac|dts|remux)\b/gi,
    ' ',
  )
  name = name.replace(/[_./\\]+/g, ' ')
  name = name.replace(/\s+/g, ' ').trim()
  return name
}

function isJunkSegment(name: string): boolean {
  return /^(S\d{1,2}|Season\s*\d+|Specials?|特别篇|Complete|全集|全\d+集|\d{4})$/i.test(name)
}

function isSeasonOnly(raw: string): boolean {
  return /(?:^|[\s._\-(（\[])(?:S\d{1,2}|Season\s*\d+|第\s*\d+\s*季|第[一二三四五六七八九十]+季)(?:$|[\s._\-)）\]])/i.test(
    raw,
  ) && cleanSegment(raw).length < 4
}
