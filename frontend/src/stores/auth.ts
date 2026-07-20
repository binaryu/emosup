import { defineStore } from 'pinia'

import type { LoginResponse } from '@/types/api'
import { apiFetch, apiGet, clearToken, getToken, parseApiResponse, setToken } from '@/utils/api'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: getToken() as string | null,
    username: '' as string,
    loading: false,
    bootstrapped: false,
  }),
  getters: {
    isAuthenticated: (state) => Boolean(state.token),
  },
  actions: {
    async login(username: string, password: string) {
      this.loading = true
      try {
        const data = await parseApiResponse<LoginResponse>(
          await apiFetch('/api/auth/login', {
            method: 'POST',
            body: JSON.stringify({ username, password }),
          }),
        )
        setToken(data.token)
        this.token = data.token
        this.username = data.username
        this.bootstrapped = true
        return data
      } finally {
        this.loading = false
      }
    },
    async fetchMe() {
      if (!this.token) {
        this.bootstrapped = true
        return null
      }
      try {
        const data = await apiGet<{ username: string }>('/api/auth/me')
        this.username = data.username
        this.bootstrapped = true
        return data
      } catch {
        this.logout(false)
        this.bootstrapped = true
        return null
      }
    },
    logout(redirect = true) {
      clearToken()
      this.token = null
      this.username = ''
      if (redirect && !window.location.pathname.startsWith('/login')) {
        window.location.href = '/login'
      }
    },
  },
})
