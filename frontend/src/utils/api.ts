import type { ApiResponse } from '@/types/api'

export async function parseApiResponse<T>(response: Response): Promise<T> {
  const payload: ApiResponse<T> = await response.json()
  if (!payload.success) {
    throw new Error(payload.message || '请求失败')
  }
  return payload.data
}
