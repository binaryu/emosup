import { defineStore } from 'pinia'

import type { ScanItem, ScanSession } from '@/types/api'
import { apiGet, apiSend } from '@/utils/api'

export const useScanStore = defineStore('scans', {
  state: () => ({
    scans: [] as ScanSession[],
    loading: false,
    deleting: false,
  }),
  actions: {
    async fetchScans() {
      this.loading = true
      try {
        this.scans = await apiGet<ScanSession[]>('/api/scans')
      } finally {
        this.loading = false
      }
    },
    async createScan(path: string, tmdbId: number, videoType = '', filePath = '', source = '', filePaths: string[] = []) {
      this.loading = true
      try {
        const data = await apiSend<ScanSession>('/api/scans', 'POST', {
          path,
          file_path: filePath || undefined,
          file_paths: filePaths.length > 0 ? filePaths : undefined,
          source: source || undefined,
          tmdb_id: tmdbId,
          video_type: videoType || undefined,
        })
        this.scans = [data, ...this.scans.filter((item) => item.id !== data.id)]
        return data
      } finally {
        this.loading = false
      }
    },
    async updateScanItem(scanId: string, itemId: string, patch: Partial<Pick<ScanItem, 'selected_item_type' | 'selected_item_id' | 'selected_title' | 'confirmed'>>) {
      this.loading = true
      try {
        const data = await apiSend<ScanItem>(`/api/scans/${scanId}/items/${itemId}`, 'PATCH', patch)

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
    async deleteScan(scanId: string) {
      this.deleting = true
      try {
        await apiSend(`/api/scans/${scanId}`, 'DELETE')
        this.scans = this.scans.filter((item) => item.id !== scanId)
      } finally {
        this.deleting = false
      }
    },
    async deleteScanItem(scanId: string, itemId: string) {
      return this.deleteScanItems(scanId, [itemId])
    },
    /** Batch-delete scan items in one request; returns the refreshed scan. */
    async deleteScanItems(scanId: string, itemIds: string[]) {
      this.deleting = true
      try {
        const data = await apiSend<ScanSession>(`/api/scans/${scanId}/items`, 'DELETE', {
          item_ids: itemIds,
        })
        this.applyScan(scanId, data)
        return data
      } finally {
        this.deleting = false
      }
    },
    /** Replace the cached scan with a server-fresh one; removes the card if empty. */
    applyScan(scanId: string, data: ScanSession) {
      const scan = this.scans.find((item) => item.id === scanId)
      if (!scan) return
      if (data.total_count === 0) {
        this.scans = this.scans.filter((item) => item.id !== scanId)
        return
      }
      scan.items = data.items
      scan.total_count = data.total_count
      scan.matched_count = data.matched_count
      scan.unmatched_count = data.unmatched_count
      scan.updated_at = data.updated_at
    },
  },
})
