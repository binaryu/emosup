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
import { parseApiResponse } from '@/utils/api'

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

        const data = await parseApiResponse<TaskListResponse>(await fetch(`/api/tasks?${params.toString()}`))
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
        const data = await parseApiResponse<Task>(await fetch(`/api/tasks/${taskId}`))
        this.activeTask = data
        return data
      } finally {
        this.loading = false
      }
    },
    async fetchTaskLog(taskId: string) {
      this.loading = true
      try {
        const data = await parseApiResponse<TaskLog>(await fetch(`/api/tasks/${taskId}/logs`))
        this.activeTaskLog = data
        return data
      } finally {
        this.loading = false
      }
    },
    async fetchTaskStats() {
      const data = await parseApiResponse<TaskStats>(await fetch('/api/tasks/stats'))
      this.stats = data
      return data
    },
    async fetchRuntimeStatus() {
      const data = await parseApiResponse<RuntimeStatus>(await fetch('/api/system/runtime'))
      this.runtime = data
      return data
    },
    async batchCreateTasks(scanSessionId: string, itemIds: string[]) {
      this.loading = true
      try {
        return await parseApiResponse<BatchCreateTasksResponse>(
          await fetch('/api/tasks/batch-create', {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
            },
            body: JSON.stringify({
              scan_session_id: scanSessionId,
              item_ids: itemIds,
            }),
          }),
        )
      } finally {
        this.loading = false
      }
    },
    async cancelTask(taskId: string) {
      this.loading = true
      try {
        const data = await parseApiResponse<Task>(
          await fetch(`/api/tasks/${taskId}/cancel`, {
            method: 'POST',
          }),
        )
        this.syncTask(data)
        return data
      } finally {
        this.loading = false
      }
    },
    async retryTask(taskId: string) {
      this.loading = true
      try {
        const data = await parseApiResponse<Task>(
          await fetch(`/api/tasks/${taskId}/retry`, {
            method: 'POST',
          }),
        )
        this.syncTask(data)
        return data
      } finally {
        this.loading = false
      }
    },
    async deleteTask(taskId: string) {
      this.loading = true
      try {
        await parseApiResponse(await fetch(`/api/tasks/${taskId}`, { method: 'DELETE' }))
        this.tasks = this.tasks.filter((item) => item.id !== taskId)
        this.total = Math.max(0, this.total - 1)
      } finally {
        this.loading = false
      }
    },
    async pauseTask(taskId: string) {
      const task = await parseApiResponse<Task>(await fetch(`/api/tasks/${taskId}/pause`, { method: 'POST' }))
      this.syncTask(task)
      return task
    },
    async resumeTask(taskId: string) {
      const task = await parseApiResponse<Task>(await fetch(`/api/tasks/${taskId}/resume`, { method: 'POST' }))
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
  },
})
