import { defineStore } from 'pinia'

import type {
  BatchCreateTasksResponse,
  RuntimeStatus,
  Task,
  TaskListResponse,
  TaskLog,
  TaskStats,
  TaskStatus,
} from '@/types/api'
import { apiGet, apiSend } from '@/utils/api'

export const useTaskStore = defineStore('tasks', {
  state: () => ({
    tasks: [] as Task[],
    total: 0,
    page: 1,
    pageSize: 20,
    statusFilter: '' as TaskStatus | '',
    activeTask: null as Task | null,
    activeTaskLog: null as TaskLog | null,
    stats: null as TaskStats | null,
    runtime: null as RuntimeStatus | null,
    loading: false,
  }),
  actions: {
    async fetchTasks(options?: {
      status?: TaskStatus | ''
      page?: number
      pageSize?: number
    }) {
      this.loading = true
      try {
        const status = options?.status ?? this.statusFilter
        const page = options?.page ?? this.page
        const pageSize = options?.pageSize ?? this.pageSize
        const params = new URLSearchParams()
        if (status) params.set('status', status)
        params.set('page', String(page))
        params.set('page_size', String(pageSize))

        const data = await apiGet<TaskListResponse>(`/api/tasks?${params.toString()}`)
        // Merge instead of replace to avoid table re-render
        if (this.tasks.length === 0 || page !== this.page || status !== this.statusFilter) {
          this.tasks = data.items
        } else {
          this.mergeTasks(data.items)
        }
        this.total = data.total
        this.page = data.page
        this.pageSize = data.page_size
        this.statusFilter = status
      } finally {
        this.loading = false
      }
    },
    mergeTasks(items: Task[]) {
      const newMap = new Map(items.map(t => [t.id, t]))
      const existingIds = new Set(this.tasks.map(t => t.id))
      // Update existing rows in-place
      for (const t of this.tasks) {
        const updated = newMap.get(t.id)
        if (updated) Object.assign(t, updated)
      }
      // Add new rows at the beginning
      for (const t of items) {
        if (!existingIds.has(t.id)) this.tasks.unshift(t)
      }
      // Remove rows no longer in list
      this.tasks = this.tasks.filter(t => newMap.has(t.id))
    },
    async fetchTask(taskId: string) {
      this.loading = true
      try {
        const data = await apiGet<Task>(`/api/tasks/${taskId}`)
        this.activeTask = data
        return data
      } finally {
        this.loading = false
      }
    },
    async fetchTaskLog(taskId: string) {
      this.loading = true
      try {
        const data = await apiGet<TaskLog>(`/api/tasks/${taskId}/logs`)
        this.activeTaskLog = data
        return data
      } finally {
        this.loading = false
      }
    },
    async fetchTaskStats() {
      const data = await apiGet<TaskStats>('/api/tasks/stats')
      this.stats = data
      return data
    },
    async fetchRuntimeStatus() {
      const data = await apiGet<RuntimeStatus>('/api/system/runtime')
      this.runtime = data
      return data
    },
    async batchCreateTasks(scanSessionId: string, itemIds: string[], keepLocalFile = false) {
      this.loading = true
      try {
        return await apiSend<BatchCreateTasksResponse>('/api/tasks/batch-create', 'POST', {
          scan_session_id: scanSessionId,
          item_ids: itemIds,
          keep_local_file: keepLocalFile,
        })
      } finally {
        this.loading = false
      }
    },
    async cancelTask(taskId: string) {
      this.loading = true
      try {
        const data = await apiSend<Task>(`/api/tasks/${taskId}/cancel`, 'POST')
        this.syncTask(data)
        return data
      } finally {
        this.loading = false
      }
    },
    async retryTask(taskId: string) {
      this.loading = true
      try {
        const data = await apiSend<Task>(`/api/tasks/${taskId}/retry`, 'POST')
        this.syncTask(data)
        return data
      } finally {
        this.loading = false
      }
    },
    async deleteTask(taskId: string) {
      this.loading = true
      try {
        await apiSend(`/api/tasks/${taskId}`, 'DELETE')
        this.tasks = this.tasks.filter((item) => item.id !== taskId)
        this.total = Math.max(0, this.total - 1)
      } finally {
        this.loading = false
      }
    },
    async pauseTask(taskId: string) {
      const task = await apiSend<Task>(`/api/tasks/${taskId}/pause`, 'POST')
      this.syncTask(task)
      return task
    },
    async resumeTask(taskId: string) {
      const task = await apiSend<Task>(`/api/tasks/${taskId}/resume`, 'POST')
      this.syncTask(task)
      return task
    },
    syncTask(task: Task) {
      const index = this.tasks.findIndex((item) => item.id === task.id)
      if (index >= 0) {
        this.tasks[index] = task
      }
      if (this.activeTask?.id === task.id) {
        this.activeTask = task
      }
    },
    /**
     * Apply a live SSE progress/status patch without full list reload.
     * Replaces the row object so Element Plus table re-renders nested fields.
     */
    applyLiveUpdate(payload: {
      taskId: string
      status?: TaskStatus | string
      download?: Partial<Task['download']>
      upload?: Partial<Task['upload']>
    }): boolean {
      const index = this.tasks.findIndex((item) => item.id === payload.taskId)
      if (index < 0) return false

      const current = this.tasks[index]
      const next: Task = {
        ...current,
        download: { ...current.download },
        upload: { ...current.upload },
        result: { ...current.result },
        updated_at: new Date().toISOString(),
      }

      if (payload.status) {
        next.status = payload.status as TaskStatus
      }
      if (payload.download) {
        for (const [key, value] of Object.entries(payload.download)) {
          if (value !== undefined) {
            ;(next.download as Record<string, unknown>)[key] = value
          }
        }
      }
      if (payload.upload) {
        for (const [key, value] of Object.entries(payload.upload)) {
          if (value !== undefined) {
            ;(next.upload as Record<string, unknown>)[key] = value
          }
        }
      }

      this.tasks[index] = next
      if (this.activeTask?.id === payload.taskId) {
        this.activeTask = {
          ...this.activeTask,
          status: next.status,
          download: { ...next.download },
          upload: { ...next.upload },
          updated_at: next.updated_at,
        }
      }
      return true
    },
    hasActiveTasks(): boolean {
      return this.tasks.some((t) =>
        ['downloading', 'uploading', 'saving', 'queued', 'upload_pending'].includes(t.status),
      )
    },
  },
})
