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
