<template>
  <div class="task-queue-view">
    <PageHeaderCard
      title="任务队列"
      subtitle="实时追踪所有离线下载与转存任务的执行状态、进度与错误信息。"
    >
      <div class="toolbar">
        <el-select v-model="filterStatus" placeholder="筛选状态" clearable class="filter-select" @change="reload">
          <el-option v-for="opt in filterOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
        </el-select>
        <el-button :loading="taskStore.loading" @click="reload" class="tool-btn">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/></svg>
          <span class="btn-label">刷新</span>
        </el-button>
        <el-button
          v-if="selectedTaskIds.length > 0"
          type="warning"
          plain
          class="tool-btn"
          @click="handleBatchPause"
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="6" y="4" width="4" height="16"/><rect x="14" y="4" width="4" height="16"/></svg>
          <span class="btn-label">暂停 ({{ selectedTaskIds.length }})</span>
        </el-button>
        <el-button
          v-if="selectedTaskIds.length > 0"
          type="success"
          plain
          class="tool-btn"
          @click="handleBatchResume"
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 3 19 12 5 21 5 3"/></svg>
          <span class="btn-label">恢复 ({{ selectedTaskIds.length }})</span>
        </el-button>
        <el-button
          v-if="selectedTaskIds.length > 0"
          type="danger"
          plain
          class="tool-btn"
          @click="handleBatchDelete"
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
          <span class="btn-label">删除 ({{ selectedTaskIds.length }})</span>
        </el-button>
        <el-button
          v-if="taskStore.total > 0"
          link
          type="primary"
          class="tool-btn"
          @click="selectAllTasks"
        >
          {{ allSelected ? '取消全选' : '全选所有' }}
        </el-button>
        <div class="toolbar-spacer"></div>
        <el-button
          link
          class="view-toggle"
          @click="viewMode = viewMode === 'table' ? 'card' : 'table'"
          :title="viewMode === 'table' ? '切换到卡片布局' : '切换到列表布局'"
        >
          <svg v-if="viewMode === 'table'" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/></svg>
          <svg v-else width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/><line x1="8" y1="18" x2="21" y2="18"/><line x1="3" y1="6" x2="3.01" y2="6"/><line x1="3" y1="12" x2="3.01" y2="12"/><line x1="3" y1="18" x2="3.01" y2="18"/></svg>
        </el-button>
      </div>
    </PageHeaderCard>

    <el-alert
      v-if="taskStore.runtime"
      class="runtime-alert"
      :title="taskStore.runtime.scheduler_running ? 'Scheduler 运行中' : 'Scheduler 未运行'"
      :type="taskStore.runtime.scheduler_running ? 'success' : 'warning'"
      :closable="false"
      show-icon
    >
      <template #default>
        <div>{{ runtimeDescription }}</div>
        <div v-if="diskInfo" style="margin-top: 4px; font-size: 12px; color: var(--text-subtle)">
          磁盘：已用 {{ formatBytes(diskInfo.used_bytes) }} / 共 {{ formatBytes(diskInfo.total_bytes) }} · 剩余 {{ formatBytes(diskInfo.free_bytes) }}
        </div>
      </template>
    </el-alert>

    <div v-if="taskStore.stats" class="stats-dashboard">
      <div class="stat-widget primary" :class="{ active: filterStatus === '' }" role="button" @click="setFilter('')">
        <div class="stat-content"><span class="stat-label">总任务</span><strong>{{ taskStore.stats.total }}</strong></div>
      </div>
      <div class="stat-widget warning" :class="{ active: filterStatus === 'queued' }" role="button" @click="setFilter('queued')">
        <div class="stat-content"><span class="stat-label">排队中</span><strong>{{ taskStore.stats.queued }}</strong></div>
      </div>
      <div class="stat-widget info" :class="{ active: filterStatus === 'downloading,uploading,saving' }" role="button" @click="setFilter('downloading,uploading,saving')">
        <div class="stat-content"><span class="stat-label">进行中</span><strong>{{ taskStore.stats.active }}</strong></div>
      </div>
      <div class="stat-widget muted" :class="{ active: filterStatus === 'canceled' }" role="button" @click="setFilter('canceled')">
        <div class="stat-content"><span class="stat-label">已取消</span><strong>{{ taskStore.stats.canceled }}</strong></div>
      </div>
      <div class="stat-widget danger" :class="{ active: filterStatus === 'download_failed,upload_failed' }" role="button" @click="setFilter('download_failed,upload_failed')">
        <div class="stat-content"><span class="stat-label">失败异常</span><strong>{{ taskStore.stats.failed }}</strong></div>
      </div>
      <div class="stat-widget success" :class="{ active: filterStatus === 'completed' }" role="button" @click="setFilter('completed')">
        <div class="stat-content"><span class="stat-label">已完成</span><strong>{{ taskStore.stats.completed }}</strong></div>
      </div>
    </div>

    <!-- Table View (Desktop) -->
    <el-card v-if="viewMode === 'table'" class="queue-card" :body-style="{ padding: '0' }">
      <el-table
        :data="taskStore.tasks"
        row-key="id"
        stripe
        class="task-table"
        @selection-change="onSelectionChange"
      >
        <el-table-column type="selection" width="42" />
        <el-table-column label="任务目标" min-width="220" show-overflow-tooltip>
          <template #default="{ row }">
            <div class="task-target">
              <span class="file-name">{{ row.source.file_name }}</span>
              <span class="task-meta">
                <span class="task-id">{{ row.id }}</span>
                <span v-if="row.parsed.season != null || row.parsed.episode != null" class="task-ep">S{{ row.parsed.season ?? '?' }}E{{ row.parsed.episode ?? '?' }}</span>
              </span>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="状态 / 进度" min-width="320">
          <template #default="{ row }">
            <div class="status-progress-cell">
              <div class="status-row">
                <StatusTag :status="row.status" />
                <StatusTag v-if="subStatus(row)" :status="subStatus(row)" size="small" />
                <el-tag v-if="row.keep_local_file" type="warning" size="small" effect="plain">保留</el-tag>
              </div>
              <template v-if="row.status === 'downloading'">
                <el-progress :percentage="Math.round(row.download.progress || 0)" :stroke-width="6" :show-text="false" />
                <div class="progress-detail">
                  <span>{{ formatBytes(row.download.completed_bytes) }} / {{ formatBytes(row.download.total_bytes) }}</span>
                  <span>{{ formatSpeed(row.download.speed) }} · {{ formatRemaining(row.download.completed_bytes, row.download.total_bytes, row.download.speed) }}</span>
                </div>
              </template>
              <template v-else-if="['uploading', 'saving'].includes(row.status)">
                <el-progress :percentage="Math.round(row.upload.progress || 0)" :stroke-width="6" :show-text="false" status="success" />
                <div class="progress-detail">
                  <span>{{ formatBytes(row.upload.uploaded_bytes) }} / {{ formatBytes(row.upload.total_bytes) }}</span>
                  <span>{{ formatSpeed(row.upload.speed) }} · {{ formatRemaining(row.upload.uploaded_bytes, row.upload.total_bytes, row.upload.speed) }}</span>
                </div>
              </template>
              <template v-else-if="row.status === 'completed'">
                <el-progress :percentage="100" :stroke-width="6" status="success" />
                <div class="progress-detail"><span style="color: var(--el-color-success)">已完成</span></div>
              </template>
              <template v-else>
                <div class="progress-detail"><span class="muted-text">-</span></div>
              </template>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="错误摘要" min-width="180" show-overflow-tooltip>
          <template #default="{ row }">
            <span v-if="row.result.error_message || row.upload.last_save_error" class="error-text">
              {{ row.upload.last_save_error || row.result.error_message }}
            </span>
            <span v-else class="muted-text">-</span>
          </template>
        </el-table-column>

        <el-table-column label="更新时间" width="150">
          <template #default="{ row }">
            <span class="muted-text">{{ formatTime(row.updated_at) }}</span>
          </template>
        </el-table-column>

        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <div class="action-group">
              <el-button link type="primary" size="small" class="action-btn" @click="goDetail(row.id)">详情</el-button>
              <el-popover placement="bottom-end" :width="150" trigger="click" :teleported="true">
                <template #reference>
                  <el-button link size="small" class="action-btn">管理</el-button>
                </template>
                <div class="popover-actions">
                  <div v-if="!row.paused && canCancel(row.status)" class="popover-item primary" @click="handlePause(row.id)">暂停</div>
                  <div v-if="row.paused" class="popover-item success" @click="handleResume(row.id)">恢复</div>
                  <div class="popover-item danger" :class="{ disabled: !canCancel(row.status) }" @click="canCancel(row.status) && handleCancel(row.id)">取消任务</div>
                  <div class="popover-item warning" :class="{ disabled: !canRetry(row.status) }" @click="canRetry(row.status) && handleRetry(row.id)">重新入队</div>
                  <div class="popover-divider"></div>
                  <div class="popover-item danger" :class="{ disabled: isActive(row.status) }" @click="!isActive(row.status) && handleDelete(row.id)">删除记录</div>
                </div>
              </el-popover>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-container">
        <el-pagination
          background
          layout="total, prev, pager, next"
          :current-page="taskStore.page"
          :page-size="taskStore.pageSize"
          :total="taskStore.total"
          @current-change="handlePageChange"
        />
      </div>
    </el-card>

    <!-- Card View (Mobile) -->
    <div v-else class="card-list">
      <el-empty v-if="!taskStore.tasks.length" description="暂无任务" />
      <div v-for="row in taskStore.tasks" :key="row.id" class="task-card" @click="goDetail(row.id)">
        <div class="card-header">
          <div class="card-status-row">
            <StatusTag :status="row.status" />
            <StatusTag v-if="subStatus(row)" :status="subStatus(row)" size="small" />
            <el-tag v-if="row.keep_local_file" type="warning" size="small" effect="plain">保留</el-tag>
            <span v-if="row.parsed.season != null || row.parsed.episode != null" class="card-ep">S{{ row.parsed.season ?? '?' }}E{{ row.parsed.episode ?? '?' }}</span>
          </div>
          <svg class="card-chevron" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="9 18 15 12 9 6"/></svg>
        </div>

        <div class="card-body">
          <div class="card-filename">{{ row.source.file_name }}</div>
          <div class="card-meta">
            <span class="card-task-id">{{ row.id }}</span>
            <span class="card-time">{{ formatTime(row.updated_at) }}</span>
          </div>

          <template v-if="row.status === 'downloading' && row.download.progress != null">
            <div class="card-progress">
              <div class="progress-header">
                <span>下载</span>
                <span>{{ Math.round(row.download.progress) }}%</span>
              </div>
              <el-progress :percentage="Math.round(row.download.progress)" :stroke-width="6" :show-text="false" />
              <div class="progress-stats">
                <span>{{ formatBytes(row.download.completed_bytes) }} / {{ formatBytes(row.download.total_bytes) }}</span>
                <span>{{ formatSpeed(row.download.speed) }} · {{ formatRemaining(row.download.completed_bytes, row.download.total_bytes, row.download.speed) }}</span>
              </div>
            </div>
          </template>
          <template v-else-if="['uploading', 'saving'].includes(row.status) && row.upload.progress != null">
            <div class="card-progress">
              <div class="progress-header">
                <span>上传</span>
                <span>{{ Math.round(row.upload.progress) }}%</span>
              </div>
              <el-progress :percentage="Math.round(row.upload.progress)" :stroke-width="6" :show-text="false" status="success" />
              <div class="progress-stats">
                <span>{{ formatBytes(row.upload.uploaded_bytes) }} / {{ formatBytes(row.upload.total_bytes) }}</span>
                <span>{{ formatSpeed(row.upload.speed) }} · {{ formatRemaining(row.upload.uploaded_bytes, row.upload.total_bytes, row.upload.speed) }}</span>
              </div>
            </div>
          </template>
          <template v-else-if="row.status === 'completed'">
            <el-progress :percentage="100" :stroke-width="6" status="success" />
          </template>

          <div v-if="row.result.error_message || row.upload.last_save_error" class="card-error">
            {{ row.upload.last_save_error || row.result.error_message }}
          </div>
        </div>

        <div class="card-actions" @click.stop>
          <button class="card-action-btn" @click="goDetail(row.id)">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
            详情
          </button>
          <button v-if="!row.paused && canCancel(row.status)" class="card-action-btn" @click="handlePause(row.id)">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="6" y="4" width="4" height="16"/><rect x="14" y="4" width="4" height="16"/></svg>
            暂停
          </button>
          <button v-if="row.paused" class="card-action-btn" @click="handleResume(row.id)">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 3 19 12 5 21 5 3"/></svg>
            恢复
          </button>
          <button class="card-action-btn" :class="{ disabled: !canCancel(row.status) }" :disabled="!canCancel(row.status)" @click="canCancel(row.status) && handleCancel(row.id)">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>
            取消
          </button>
          <button class="card-action-btn" :class="{ disabled: !canRetry(row.status) }" :disabled="!canRetry(row.status)" @click="canRetry(row.status) && handleRetry(row.id)">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="1 4 1 10 7 10"/><polyline points="23 20 23 14 17 14"/><path d="M20.49 9A9 9 0 0 0 5.64 5.64L1 10m22 4l-4.64 4.36A9 9 0 0 1 3.51 15"/></svg>
            重试
          </button>
          <button class="card-action-btn danger" :class="{ disabled: isActive(row.status) }" :disabled="isActive(row.status)" @click="!isActive(row.status) && handleDelete(row.id)">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
            删除
          </button>
        </div>
      </div>

      <div v-if="taskStore.tasks.length > 0" class="pagination-container">
        <el-pagination
          background
          small
          layout="prev, pager, next"
          :current-page="taskStore.page"
          :page-size="taskStore.pageSize"
          :total="taskStore.total"
          @current-change="handlePageChange"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRouter } from 'vue-router'

import PageHeaderCard from '@/components/PageHeaderCard.vue'
import StatusTag from '@/components/StatusTag.vue'
import { useTaskStore } from '@/stores/tasks'
import type { Task, TaskStatus } from '@/types/api'
import { apiFetch, getToken } from '@/utils/api'
import { formatBytes, formatRemaining, formatSpeed, formatTime } from '@/utils/format'

const router = useRouter()
const taskStore = useTaskStore()

const filterStatus = ref('')
const selectedTaskIds = ref<string[]>([])
const allSelected = ref(false)
const diskInfo = ref<{ total_bytes: number; used_bytes: number; free_bytes: number } | null>(null)
const viewMode = ref<'table' | 'card'>(typeof window !== 'undefined' && window.innerWidth < 768 ? 'card' : 'table')
// Comma-separated values group statuses (进行中 / 失败异常) into one filter.
const filterOptions: { value: string; label: string }[] = [
  { value: 'queued', label: '排队中' },
  { value: 'downloading,uploading,saving', label: '进行中' },
  { value: 'download_failed,upload_failed', label: '失败异常' },
  { value: 'completed', label: '已完成' },
  { value: 'canceled', label: '已取消' },
  { value: 'downloading', label: '下载中' },
  { value: 'uploading', label: '上传中' },
  { value: 'saving', label: '保存中' },
  { value: 'upload_pending', label: '待上传' },
  { value: 'download_completed', label: '下载完成' },
  { value: 'download_failed', label: '下载失败' },
  { value: 'upload_failed', label: '上传失败' },
]

function setFilter(value: string) {
  filterStatus.value = value
  reload()
}

function syncViewMode() {
  if (typeof window === 'undefined') return
  viewMode.value = window.innerWidth < 768 ? 'card' : 'table'
}

const runtimeDescription = computed(() => {
  if (!taskStore.runtime) return ''
  const ids = taskStore.runtime.current_task_ids
  if (!ids?.length) return `最大并发：${taskStore.runtime.max_concurrency}，当前没有正在执行的任务`
  return `最大并发：${taskStore.runtime.max_concurrency}，正在执行：${ids.join(', ')}`
})

type LiveTaskEvent = {
  task_id: string
  status?: string
  dl_prog?: number
  dl_speed?: number
  dl_done?: number
  dl_total?: number
  ul_prog?: number
  ul_speed?: number
  ul_done?: number
  ul_total?: number
}

let backupTimer: number | undefined
let softRefreshTimer: number | undefined
let eventSource: EventSource | undefined
let pendingEvents: LiveTaskEvent[] = []
let flushTimer: number | undefined
let reconnectTimer: number | undefined
let pageVisible = true

function connectSSE() {
  if (!pageVisible || eventSource) return
  const token = getToken()
  const url = token
    ? `/api/tasks/events?token=${encodeURIComponent(token)}`
    : '/api/tasks/events'
  eventSource = new EventSource(url)
  eventSource.onmessage = (e) => {
    try {
      pendingEvents.push(JSON.parse(e.data) as LiveTaskEvent)
      scheduleFlush()
    } catch { /* ignore malformed */ }
  }
  eventSource.onerror = () => {
    eventSource?.close()
    eventSource = undefined
    if (!pageVisible) return
    if (reconnectTimer) window.clearTimeout(reconnectTimer)
    reconnectTimer = window.setTimeout(connectSSE, 3000)
  }
}

function scheduleFlush() {
  if (flushTimer) return
  flushTimer = window.setTimeout(() => {
    flushTimer = undefined
    const events = pendingEvents.splice(0)
    if (!events.length) return

    const latest = new Map<string, LiveTaskEvent>()
    for (const evt of events) {
      if (!evt?.task_id) continue
      const prev = latest.get(evt.task_id)
      latest.set(evt.task_id, prev ? { ...prev, ...evt } : evt)
    }

    let needSoftRefresh = false
    for (const evt of latest.values()) {
      if (evt.status === 'done') {
        needSoftRefresh = true
        continue
      }
      // Ignore the return value: progress events for tasks not on the
      // current page/filter are skipped silently instead of triggering a
      // full refetch (which previously caused a 1s fetch storm → UI freeze).
      applyLiveEvent(evt)
    }
    if (needSoftRefresh) scheduleSoftRefresh()
  }, 250)
}

function applyLiveEvent(evt: LiveTaskEvent): boolean {
  const hasDownload =
    evt.dl_prog !== undefined ||
    evt.dl_done !== undefined ||
    evt.dl_total !== undefined ||
    evt.dl_speed !== undefined
  const hasUpload =
    evt.ul_prog !== undefined ||
    evt.ul_done !== undefined ||
    evt.ul_total !== undefined ||
    evt.ul_speed !== undefined

  let status: TaskStatus | string | undefined
  switch (evt.status) {
    case 'download':
    case 'downloading':
      status = 'downloading'
      break
    case 'uploading':
      status = 'uploading'
      break
    case 'saving':
      status = 'saving'
      break
    default:
      if (hasDownload) status = 'downloading'
      else if (hasUpload) status = 'uploading'
  }

  return taskStore.applyLiveUpdate({
    taskId: evt.task_id,
    status,
    download: hasDownload
      ? {
          progress: evt.dl_prog,
          speed: evt.dl_speed,
          completed_bytes: evt.dl_done,
          total_bytes: evt.dl_total,
          status: 'active',
        }
      : undefined,
    upload: hasUpload
      ? {
          progress: evt.ul_prog,
          speed: evt.ul_speed,
          uploaded_bytes: evt.ul_done,
          total_bytes: evt.ul_total,
          status: status === 'saving' ? 'saving' : 'uploading',
        }
      : undefined,
  })
}

let lastSoftRefresh = 0
function scheduleSoftRefresh() {
  if (softRefreshTimer) return
  // Rate-limit background refetches: at most one per 3s, otherwise busy
  // queues keep hitting /api/tasks + /stats + /runtime every second.
  const now = Date.now()
  if (now - lastSoftRefresh < 3000) return
  softRefreshTimer = window.setTimeout(async () => {
    softRefreshTimer = undefined
    lastSoftRefresh = Date.now()
    try {
      await Promise.all([
        taskStore.fetchTasks({ status: filterStatus.value, page: taskStore.page }),
        taskStore.fetchTaskStats(),
        taskStore.fetchRuntimeStatus(),
      ])
    } catch { /* ignore background refresh errors */ }
  }, 1000)
}

function startBackupPoll() {
  stopBackupPoll()
  backupTimer = window.setInterval(() => {
    if (!pageVisible) return
    if (!taskStore.hasActiveTasks() && !taskStore.runtime?.current_task_ids?.length) return
    scheduleSoftRefresh()
  }, 12000)
}

function stopBackupPoll() {
  if (backupTimer) {
    window.clearInterval(backupTimer)
    backupTimer = undefined
  }
}

function canRetry(status: TaskStatus) { return ['canceled', 'download_failed', 'upload_failed', 'completed'].includes(status) }
function canCancel(status: TaskStatus) { return ['queued', 'downloading', 'upload_pending', 'uploading', 'saving', 'download_failed', 'upload_failed'].includes(status) }
function isActive(status: TaskStatus) { return ['downloading', 'uploading', 'saving'].includes(status) }

/** Secondary sub-status tag: only shown when it adds info beyond the main status. */
function subStatus(row: Task): string {
  const sub = (row.upload.status || row.download.status || '').trim()
  return sub && sub !== row.status ? sub : ''
}

function onSelectionChange(rows: Task[]) {
  selectedTaskIds.value = rows.map(r => r.id)
  allSelected.value = false
}

async function selectAllTasks() {
  if (allSelected.value) {
    allSelected.value = false
    selectedTaskIds.value = []
    return
  }
  try {
    const resp = await apiFetch(`/api/tasks/ids?status=${filterStatus.value}`)
    const d = await resp.json()
    if (d.success) { selectedTaskIds.value = d.data; allSelected.value = true }
  } catch { /* ignore */ }
}

async function fetchDiskInfo() {
  try {
    const resp = await apiFetch('/api/system/disk')
    const d = await resp.json()
    if (d.success) diskInfo.value = d.data
  } catch { /* ignore */ }
}

async function reload() {
  try {
    await Promise.all([
      taskStore.fetchTasks({ status: filterStatus.value, page: 1 }),
      taskStore.fetchTaskStats(),
      taskStore.fetchRuntimeStatus(),
      fetchDiskInfo(),
    ])
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载任务失败')
  }
}

function goDetail(taskId: string) { router.push(`/tasks/${taskId}`) }

async function handlePause(taskId: string) {
  try { await taskStore.pauseTask(taskId); ElMessage.success('已暂停') }
  catch (e) { ElMessage.error(e instanceof Error ? e.message : '操作失败') }
}

async function handleResume(taskId: string) {
  try { await taskStore.resumeTask(taskId); ElMessage.success('已恢复') }
  catch (e) { ElMessage.error(e instanceof Error ? e.message : '操作失败') }
}

async function handleCancel(taskId: string) {
  try { await taskStore.cancelTask(taskId); await refreshData(); ElMessage.success('任务已取消') }
  catch (e) { ElMessage.error(e instanceof Error ? e.message : '取消失败') }
}

async function handleDelete(taskId: string) {
  try {
    await ElMessageBox.confirm('确定要删除此任务吗？', '删除任务', { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' })
  } catch { return }
  try {
    await taskStore.deleteTask(taskId)
    await taskStore.fetchTaskStats()
    ElMessage.success('任务已删除')
  } catch (error) { ElMessage.error(error instanceof Error ? error.message : '删除任务失败') }
}

async function handleBatchPause() {
  const ids = selectedTaskIds.value
  if (!ids.length) return
  try {
    const resp = await apiFetch('/api/tasks/batch-pause', { method: 'POST', body: JSON.stringify({ ids }) })
    const d = await resp.json()
    if (d.success) { selectedTaskIds.value = []; refreshData(); ElMessage.success('已暂停 ' + (d.data.paused?.length || 0) + ' 个任务') }
  } catch { ElMessage.error('操作失败') }
}

async function handleBatchResume() {
  const ids = selectedTaskIds.value
  if (!ids.length) return
  try {
    const resp = await apiFetch('/api/tasks/batch-resume', { method: 'POST', body: JSON.stringify({ ids }) })
    const d = await resp.json()
    if (d.success) { selectedTaskIds.value = []; refreshData(); ElMessage.success('已恢复 ' + (d.data.resumed?.length || 0) + ' 个任务') }
  } catch { ElMessage.error('操作失败') }
}

async function handleBatchDelete() {
  const ids = selectedTaskIds.value
  if (!ids.length) return
  try {
    await ElMessageBox.confirm(`确定要删除选中的 ${ids.length} 个任务吗？`, '批量删除', { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' })
  } catch { return }
  try {
    const resp = await apiFetch('/api/tasks/batch-delete', {
      method: 'POST',
      body: JSON.stringify({ ids }),
    })
    const data = await resp.json()
    if (data.success) {
      selectedTaskIds.value = []
      await refreshData()
      await taskStore.fetchTaskStats()
      ElMessage.success(`已删除 ${data.data.deleted?.length || 0} 个任务`)
    }
  } catch (error) { ElMessage.error(error instanceof Error ? error.message : '批量删除失败') }
}

async function handleRetry(taskId: string) {
  try {
    await taskStore.retryTask(taskId)
    await refreshData()
    ElMessage.success('任务已重新入队')
  } catch (error) { ElMessage.error(error instanceof Error ? error.message : '重试任务失败') }
}

async function refreshData() {
  await Promise.all([
    taskStore.fetchTasks({ status: filterStatus.value, page: taskStore.page }),
    taskStore.fetchTaskStats(),
    taskStore.fetchRuntimeStatus(),
    fetchDiskInfo(),
  ])
}

async function handlePageChange(page: number) {
  try { await taskStore.fetchTasks({ status: filterStatus.value, page }) }
  catch (error) { ElMessage.error(error instanceof Error ? error.message : '切换分页失败') }
}

onMounted(() => {
  syncViewMode()
  window.addEventListener('resize', syncViewMode)
  reload()
  connectSSE()
  startBackupPoll()

  function onVisibilityChange() {
    pageVisible = !document.hidden
    if (document.hidden) {
      eventSource?.close()
      eventSource = undefined
      if (reconnectTimer) { window.clearTimeout(reconnectTimer); reconnectTimer = undefined }
      if (flushTimer) { window.clearTimeout(flushTimer); flushTimer = undefined }
      pendingEvents = []
    } else {
      connectSSE()
      scheduleSoftRefresh()
    }
  }

  document.addEventListener('visibilitychange', onVisibilityChange)
  onUnmounted(() => {
    pageVisible = false
    eventSource?.close()
    eventSource = undefined
    stopBackupPoll()
    if (reconnectTimer) { window.clearTimeout(reconnectTimer); reconnectTimer = undefined }
    if (flushTimer) { window.clearTimeout(flushTimer); flushTimer = undefined }
    if (softRefreshTimer) { window.clearTimeout(softRefreshTimer); softRefreshTimer = undefined }
    pendingEvents = []
    document.removeEventListener('visibilitychange', onVisibilityChange)
    window.removeEventListener('resize', syncViewMode)
  })
})
</script>

<style scoped>
.task-queue-view { display: flex; flex-direction: column; gap: 16px; }

/* Toolbar */
.toolbar {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}
.toolbar-spacer { flex: 1; }

.filter-select { width: 140px; }

.tool-btn { display: flex; align-items: center; gap: 4px; }
.view-toggle { padding: 6px; min-width: unset; }
.btn-label {}

@media (max-width: 640px) {
  .toolbar { gap: 4px; }
  .filter-select { width: 120px; }
  .btn-label { display: none; }
  .tool-btn { padding: 8px 10px; min-width: unset; }
}

.runtime-alert { margin-bottom: 0; }

/* Stats Dashboard */
.stats-dashboard { display: grid; grid-template-columns: repeat(6, 1fr); gap: 12px; }
.stat-widget { background: var(--bg-panel); border: 1px solid var(--line-soft); border-radius: 12px; padding: 16px 20px; text-align: center; cursor: pointer; transition: border-color 0.15s, background 0.15s; }
.stat-widget:hover { border-color: var(--line-strong, #9ca3af); }
.stat-widget.active { outline: 2px solid var(--brand); outline-offset: -1px; background: color-mix(in srgb, var(--brand) 8%, transparent); }
.stat-widget.primary { border-left: 3px solid var(--brand); }
.stat-widget.warning { border-left: 3px solid #eab308; }
.stat-widget.info { border-left: 3px solid #3b82f6; }
.stat-widget.muted { border-left: 3px solid #9ca3af; }
.stat-widget.danger { border-left: 3px solid #ef4444; }
.stat-widget.success { border-left: 3px solid #22c55e; }
.stat-label { display: block; color: var(--text-subtle); font-size: 12px; margin-bottom: 4px; }
.stat-content strong { font-size: 22px; color: var(--text-main); }

/* Table View */
.task-target { display: flex; flex-direction: column; gap: 2px; }
.file-name { font-weight: 600; color: var(--text-main); }
.task-id { font-size: 11px; color: var(--text-muted); font-family: monospace; }
.task-meta { display: flex; gap: 8px; align-items: center; }
.task-ep { font-size: 11px; color: var(--brand); font-weight: 600; font-family: monospace; }

.status-progress-cell { display: flex; flex-direction: column; gap: 8px; }
.status-row { display: flex; align-items: center; gap: 6px; }
.progress-detail { display: flex; justify-content: space-between; font-size: 12px; color: var(--text-subtle); }

.error-text { font-size: 12px; color: #ef4444; }

.action-group { display: flex; align-items: center; gap: 4px; }
.action-btn { min-width: 52px; text-align: center; }
.popover-actions { display: flex; flex-direction: column; min-width: 120px; }
.popover-item { padding: 8px 12px; font-size: 13px; cursor: pointer; border-radius: 6px; transition: background 0.15s; white-space: nowrap; }
.popover-item:hover:not(.disabled) { background: rgba(0,0,0,0.04); }
.popover-item.danger { color: #ef4444; }
.popover-item.warning { color: #eab308; }
.popover-item.primary { color: var(--el-color-primary); }
.popover-item.success { color: var(--el-color-success); }
.popover-item.disabled { opacity: 0.35; cursor: not-allowed; }
.popover-divider { height: 1px; background: var(--line-soft); margin: 4px 0; }

/* Card View (Mobile) */
.card-list { display: flex; flex-direction: column; gap: 10px; }

.task-card {
  background: var(--bg-panel);
  border: 1px solid var(--line-soft);
  border-radius: 12px;
  box-shadow: var(--shadow-sm);
  overflow: hidden;
  cursor: pointer;
  transition: box-shadow 0.2s ease, border-color 0.2s ease;
}
.task-card:active { box-shadow: var(--shadow-md); }

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid var(--line-soft);
  background: var(--bg-hover);
}
.card-status-row { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.card-ep {
  font-size: 12px;
  color: var(--brand);
  font-weight: 600;
  font-family: monospace;
  background: var(--brand-soft);
  padding: 1px 6px;
  border-radius: 4px;
}
.card-chevron { color: var(--text-muted); flex-shrink: 0; }

.card-body {
  padding: 12px 16px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.card-filename {
  font-weight: 600;
  font-size: 14px;
  color: var(--text-main);
  word-break: break-all;
  line-height: 1.4;
}
.card-meta { display: flex; justify-content: space-between; align-items: center; }
.card-task-id { font-size: 11px; color: var(--text-muted); font-family: monospace; }
.card-time { font-size: 11px; color: var(--text-muted); }

.card-progress { display: flex; flex-direction: column; gap: 6px; }
.progress-header { display: flex; justify-content: space-between; font-size: 12px; color: var(--text-subtle); font-weight: 500; }
.progress-stats { display: flex; justify-content: space-between; font-size: 11px; color: var(--text-muted); }

.card-error {
  font-size: 12px;
  color: #ef4444;
  background: rgba(239, 68, 68, 0.06);
  padding: 8px 12px;
  border-radius: 8px;
  line-height: 1.4;
  word-break: break-all;
}

.card-actions {
  display: flex;
  border-top: 1px solid var(--line-soft);
  padding: 0;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
}
.card-action-btn {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 3px;
  padding: 10px 6px;
  border: none;
  background: transparent;
  color: var(--text-subtle);
  font-size: 11px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
  white-space: nowrap;
  min-width: 56px;
}
.card-action-btn:not(:last-child) { border-right: 1px solid var(--line-soft); }
.card-action-btn:hover:not(:disabled) { background: var(--bg-hover); color: var(--text-main); }
.card-action-btn:disabled { opacity: 0.3; cursor: not-allowed; }
.card-action-btn.danger:hover:not(:disabled) { color: #ef4444; background: rgba(239, 68, 68, 0.06); }

/* Pagination */
.pagination-container { padding: 14px 20px; display: flex; justify-content: flex-end; border-top: 1px solid var(--line-soft); }

/* Shared */
.muted-text { color: var(--text-subtle); font-size: 12px; }

/* Responsive */
@media (max-width: 960px) { .stats-dashboard { grid-template-columns: repeat(2, 1fr); } }
@media (max-width: 600px) {
  .stats-dashboard { grid-template-columns: 1fr 1fr; gap: 8px; }
  .stat-widget { padding: 12px 14px; }
  .stat-content strong { font-size: 18px; }
  .pagination-container { padding: 12px; justify-content: center; }
}

@media (max-width: 480px) {
  .task-queue-view { gap: 10px; }
  .stats-dashboard { grid-template-columns: 1fr 1fr; gap: 6px; }
}
</style>
