<template>
  <div class="task-queue-view">
    <PageHeaderCard
      title="任务队列"
      subtitle="实时追踪所有离线下载与转存任务的执行状态、进度与错误信息。"
    >
      <div class="toolbar">
        <el-select v-model="filterStatus" placeholder="筛选状态" clearable style="width: 160px" @change="reload">
          <template #prefix>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3"></polygon></svg>
          </template>
          <el-option v-for="status in statuses" :key="status" :label="status" :value="status" />
        </el-select>
        <el-button :loading="taskStore.loading" @click="reload">
          <template #icon>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"></polyline><polyline points="1 20 1 14 7 14"></polyline><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path></svg>
          </template>
          刷新
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
        <div class="stat-icon"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="3" width="20" height="14" rx="2" ry="2"></rect><line x1="8" y1="21" x2="16" y2="21"></line><line x1="12" y1="17" x2="12" y2="21"></line></svg></div>
        <div class="stat-content">
          <span class="stat-label">总任务</span>
          <strong>{{ taskStore.stats.total }}</strong>
        </div>
      </div>
      <div class="stat-widget warning">
        <div class="stat-icon"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><polyline points="12 6 12 12 16 14"></polyline></svg></div>
        <div class="stat-content">
          <span class="stat-label">排队中</span>
          <strong>{{ taskStore.stats.queued }}</strong>
        </div>
      </div>
      <div class="stat-widget muted">
        <div class="stat-icon"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="15" y1="9" x2="9" y2="15"></line><line x1="9" y1="9" x2="15" y2="15"></line></svg></div>
        <div class="stat-content">
          <span class="stat-label">已取消</span>
          <strong>{{ taskStore.stats.canceled }}</strong>
        </div>
      </div>
      <div class="stat-widget danger">
        <div class="stat-icon"><svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path><line x1="12" y1="9" x2="12" y2="13"></line><line x1="12" y1="17" x2="12.01" y2="17"></line></svg></div>
        <div class="stat-content">
          <span class="stat-label">失败异常</span>
          <strong>{{ taskStore.stats.failed }}</strong>
        </div>
      </div>
    </div>

    <el-card class="queue-card" :body-style="{ padding: '0', overflow: 'visible' }">
      <el-table :data="taskStore.tasks" stripe class="task-table">
        <el-table-column label="任务目标" min-width="260">
          <template #default="{ row }">
            <div class="task-target">
              <span class="file-name" :title="row.source.file_name">{{ row.source.file_name }}</span>
              <span class="task-id">ID: {{ row.id }}</span>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="状态" width="160">
          <template #default="{ row }">
            <div class="status-stack">
              <StatusTag :status="row.status" />
              <StatusTag v-if="isActive(row.status) && (row.upload.status || row.download.status)" :status="row.upload.status || row.download.status" size="small" />
            </div>
          </template>
        </el-table-column>

        <el-table-column label="当前进度" min-width="280">
          <template #default="{ row }">
            <div class="progress-cell">
              <template v-if="row.status === 'downloading'">
                <div class="progress-header">
                  <span class="progress-label">下载中...</span>
                  <span class="progress-speed">{{ formatSpeed(row.download.speed) }}</span>
                </div>
                <el-progress :percentage="Math.round(row.download.progress || 0)" :stroke-width="8" :show-text="false" />
                <div class="progress-footer">
                  <span>{{ formatBytes(row.download.completed_bytes) }} / {{ formatBytes(row.download.total_bytes) }}</span>
                  <span>{{ (row.download.progress || 0).toFixed(1) }}%</span>
                </div>
              </template>
              <template v-else-if="['uploading', 'saving'].includes(row.status)">
                <div class="progress-header">
                  <span class="progress-label">上传中...</span>
                  <span class="progress-speed">{{ formatSpeed(row.upload.speed) }}</span>
                </div>
                <el-progress :percentage="Math.round(row.upload.progress || 0)" :stroke-width="8" :show-text="false" status="success" />
                <div class="progress-footer">
                  <span>{{ formatBytes(row.upload.uploaded_bytes) }} / {{ formatBytes(row.upload.total_bytes) }}</span>
                  <span>{{ (row.upload.progress || 0).toFixed(1) }}%</span>
                </div>
              </template>
              <template v-else-if="row.status === 'completed'">
                <el-progress :percentage="100" :stroke-width="8" status="success" />
                <div class="progress-footer">
                  <span style="color: var(--el-color-success)">已完成转存</span>
                </div>
              </template>
              <template v-else>
                <el-progress :percentage="0" :stroke-width="8" :show-text="false" color="var(--line-soft)" />
                <div class="progress-footer">
                  <span class="muted-text">等待中或已取消</span>
                </div>
              </template>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="错误信息" min-width="200">
          <template #default="{ row }">
            <div v-if="row.result.error_message || row.upload.last_save_error" class="error-cell">
              <span class="error-code" v-if="row.result.error_code">{{ row.result.error_code }}</span>
              <span class="error-msg" :title="row.upload.last_save_error || row.result.error_message">
                {{ row.upload.last_save_error || row.result.error_message }}
              </span>
            </div>
            <span v-else class="muted-text">-</span>
          </template>
        </el-table-column>

        <el-table-column label="更新时间" width="160">
          <template #default="{ row }">
            <span class="muted-text">{{ formatTime(row.updated_at) }}</span>
          </template>
        </el-table-column>

        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <div class="action-group">
              <el-button link type="primary" @click="goDetail(row.id)">详情</el-button>
              <el-divider direction="vertical" />
              <el-popover placement="bottom-end" :width="160" trigger="click" :teleported="true">
                <template #reference>
                  <el-button link>管理<el-icon class="el-icon--right"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="6 9 12 15 18 9"></polyline></svg></el-icon></el-button>
                </template>
                <div class="popover-actions">
                  <el-button link type="danger" :disabled="!canCancel(row.status)" @click="handleCancel(row.id)" size="small">取消任务</el-button>
                  <el-button link type="warning" :disabled="!canRetry(row.status)" @click="handleRetry(row.id)" size="small">重新入队</el-button>
                  <el-divider style="margin: 6px 0" />
                  <el-button link type="danger" :disabled="isActive(row.status)" @click="handleDelete(row.id)" size="small">删除记录</el-button>
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
import { formatBytes, formatSpeed, formatTime } from '@/utils/format'

const router = useRouter()
const taskStore = useTaskStore()

const filterStatus = ref<TaskStatus | ''>('')
const statuses: TaskStatus[] = [
  'queued',
  'downloading',
  'download_failed',
  'download_completed',
  'upload_pending',
  'uploading',
  'saving',
  'upload_failed',
  'completed',
  'canceled',
]

const runtimeDescription = computed(() => {
  if (!taskStore.runtime) return ''
  const ids = taskStore.runtime.current_task_ids
  if (!ids?.length) {
    return `最大并发：${taskStore.runtime.max_concurrency}，当前没有正在执行的任务`
  }
  return `最大并发：${taskStore.runtime.max_concurrency}，正在执行：${ids.join(', ')}`
})

let timer: number | undefined

function canRetry(status: TaskStatus) {
  return ['canceled', 'download_failed', 'upload_failed'].includes(status)
}

function canCancel(status: TaskStatus) {
  return ['queued', 'downloading', 'upload_pending', 'uploading', 'saving', 'download_failed', 'upload_failed'].includes(status)
}

function isActive(status: TaskStatus) {
  return ['downloading', 'uploading', 'saving'].includes(status)
}

async function reload() {
  try {
    await Promise.all([
      taskStore.fetchTasks({
        status: filterStatus.value,
        page: 1,
      }),
      taskStore.fetchTaskStats(),
      taskStore.fetchRuntimeStatus(),
    ])
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载任务失败')
  }
}

function goDetail(taskId: string) {
  router.push(`/tasks/${taskId}`)
}

async function handleCancel(taskId: string) {
  try {
    await taskStore.cancelTask(taskId)
    await Promise.all([
      taskStore.fetchTasks({
        status: filterStatus.value,
        page: taskStore.page,
      }),
      taskStore.fetchTaskStats(),
      taskStore.fetchRuntimeStatus(),
    ])
    ElMessage.success('任务已取消')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '取消任务失败')
  }
}

async function handleDelete(taskId: string) {
  try {
    await ElMessageBox.confirm('确定要删除此任务吗？任务记录和日志将被永久移除。', '删除任务', {
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch {
    return
  }

  try {
    await taskStore.deleteTask(taskId)
    await taskStore.fetchTaskStats()
    ElMessage.success('任务已删除')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '删除任务失败')
  }
}

async function handleRetry(taskId: string) {
  try {
    await taskStore.retryTask(taskId)
    await Promise.all([
      taskStore.fetchTasks({
        status: filterStatus.value,
        page: taskStore.page,
      }),
      taskStore.fetchTaskStats(),
      taskStore.fetchRuntimeStatus(),
    ])
    ElMessage.success('任务已重新入队')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '重试任务失败')
  }
}

async function handlePageChange(page: number) {
  try {
    await taskStore.fetchTasks({
      status: filterStatus.value,
      page,
    })
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '切换分页失败')
  }
}

onMounted(() => {
  reload()

  function startPolling() {
    if (timer) return
    timer = window.setInterval(() => {
      void Promise.all([
        taskStore.fetchTasks({
          status: filterStatus.value,
          page: taskStore.page,
        }),
        taskStore.fetchTaskStats(),
        taskStore.fetchRuntimeStatus(),
      ])
    }, 4000)
  }

  function stopPolling() {
    if (timer) {
      window.clearInterval(timer)
      timer = undefined
    }
  }

  function onVisibilityChange() {
    if (document.hidden) {
      stopPolling()
    } else {
      reload()
      startPolling()
    }
  }

  startPolling()
  document.addEventListener('visibilitychange', onVisibilityChange)

  onUnmounted(() => {
    stopPolling()
    document.removeEventListener('visibilitychange', onVisibilityChange)
  })
})
</script>

<style scoped>
.task-queue-view {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.toolbar {
  display: flex;
  gap: 12px;
  align-items: center;
}

.stats-dashboard {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  margin-bottom: 8px;
}

.stat-widget {
  background: var(--bg-panel);
  border: 1px solid var(--line-soft);
  border-radius: 12px;
  padding: 20px;
  display: flex;
  align-items: center;
  gap: 16px;
  box-shadow: var(--shadow-sm);
  transition: transform 0.2s, box-shadow 0.2s;
}

.stat-widget:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-md);
}

.stat-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  border-radius: 12px;
  flex-shrink: 0;
}

.stat-widget.primary .stat-icon { background: var(--brand-soft); color: var(--brand); }
.stat-widget.warning .stat-icon { background: rgba(234, 179, 8, 0.1); color: #eab308; }
.stat-widget.muted .stat-icon { background: var(--bg-hover); color: var(--text-subtle); }
.stat-widget.danger .stat-icon { background: rgba(239, 68, 68, 0.1); color: #ef4444; }

.stat-content {
  display: flex;
  flex-direction: column;
}

.stat-label {
  color: var(--text-subtle);
  font-size: 13px;
  font-weight: 500;
}

.stat-content strong {
  font-size: 24px;
  color: var(--text-main);
  line-height: 1.2;
  margin-top: 4px;
}

.task-target {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.file-name {
  font-weight: 600;
  color: var(--text-main);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.task-id {
  font-size: 12px;
  color: var(--text-muted);
  font-family: monospace;
}

.status-stack {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 6px;
}

.progress-cell {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding-right: 16px;
}

.progress-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
}

.progress-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--brand);
}

.progress-speed {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-main);
}

.progress-footer {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: var(--text-subtle);
}

.error-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.error-code {
  font-family: monospace;
  font-size: 11px;
  color: #ef4444;
  background: rgba(239, 68, 68, 0.1);
  padding: 2px 6px;
  border-radius: 4px;
  align-self: flex-start;
}

.error-msg {
  font-size: 12px;
  color: var(--text-subtle);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.action-group {
  display: flex;
  align-items: center;
  gap: 6px;
}

.popover-actions {
  display: flex;
  flex-direction: column;
}

.popover-actions .el-button {
  justify-content: flex-start;
  padding: 8px 12px;
}

.text-danger { color: #ef4444; }
.text-warning { color: #eab308; }

.pagination-container {
  padding: 16px 20px;
  display: flex;
  justify-content: flex-end;
  border-top: 1px solid var(--line-soft);
}

@media (max-width: 1200px) {
  .stats-dashboard {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 768px) {
  .stats-dashboard {
    grid-template-columns: 1fr;
  }
  .toolbar {
    width: 100%;
    flex-wrap: wrap;
  }
}
</style>
