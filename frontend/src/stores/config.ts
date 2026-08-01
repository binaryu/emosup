import { defineStore } from 'pinia'

import type { AppConfig } from '@/types/api'
import { apiGet, apiSend } from '@/utils/api'

const defaultConfig: AppConfig = {
  server: {
    host: '0.0.0.0',
    port: 8080,
  },
  auth: {
    username: 'admin',
    password: '',
    jwt_secret: '',
    token_ttl_hours: 72,
  },
  local: {
    root: '',
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
    download_threads: 1,
    upload_chunk_size_mb: 8,
    upload_part_concurrency: 3,
    save_retry_interval_seconds: 25,
    save_retry_max_attempts: 8,
    tmdb_api_key: '',
    proxy_backends: 'quark,夸克',
  },
}

export const useConfigStore = defineStore('config', {
  state: () => ({
    config: structuredClone(defaultConfig) as AppConfig,
    loading: false,
  }),
  actions: {
    async fetchConfig() {
      this.loading = true
      try {
        const data = await apiGet<AppConfig>('/api/config')
        this.config = {
          ...structuredClone(defaultConfig),
          ...data,
          auth: {
            ...defaultConfig.auth,
            ...(data.auth || {}),
            // API never returns secrets; keep empty so save means "unchanged".
            password: '',
            jwt_secret: '',
          },
          server: { ...defaultConfig.server, ...(data.server || {}) },
          local: { ...defaultConfig.local, ...(data.local || {}) },
          openlist: { ...defaultConfig.openlist, ...(data.openlist || {}) },
          aria2: { ...defaultConfig.aria2, ...(data.aria2 || {}) },
          emos: { ...defaultConfig.emos, ...(data.emos || {}) },
          worker: { ...defaultConfig.worker, ...(data.worker || {}) },
        }
      } finally {
        this.loading = false
      }
    },
    async saveConfig() {
      this.loading = true
      try {
        const data = await apiSend<AppConfig>('/api/config', 'PUT', this.config)
        this.config = {
          ...this.config,
          ...data,
          auth: {
            ...this.config.auth,
            ...(data.auth || {}),
            password: '',
            jwt_secret: '',
          },
        }
      } finally {
        this.loading = false
      }
    },
  },
})
