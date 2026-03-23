import { defineStore } from 'pinia'

import type { ScanItem, ScanSession } from '@/types/api'
import { parseApiResponse } from '@/utils/api'

export const useScanStore = defineStore('scans', {
  state: () => ({
    scans: [] as ScanSession[],
    loading: false,
  }),
  actions: {
    async fetchScans() {
      this.loading = true
      try {
        this.scans = await parseApiResponse<ScanSession[]>(await fetch('/api/scans'))
      } finally {
        this.loading = false
      }
    },
    async createScan(path: string, tmdbId: number, videoType = '') {
      this.loading = true
      try {
        const data = await parseApiResponse<ScanSession>(
          await fetch('/api/scans', {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
            },
            body: JSON.stringify({
              path,
              tmdb_id: tmdbId,
              video_type: videoType,
            }),
          }),
        )
        this.scans = [data, ...this.scans.filter((item) => item.id !== data.id)]
        return data
      } finally {
        this.loading = false
      }
    },
    async updateScanItem(scanId: string, itemId: string, patch: Partial<Pick<ScanItem, 'selected_item_type' | 'selected_item_id' | 'selected_title' | 'confirmed'>>) {
      this.loading = true
      try {
        const data = await parseApiResponse<ScanItem>(
          await fetch(`/api/scans/${scanId}/items/${itemId}`, {
            method: 'PATCH',
            headers: {
              'Content-Type': 'application/json',
            },
            body: JSON.stringify(patch),
          }),
        )

        const scan = this.scans.find((item) => item.id === scanId)
        if (scan) {
          const index = scan.items.findIndex((item) => item.id === itemId)
          if (index >= 0) {
            scan.items[index] = data
            scan.updated_at = data.updated_at
          }
        }

        return data
      } finally {
        this.loading = false
      }
    },
  },
})
