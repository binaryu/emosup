import { defineStore } from 'pinia'

import type { QBittorrentFile, QBittorrentTorrent, ScanSession } from '@/types/api'
import { apiGet, apiSend } from '@/utils/api'

export const useQBittorrentStore = defineStore('qbittorrent', {
  state: () => ({
    torrents: [] as QBittorrentTorrent[],
    loading: false,
  }),
  actions: {
    async fetchTorrents() {
      this.loading = true
      try {
        this.torrents = await apiGet<QBittorrentTorrent[]>('/api/qbittorrent/torrents')
        return this.torrents
      } finally {
        this.loading = false
      }
    },
    async addTorrents(urls: string[]) {
      this.loading = true
      try {
        this.torrents = await apiSend<QBittorrentTorrent[]>('/api/qbittorrent/torrents', 'POST', {
          urls,
        })
        return this.torrents
      } finally {
        this.loading = false
      }
    },
    async fetchFiles(hash: string) {
      return await apiGet<QBittorrentFile[]>(`/api/qbittorrent/torrents/${hash}/files`)
    },
    async pause(hashes: string[]) {
      await apiSend('/api/qbittorrent/torrents/pause', 'POST', { hashes })
    },
    async resume(hashes: string[]) {
      await apiSend('/api/qbittorrent/torrents/resume', 'POST', { hashes })
    },
    async remove(hashes: string[], deleteFiles: boolean) {
      await apiSend('/api/qbittorrent/torrents', 'DELETE', { hashes, delete_files: deleteFiles })
    },
    async testConnection() {
      const data = await apiSend<{ ok: boolean }>('/api/qbittorrent/test', 'POST')
      return data.ok
    },
    async scanTorrent(hash: string, tmdbId: number, videoType: string) {
      return await apiSend<ScanSession>(`/api/qbittorrent/torrents/${hash}/scan`, 'POST', {
        tmdb_id: tmdbId,
        video_type: videoType,
      })
    },
  },
})
