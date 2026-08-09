import { defineStore } from 'pinia'

import type { AppConfig } from '@/types/api'
import { apiGet, apiSend } from '@/utils/api'

const defaultConfig: AppConfig = {
  server: {
    host: '0.0.0.0',
    port: 8080,
    web_title: 'Emos Upload Panel',
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
  download: {
    dir: './data/downloads',
  },
  emos: {
    base_url: 'https://emos.best',
    token: '',
    storage: 'default',
  },
  qbittorrent: {
    base_url: '',
    username: '',
    password: '',
    save_path: '',
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
    auto_tune: true,
  },
}

export const useConfigStore = defineStore('config', {
  state: () => ({
    config: structuredClone(defaultConfig) as AppConfig,
    loading: false,
    loaded: false,
  }),
  actions: {
    async fetchConfig(force = false) {
      if (this.loaded && !force) return
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
          download: { ...defaultConfig.download, ...(data.download || {}) },
          emos: { ...defaultConfig.emos, ...(data.emos || {}) },
          qbittorrent: { ...defaultConfig.qbittorrent, ...(data.qbittorrent || {}) },
          worker: {
            ...defaultConfig.worker,
            ...(data.worker || {}),
            // nil from the API means "not configured" → auto-tune is on by default.
            auto_tune: data.worker?.auto_tune ?? true,
          },
        }
        this.loaded = true
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
