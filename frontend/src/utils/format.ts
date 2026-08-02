export function formatTime(value?: string) {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

export function formatBytes(value?: number) {
  if (!value) return '0 B'

  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = value
  let unitIndex = 0
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024
    unitIndex += 1
  }

  return `${size.toFixed(unitIndex === 0 ? 0 : 2)} ${units[unitIndex]}`
}

export function formatSpeed(value?: number) {
  if (!value) return '0 B/s'
  return `${formatBytes(value)}/s`
}

/** Remaining seconds for (total - done) at the given speed; undefined when unknown. */
export function computeETA(done: number, total: number, speed: number): number | undefined {
  if (!speed || speed <= 0 || !total || total <= 0) return undefined
  const remaining = total - done
  if (remaining <= 0) return 0
  return remaining / speed
}

export function formatETA(seconds?: number) {
  if (seconds === undefined || seconds === null || !isFinite(seconds) || seconds < 0) return '--'
  const s = Math.round(seconds)
  if (s < 60) return `${s} 秒`
  const m = Math.floor(s / 60)
  if (m < 60) return m % 60 === 0 ? `${m} 分钟` : `${m} 分 ${s % 60} 秒`
  const h = Math.floor(m / 60)
  if (h < 24) return m % 60 === 0 ? `${h} 小时` : `${h} 小时 ${m % 60} 分`
  return `${Math.floor(h / 24)} 天 ${h % 24} 小时`
}

/** "剩余 12 分钟" label or '--' when ETA is unknown. */
export function formatRemaining(done: number, total: number, speed: number) {
  const eta = computeETA(done, total, speed)
  if (eta === undefined) return '--'
  return `剩余 ${formatETA(eta)}`
}

export function formatSizeInMB(value?: number) {
  if (!value) return '-'
  return `${(value / 1024 / 1024).toFixed(1)} MB`
}

export function maskUrl(value?: string) {
  if (!value) return '-'

  try {
    const url = new URL(value)
    return `${url.origin}${url.pathname.slice(0, 24)}...`
  } catch {
    return `${value.slice(0, 40)}...`
  }
}
