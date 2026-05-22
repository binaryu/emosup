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
                  <el-button link size="small" type="danger" :disabled="!canCancel(row.status)" @click="handleCancel(row.id)">取消任务</el-button>
                  <el-button link size="small" type="warning" :disabled="!canRetry(row.status)" @click="handleRetry(row.id)">重新入队</el-button>
                  <el-divider style="margin: 4px 0" />
                  <el-button link size="small" type="danger" :disabled="isActive(row.status)" @click="handleDelete(row.id)">删除记录</el-button>
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

function canRetry(status: TaskStatus) { return ['canceled', 'download_failed', 'upload_failed'].includes(status) }
function canCancel(status: TaskStatus) { return ['queued', 'downloading', 'upload_pending', 'uploading', 'saving', 'download_failed', 'upload_failed'].includes(status) }
function isActive(status: TaskStatus) { return ['downloading', 'uploading', 'saving'].includes(status) }

function onSelectionChange(rows: Task[]) {
  selectedTaskIds.value = rows.map(r => r.id)
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

async function handleCancel(taskId: string) {
  try {
    await taskStore.cancelTask(taskId)
    await refreshData()
    ElMessage.success('任务已取消')
  } catch (error) { ElMessage.error(error instanceof Error ? error.message : '取消任务失败') }
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
  function startPolling() {
    if (timer) return
    timer = window.setInterval(refreshData, 4000)
  }
  function stopPolling() {
    if (timer) { window.clearInterval(timer); timer = undefined }
  }
  function onVisibilityChange() {
    if (document.hidden) { stopPolling() } else { reload(); startPolling() }
  }
  startPolling()
  document.addEventListener('visibilitychange', onVisibilityChange)
  onUnmounted(() => { stopPolling(); document.removeEventListener('visibilitychange', onVisibilityChange) })
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
.popover-actions { display: flex; flex-direction: column; }
.popover-actions .el-button { justify-content: flex-start; padding: 8px 12px; width: 100%; }

.pagination-container { padding: 14px 20px; display: flex; justify-content: flex-end; border-top: 1px solid var(--line-soft); }

.muted-text { color: var(--text-subtle); font-size: 12px; }

@media (max-width: 960px) { .stats-dashboard { grid-template-columns: repeat(2, 1fr); } }
@media (max-width: 600px) { .stats-dashboard { grid-template-columns: 1fr; } .toolbar { flex-wrap: wrap; } }
</style>
