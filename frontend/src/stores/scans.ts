import { defineStore } from 'pinia'

import type { ApiResponse, ScanItem, ScanSession } from '@/types/api'

export const useScanStore = defineStore('scans', {
  state: () => ({
    scans: [] as ScanSession[],
    loading: false,
  }),
  actions: {
    async fetchScans() {
      this.loading = true
      try {
        const response = await fetch('/api/scans')
        const payload: ApiResponse<ScanSession[]> = await response.json()
        if (payload.success) {
          this.scans = payload.data
        }
      } finally {
        this.loading = false
      }
    },
    async createScan(path: string, tmdbId: number, videoType = '') {
      this.loading = true
      try {
        const response = await fetch('/api/scans', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            path,
            tmdb_id: tmdbId,
            video_type: videoType,
          }),
        })
        const payload: ApiResponse<ScanSession> = await response.json()
        if (payload.success) {
          this.scans = [payload.data, ...this.scans.filter((item) => item.id !== payload.data.id)]
          return payload.data
        }
        throw new Error(payload.message || '扫描失败')
      } finally {
        this.loading = false
      }
    },
    async updateScanItem(scanId: string, itemId: string, patch: Partial<Pick<ScanItem, 'selected_item_type' | 'selected_item_id' | 'selected_title' | 'confirmed'>>) {
      this.loading = true
      try {
        const response = await fetch(`/api/scans/${scanId}/items/${itemId}`, {
          method: 'PATCH',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify(patch),
        })
        const payload: ApiResponse<ScanItem> = await response.json()
        if (!payload.success) {
          throw new Error(payload.message || '保存扫描项失败')
        }

        const scan = this.scans.find((item) => item.id === scanId)
        if (scan) {
          const index = scan.items.findIndex((item) => item.id === itemId)
          if (index >= 0) {
            scan.items[index] = payload.data
            scan.updated_at = payload.data.updated_at
          }
        }

        return payload.data
      } finally {
        this.loading = false
      }
    },
  },
})
