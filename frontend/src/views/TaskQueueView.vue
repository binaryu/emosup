<template>
  <div class="task-queue-view">
    <PageHeaderCard
      title="任务队列"
      subtitle="实时追踪所有离线下载与转存任务的执行状态、进度与错误信息。"
    >
      <div class="toolbar">
        <el-select v-model="filterStatus" placeholder="筛选状态" clearable style="width: 160px" @change="reload">
          <el-option v-for="status in statuses" :key="status" :label="status" :value="status" />
        </el-select>
        <el-button :loading="taskStore.loading" @click="reload">刷新</el-button>
        <el-button
          v-if="selectedTaskIds.length > 0"
          type="danger"
          plain
          @click="handleBatchDelete"
        >
          批量删除 ({{ selectedTaskIds.length }})
        </el-button>
        <el-button
          v-if="taskStore.total > 0"
          link
          type="primary"
          @click="selectAllTasks"
        >
          {{ allSelected ? '取消全选' : '全选所有' }}
        </el-button>
      </div>
    </PageHeaderCard>

    <el-alert
      v-if="taskStore.runtime"
      class="runtime-alert"
      :title="taskStore.runtime.scheduler_running ? 'Scheduler 运行中' : 'Scheduler 未运行'"
      :type="taskStore.runtime.scheduler_running ? 'success' : 'warning'"
      :description="runtimeDescription"
      :closable="false"
      show-icon
    />

    <div v-if="taskStore.stats" class="stats-dashboard">
      <div class="stat-widget primary">
        <div class="stat-content"><span class="stat-label">总任务</span><strong>{{ taskStore.stats.total }}</strong></div>
      </div>
      <div class="stat-widget warning">
        <div class="stat-content"><span class="stat-label">排队中</span><strong>{{ taskStore.stats.queued }}</strong></div>
      </div>
      <div class="stat-widget muted">
        <div class="stat-content"><span class="stat-label">已取消</span><strong>{{ taskStore.stats.canceled }}</strong></div>
      </div>
      <div class="stat-widget danger">
        <div class="stat-content"><span class="stat-label">失败异常</span><strong>{{ taskStore.stats.failed }}</strong></div>
      </div>
    </div>

    <el-card class="queue-card" :body-style="{ padding: '0' }">
      <el-table
        :data="taskStore.tasks"
        stripe
        class="task-table"
        @selection-change="onSelectionChange"
      >
        <el-table-column type="selection" width="42" />
        <el-table-column label="任务目标" min-width="220" show-overflow-tooltip>
          <template #default="{ row }">
            <div class="task-target">
              <span class="file-name">{{ row.source.file_name }}</span>
              <span class="task-id">{{ row.id }}</span>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="状态 / 进度" min-width="320">
          <template #default="{ row }">
            <div class="status-progress-cell">
              <div class="status-row">
                <StatusTag :status="row.status" />
                <StatusTag v-if="isActive(row.status) && (row.upload.status || row.download.status)" :status="row.upload.status || row.download.status" size="small" />
              </div>
              <template v-if="row.status === 'downloading'">
                <el-progress :percentage="Math.round(row.download.progress || 0)" :stroke-width="6" :show-text="false" />
                <div class="progress-detail">
                  <span>{{ formatBytes(row.download.completed_bytes) }} / {{ formatBytes(row.download.total_bytes) }}</span>
                  <span>{{ formatSpeed(row.download.speed) }}</span>
                </div>
              </template>
              <template v-else-if="['uploading', 'saving'].includes(row.status)">
                <el-progress :percentage="Math.round(row.upload.progress || 0)" :stroke-width="6" :show-text="false" status="success" />
                <div class="progress-detail">
                  <span>{{ formatBytes(row.upload.uploaded_bytes) }} / {{ formatBytes(row.upload.total_bytes) }}</span>
                  <span>{{ formatSpeed(row.upload.speed) }}</span>
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
import { parseApiResponse } from '@/utils/api'
import { formatBytes, formatSpeed, formatTime } from '@/utils/format'

const router = useRouter()
const taskStore = useTaskStore()

const filterStatus = ref<TaskStatus | ''>('')
const selectedTaskIds = ref<string[]>([])
const allSelected = ref(false)
const statuses: TaskStatus[] = [
  'queued', 'downloading', 'download_failed', 'download_completed',
  'upload_pending', 'uploading', 'saving', 'upload_failed', 'completed', 'canceled',
]

const runtimeDescription = computed(() => {
  if (!taskStore.runtime) return ''
  const ids = taskStore.runtime.current_task_ids
  if (!ids?.length) return `最大并发：${taskStore.runtime.max_concurrency}，当前没有正在执行的任务`
  return `最大并发：${taskStore.runtime.max_concurrency}，正在执行：${ids.join(', ')}`
})

let timer: number | undefined
let eventSource: EventSource | undefined
let progressTimer: number | undefined

function connectSSE() {
  if (eventSource) return
  eventSource = new EventSource('/api/tasks/events')
  eventSource.onmessage = (e) => {
    try {
      const evt = JSON.parse(e.data)
      // In-place update the matching task row
      const task = taskStore.tasks.find(t => t.id === evt.task_id)
      if (task) {
        if (evt.status === 'done') {
          // Task completed/finished - do a full reload to get final state
          refreshData()
        } else {
          // Update progress in-place without HTTP requests
          if (evt.status) task.status = evt.status as TaskStatus
          if (evt.dl_prog !== undefined) {
            task.download.progress = evt.dl_prog
            task.download.speed = evt.dl_speed
            task.download.completed_bytes = evt.dl_done
            task.download.total_bytes = evt.dl_total
          }
          if (evt.ul_prog !== undefined) {
            task.upload.progress = evt.ul_prog
            task.upload.speed = evt.ul_speed
            task.upload.uploaded_bytes = evt.ul_done
            task.upload.total_bytes = evt.ul_total
          }
        }
      }
    } catch { /* ignore parse errors */ }
  }
  eventSource.onerror = () => {
    eventSource?.close()
    eventSource = undefined
    setTimeout(connectSSE, 3000)
  }
}

function canRetry(status: TaskStatus) { return ['canceled', 'download_failed', 'upload_failed'].includes(status) }
function canCancel(status: TaskStatus) { return ['queued', 'downloading', 'upload_pending', 'uploading', 'saving', 'download_failed', 'upload_failed'].includes(status) }
function isActive(status: TaskStatus) { return ['downloading', 'uploading', 'saving'].includes(status) }

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
    const resp = await fetch(`/api/tasks/ids?status=${filterStatus.value}`)
    const d = await resp.json()
    if (d.success) { selectedTaskIds.value = d.data; allSelected.value = true }
  } catch { /* ignore */ }
}

async function reload() {
  try {
    await Promise.all([
      taskStore.fetchTasks({ status: filterStatus.value, page: 1 }),
      taskStore.fetchTaskStats(),
      taskStore.fetchRuntimeStatus(),
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

async function handleBatchDelete() {
  const ids = selectedTaskIds.value
  if (!ids.length) return
  try {
    await ElMessageBox.confirm(`确定要删除选中的 ${ids.length} 个任务吗？`, '批量删除', { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' })
  } catch { return }
  try {
    const resp = await fetch('/api/tasks/batch-delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
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
  ])
}

async function handlePageChange(page: number) {
  try { await taskStore.fetchTasks({ status: filterStatus.value, page }) }
  catch (error) { ElMessage.error(error instanceof Error ? error.message : '切换分页失败') }
}

onMounted(() => {
  reload()
  connectSSE()

  function onVisibilityChange() {
    if (document.hidden) {
      eventSource?.close()
      eventSource = undefined
      if (progressTimer) { window.clearInterval(progressTimer); progressTimer = undefined }
      if (timer) { window.clearInterval(timer); timer = undefined }
    } else {
      connectSSE()
    }
  }

  document.addEventListener('visibilitychange', onVisibilityChange)
  onUnmounted(() => {
    eventSource?.close()
    eventSource = undefined
    if (progressTimer) { window.clearInterval(progressTimer); progressTimer = undefined }
    if (timer) { window.clearInterval(timer); timer = undefined }
    document.removeEventListener('visibilitychange', onVisibilityChange)
  })
})
</script>

<style scoped>
.task-queue-view { display: flex; flex-direction: column; gap: 16px; }
.toolbar { display: flex; gap: 12px; align-items: center; }
.runtime-alert { margin-bottom: 0; }

.stats-dashboard { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; }
.stat-widget { background: var(--bg-panel); border: 1px solid var(--line-soft); border-radius: 12px; padding: 16px 20px; text-align: center; }
.stat-widget.primary { border-left: 3px solid var(--brand); }
.stat-widget.warning { border-left: 3px solid #eab308; }
.stat-widget.muted { border-left: 3px solid #9ca3af; }
.stat-widget.danger { border-left: 3px solid #ef4444; }
.stat-label { display: block; color: var(--text-subtle); font-size: 12px; margin-bottom: 4px; }
.stat-content strong { font-size: 22px; color: var(--text-main); }

.task-target { display: flex; flex-direction: column; gap: 2px; }
.file-name { font-weight: 600; color: var(--text-main); }
.task-id { font-size: 11px; color: var(--text-muted); font-family: monospace; }

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

.pagination-container { padding: 14px 20px; display: flex; justify-content: flex-end; border-top: 1px solid var(--line-soft); }

.muted-text { color: var(--text-subtle); font-size: 12px; }

@media (max-width: 960px) { .stats-dashboard { grid-template-columns: repeat(2, 1fr); } }
@media (max-width: 600px) { .stats-dashboard { grid-template-columns: 1fr; } .toolbar { flex-wrap: wrap; } }
</style>
