import type { ApiResponse } from '@/types/api'

const TOKEN_KEY = 'emosup_token'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string | null) {
  if (token) {
    localStorage.setItem(TOKEN_KEY, token)
  } else {
    localStorage.removeItem(TOKEN_KEY)
  }
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
}

function redirectToLogin() {
  const path = window.location.pathname + window.location.search
  if (path.startsWith('/login')) return
  const redirect = encodeURIComponent(path || '/tasks')
  window.location.href = `/login?redirect=${redirect}`
}

/** fetch wrapper that attaches JWT and handles 401. */
export async function apiFetch(input: string, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers || {})
  const token = getToken()
  if (token && !headers.has('Authorization')) {
    headers.set('Authorization', `Bearer ${token}`)
  }
  if (init.body && !headers.has('Content-Type') && !(init.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json')
  }

  const response = await fetch(input, { ...init, headers })

  if (response.status === 401) {
    const isLoginRequest = input.includes('/api/auth/login')
    if (!isLoginRequest) {
      clearToken()
      redirectToLogin()
    }
  }

  return response
}

export async function parseApiResponse<T>(response: Response): Promise<T> {
  const payload: ApiResponse<T> = await response.json()
  if (!payload.success) {
    throw new Error(payload.message || '请求失败')
  }
  return payload.data
}

export async function apiGet<T>(url: string): Promise<T> {
  return parseApiResponse<T>(await apiFetch(url))
}

export async function apiSend<T>(url: string, method: string, body?: unknown): Promise<T> {
  return parseApiResponse<T>(
    await apiFetch(url, {
      method,
      body: body === undefined ? undefined : JSON.stringify(body),
    }),
  )
}
