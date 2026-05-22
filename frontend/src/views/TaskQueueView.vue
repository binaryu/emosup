<template>
  <div>
    <PageHeaderCard
      title="任务队列"
      subtitle="第六阶段强化了恢复、重试与错误分类。这里会持续展示主状态、子状态和失败原因。"
    >
      <div class="toolbar">
        <el-select v-model="filterStatus" placeholder="全部状态" clearable style="width: 180px" @change="reload">
          <el-option v-for="status in statuses" :key="status" :label="status" :value="status" />
        </el-select>
        <el-button :loading="taskStore.loading" @click="reload">刷新</el-button>
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

    <div v-if="taskStore.stats" class="stats-grid">
      <el-card class="stat-card">
        <span class="stat-label">总任务</span>
        <strong>{{ taskStore.stats.total }}</strong>
      </el-card>
      <el-card class="stat-card">
        <span class="stat-label">排队中</span>
        <strong>{{ taskStore.stats.queued }}</strong>
      </el-card>
      <el-card class="stat-card">
        <span class="stat-label">已取消</span>
        <strong>{{ taskStore.stats.canceled }}</strong>
      </el-card>
      <el-card class="stat-card">
        <span class="stat-label">失败</span>
        <strong>{{ taskStore.stats.failed }}</strong>
      </el-card>
    </div>

    <el-card class="queue-card">
      <el-table :data="taskStore.tasks" stripe>
        <el-table-column prop="id" label="任务 ID" min-width="220" />
        <el-table-column label="文件名" min-width="220">
          <template #default="{ row }">
            {{ row.source.file_name }}
          </template>
        </el-table-column>
        <el-table-column label="主状态" width="120">
          <template #default="{ row }">
            <StatusTag :status="row.status" />
          </template>
        </el-table-column>
        <el-table-column label="子状态" width="130">
          <template #default="{ row }">
            <StatusTag :status="row.upload.status || row.download.status || 'pending'" />
          </template>
        </el-table-column>
        <el-table-column label="下载进度" min-width="220">
          <template #default="{ row }">
            <div class="progress-cell">
              <el-progress :percentage="Math.round(row.download.progress || 0)" :stroke-width="10" />
              <span class="muted-text">
                {{ formatBytes(row.download.completed_bytes) }} / {{ formatBytes(row.download.total_bytes) }}
              </span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="上传进度" min-width="220">
          <template #default="{ row }">
            <div class="progress-cell">
              <el-progress :percentage="Math.round(row.upload.progress || 0)" :stroke-width="10" />
              <span class="muted-text">
                {{ formatBytes(row.upload.uploaded_bytes) }} / {{ formatBytes(row.upload.total_bytes) }}
              </span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="速度" width="150">
          <template #default="{ row }">
            {{ formatSpeed(activeSpeed(row)) }}
          </template>
        </el-table-column>
        <el-table-column label="错误码" width="150">
          <template #default="{ row }">
            <span class="error-text">{{ row.result.error_code || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="错误摘要" min-width="240">
          <template #default="{ row }">
            <span class="error-text">
              {{ row.upload.last_save_error || row.result.error_message || '-' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="更新时间" min-width="180">
          <template #default="{ row }">
            {{ formatTime(row.updated_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="260" fixed="right">
          <template #default="{ row }">
            <div class="action-group">
              <el-button link type="primary" @click="goDetail(row.id)">详情</el-button>
              <el-button
                link
                type="danger"
                :disabled="!canCancel(row.status)"
                @click="handleCancel(row.id)"
              >
                取消
              </el-button>
              <el-button link type="warning" :disabled="!canRetry(row.status)" @click="handleRetry(row.id)">
                重试
              </el-button>
              <el-button
                link
                type="danger"
                :disabled="isActive(row.status)"
                @click="handleDelete(row.id)"
              >
                删除
              </el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-row">
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
  if (!taskStore.runtime?.current_task_id) {
    return '当前没有正在执行的任务'
  }

  const stage = taskStore.runtime.current_stage ? `，阶段：${taskStore.runtime.current_stage}` : ''
  const startedAt = taskStore.runtime.started_at ? `，开始于：${formatTime(taskStore.runtime.started_at)}` : ''
  return `当前任务：${taskStore.runtime.current_task_id}${stage}${startedAt}`
})

let timer: number | undefined

function activeSpeed(task: Task) {
  return task.upload.speed || task.download.speed
}

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
})

onUnmounted(() => {
  if (timer) {
    window.clearInterval(timer)
  }
})
</script>

<style scoped>
.toolbar {
  display: flex;
  gap: 12px;
}

.runtime-alert {
  margin-bottom: 16px;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}

.stat-card {
  border-radius: 20px;
}

.stat-label {
  display: block;
  margin-bottom: 10px;
  color: #6a746f;
  font-size: 13px;
}

.stat-card strong {
  font-size: 28px;
}

.queue-card {
  border-radius: 20px;
}

.progress-cell {
  display: grid;
  gap: 8px;
}

.action-group {
  display: flex;
  gap: 6px;
}

.pagination-row {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

.muted-text,
.error-text {
  color: #6a746f;
  font-size: 12px;
}

@media (max-width: 960px) {
  .stats-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .toolbar {
    width: 100%;
    flex-wrap: wrap;
  }
}
</style>
