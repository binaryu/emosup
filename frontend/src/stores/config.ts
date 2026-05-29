import { defineStore } from 'pinia'

import type { AppConfig, ApiResponse } from '@/types/api'

const defaultConfig: AppConfig = {
  server: {
    host: '127.0.0.1',
    port: 8080,
  },
  openlist: {
    base_url: '',
    username: '',
    password: '',
    token: '',
  },
  aria2: {
    rpc_url: 'http://127.0.0.1:6800/jsonrpc',
    secret: '',
    download_dir: './data/downloads',
    poll_interval_seconds: 3,
    connect_timeout_seconds: 10,
  },
  emos: {
    base_url: 'https://emos.best',
    token: '',
    storage: 'default',
  },
  worker: {
    poll_interval_seconds: 5,
    max_concurrency: 1,
    download_threads: 4,
    upload_chunk_size_mb: 8,
    save_retry_interval_seconds: 25,
    save_retry_max_attempts: 8,
    tmdb_api_key: '',
    proxy_backends: 'quark,夸克',
  },
}

export const useConfigStore = defineStore('config', {
  state: () => ({
    config: defaultConfig as AppConfig,
    loading: false,
  }),
  actions: {
    async fetchConfig() {
      this.loading = true
      try {
        const response = await fetch('/api/config')
        const payload: ApiResponse<AppConfig> = await response.json()
        if (payload.success) {
          this.config = payload.data
        }
      } finally {
        this.loading = false
      }
    },
    async saveConfig() {
      this.loading = true
      try {
        const response = await fetch('/api/config', {
          method: 'PUT',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify(this.config),
        })
        const payload: ApiResponse<AppConfig> = await response.json()
        if (payload.success) {
          this.config = payload.data
        }
      } finally {
        this.loading = false
      }
    },
  },
})
