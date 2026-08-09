import { defineStore } from 'pinia'

import type { CacheEntry, CacheListResult } from '@/types/api'
import { apiGet, apiSend } from '@/utils/api'

export const useCacheStore = defineStore('cache', {
  state: () => ({
    result: null as CacheListResult | null,
    loading: false,
  }),
  actions: {
    async fetchCache() {
      this.loading = true
      try {
        this.result = await apiGet<CacheListResult>('/api/cache')
        return this.result
      } finally {
        this.loading = false
      }
    },
    async deletePaths(paths: string[]) {
      this.loading = true
      try {
        const data = await apiSend<{ deleted: string[]; failed: Record<string, string> }>(
          '/api/cache/delete',
          'POST',
          { paths },
        )
        return data
      } finally {
        this.loading = false
      }
    },
  },
})
