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
        this.tasks = data.items
        this.total = data.total
        this.page = data.page
        this.pageSize = data.page_size
        this.statusFilter = status
      } finally {
        this.loading = false
      }
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
