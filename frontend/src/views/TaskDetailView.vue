<template>
  <div>
    <PageHeaderCard title="任务详情" subtitle="展示任务快照、恢复后的运行状态，以及结构化错误信息。">
      <div class="header-actions">
        <el-button :loading="taskStore.loading" @click="loadDetail">刷新详情</el-button>
        <el-button
          type="danger"
          plain
          :disabled="!task || !canCancel(task.status)"
          @click="cancelTask"
        >
          取消任务
        </el-button>
        <el-button type="warning" plain :disabled="!task || !canRetry(task.status)" @click="retryTask">
          重新入队
        </el-button>
      </div>
    </PageHeaderCard>

    <el-row :gutter="16">
      <el-col :xs="24" :lg="12">
        <el-card v-if="task" class="detail-card">
          <template #header>基础信息</template>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="任务 ID">{{ task.id }}</el-descriptions-item>
            <el-descriptions-item label="扫描会话">{{ task.scan_session_id }}</el-descriptions-item>
            <el-descriptions-item label="扫描项">{{ task.scan_item_id }}</el-descriptions-item>
            <el-descriptions-item label="主状态">
              <StatusTag :status="task.status" />
            </el-descriptions-item>
            <el-descriptions-item label="上传子状态">
              <StatusTag :status="task.upload.status || 'pending'" />
            </el-descriptions-item>
            <el-descriptions-item label="重试次数">{{ task.retry_count }}</el-descriptions-item>
            <el-descriptions-item label="创建时间">{{ formatTime(task.created_at) }}</el-descriptions-item>
            <el-descriptions-item label="更新时间">{{ formatTime(task.updated_at) }}</el-descriptions-item>
            <el-descriptions-item label="结束时间">{{ formatTime(task.finished_at) }}</el-descriptions-item>
          </el-descriptions>
        </el-card>

        <el-card v-if="task" class="detail-card">
          <template #header>来源快照</template>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="来源类型">{{ task.source.type }}</el-descriptions-item>
            <el-descriptions-item label="OpenList 路径">{{ task.source.path }}</el-descriptions-item>
            <el-descriptions-item label="文件名">{{ task.source.file_name }}</el-descriptions-item>
            <el-descriptions-item label="文件大小">{{ formatBytes(task.source.file_size) }}</el-descriptions-item>
            <el-descriptions-item label="直链">
              <span class="url-text">{{ task.source.raw_url || '-' }}</span>
            </el-descriptions-item>
          </el-descriptions>
        </el-card>

        <el-card v-if="task" class="detail-card">
          <template #header>解析与目标</template>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="季">{{ task.parsed.season ?? '-' }}</el-descriptions-item>
            <el-descriptions-item label="集">{{ task.parsed.episode ?? '-' }}</el-descriptions-item>
            <el-descriptions-item label="特别篇">
              {{ task.parsed.is_special ? '是' : '否' }}
            </el-descriptions-item>
            <el-descriptions-item label="TMDB ID">{{ task.target.tmdb_id }}</el-descriptions-item>
            <el-descriptions-item label="item_type">{{ task.target.item_type }}</el-descriptions-item>
            <el-descriptions-item label="item_id">{{ task.target.item_id }}</el-descriptions-item>
            <el-descriptions-item label="标题">{{ task.target.title }}</el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-col>

      <el-col :xs="24" :lg="12">
        <el-card v-if="task" class="detail-card">
          <template #header>下载字段</template>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="aria2 gid">{{ task.download.aria2_gid || '-' }}</el-descriptions-item>
            <el-descriptions-item label="下载目录">{{ task.download.save_dir || '-' }}</el-descriptions-item>
            <el-descriptions-item label="本地路径">
              <span class="url-text">{{ task.download.local_path || '-' }}</span>
            </el-descriptions-item>
            <el-descriptions-item label="下载状态">{{ task.download.status || '-' }}</el-descriptions-item>
            <el-descriptions-item label="总大小">{{ formatBytes(task.download.total_bytes) }}</el-descriptions-item>
            <el-descriptions-item label="已完成">{{ formatBytes(task.download.completed_bytes) }}</el-descriptions-item>
            <el-descriptions-item label="进度">
              <div class="progress-wrapper">
                <el-progress :percentage="Math.round(task.download.progress || 0)" />
                <span>{{ (task.download.progress || 0).toFixed(2) }}%</span>
              </div>
            </el-descriptions-item>
            <el-descriptions-item label="速度">{{ formatSpeed(task.download.speed) }}</el-descriptions-item>
          </el-descriptions>
        </el-card>

        <el-card v-if="task" class="detail-card">
          <template #header>上传字段</template>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="存储">{{ task.upload.storage }}</el-descriptions-item>
            <el-descriptions-item label="上传状态">
              <StatusTag :status="task.upload.status || 'pending'" />
            </el-descriptions-item>
            <el-descriptions-item label="file_id">{{ task.upload.file_id || '-' }}</el-descriptions-item>
            <el-descriptions-item label="upload_url">
              <span class="url-text">{{ maskUrl(task.upload.upload_url) }}</span>
            </el-descriptions-item>
            <el-descriptions-item label="media_id">{{ task.upload.media_id || '-' }}</el-descriptions-item>
            <el-descriptions-item label="总大小">{{ formatBytes(task.upload.total_bytes) }}</el-descriptions-item>
            <el-descriptions-item label="已上传">{{ formatBytes(task.upload.uploaded_bytes) }}</el-descriptions-item>
            <el-descriptions-item label="进度">
              <div class="progress-wrapper">
                <el-progress :percentage="Math.round(task.upload.progress || 0)" />
                <span>{{ (task.upload.progress || 0).toFixed(2) }}%</span>
              </div>
            </el-descriptions-item>
            <el-descriptions-item label="速度">{{ formatSpeed(task.upload.speed) }}</el-descriptions-item>
            <el-descriptions-item label="save 重试次数">{{ task.upload.save_retry_count }}</el-descriptions-item>
            <el-descriptions-item label="最近 save 错误">
              {{ task.upload.last_save_error || '-' }}
            </el-descriptions-item>
          </el-descriptions>
        </el-card>

        <el-card v-if="task" class="detail-card">
          <template #header>执行结果</template>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="错误信息">
              {{ task.result.error_message || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="错误阶段">
              {{ task.result.error_stage || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="错误码">
              {{ task.result.error_code || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="最近错误时间">
              {{ formatTime(task.result.last_error_at) }}
            </el-descriptions-item>
          </el-descriptions>
        </el-card>

        <el-card class="detail-card">
          <template #header>任务日志</template>
          <el-timeline v-if="taskStore.activeTaskLog?.items.length">
            <el-timeline-item
              v-for="entry in taskStore.activeTaskLog.items"
              :key="entry.id"
              :timestamp="formatTime(entry.time)"
              :type="entry.level === 'error' ? 'danger' : entry.level === 'warn' ? 'warning' : 'primary'"
            >
              {{ entry.message }}
            </el-timeline-item>
          </el-timeline>
          <el-empty v-else description="暂无日志" />
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useRoute } from 'vue-router'

import PageHeaderCard from '@/components/PageHeaderCard.vue'
import StatusTag from '@/components/StatusTag.vue'
import { useTaskStore } from '@/stores/tasks'
import type { TaskStatus } from '@/types/api'

const route = useRoute()
const taskStore = useTaskStore()

const task = computed(() => taskStore.activeTask)

let timer: number | undefined

function formatTime(value?: string) {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

function formatBytes(value?: number) {
  if (!value) return '0 B'

  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = value
  let unitIndex = 0
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024
    unitIndex += 1
  }
  return `${size.toFixed(unitIndex === 0 ? 0 : 2)} ${units[unitIndex]}`
}

function formatSpeed(value?: number) {
  if (!value) return '0 B/s'
  return `${formatBytes(value)}/s`
}

function maskUrl(value?: string) {
  if (!value) return '-'
  try {
    const url = new URL(value)
    return `${url.origin}${url.pathname.slice(0, 24)}...`
  } catch {
    return `${value.slice(0, 40)}...`
  }
}

function canRetry(status: TaskStatus) {
  return ['canceled', 'download_failed', 'upload_failed'].includes(status)
}

function canCancel(status: TaskStatus) {
  return ['queued', 'downloading', 'upload_pending', 'uploading', 'saving'].includes(status)
}

async function loadDetail() {
  const taskId = route.params.id as string
  try {
    await Promise.all([taskStore.fetchTask(taskId), taskStore.fetchTaskLog(taskId)])
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载任务详情失败')
  }
}

async function cancelTask() {
  if (!task.value) return
  try {
    await taskStore.cancelTask(task.value.id)
    await taskStore.fetchTaskLog(task.value.id)
    ElMessage.success('任务已取消')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '取消任务失败')
  }
}

async function retryTask() {
  if (!task.value) return
  try {
    await taskStore.retryTask(task.value.id)
    await taskStore.fetchTaskLog(task.value.id)
    ElMessage.success('任务已重新入队')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '重新入队失败')
  }
}

onMounted(() => {
  loadDetail()
  timer = window.setInterval(() => {
    void loadDetail()
  }, 4000)
})

onUnmounted(() => {
  if (timer) {
    window.clearInterval(timer)
  }
})
</script>

<style scoped>
.header-actions {
  display: flex;
  gap: 12px;
}

.detail-card {
  margin-bottom: 16px;
  border-radius: 20px;
}

.url-text {
  word-break: break-all;
  color: #6a746f;
}

.progress-wrapper {
  display: grid;
  gap: 8px;
}

@media (max-width: 960px) {
  .header-actions {
    width: 100%;
    flex-wrap: wrap;
  }
}
</style>
