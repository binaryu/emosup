<template>
  <div class="task-detail-view">
    <!-- Top Header -->
    <PageHeaderCard title="任务详情" subtitle="查看任务的完整执行上下文、来源快照与实时流转日志。">
      <div class="header-actions">
        <el-button :loading="taskStore.loading" @click="loadDetail" round>
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"></polyline><polyline points="1 20 1 14 7 14"></polyline><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path></svg>
          <span class="btn-label">刷新数据</span>
        </el-button>
      </div>
    </PageHeaderCard>

    <el-row :gutter="24">
      <!-- Left Column: Data Panels -->
      <el-col :xs="24" :lg="16">
        <div class="data-panels">
          <!-- Main Context Card -->
          <el-card class="detail-card" v-if="task">
            <template #header>
              <div class="card-title">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="16" x2="12" y2="12"></line><line x1="12" y1="8" x2="12.01" y2="8"></line></svg>
                核心信息
              </div>
            </template>
            <div class="info-grid">
              <div class="info-item">
                <span class="info-label">任务 ID</span>
                <span class="info-value font-mono">{{ task.id }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">扫描会话</span>
                <span class="info-value">{{ task.scan_session_id }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">创建时间</span>
                <span class="info-value">{{ formatTime(task.created_at) }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">结束时间</span>
                <span class="info-value">{{ formatTime(task.finished_at) || '-' }}</span>
              </div>
            </div>
            
            <el-divider />
            
            <div class="info-section-title">来源快照 (Source)</div>
            <div class="info-grid three-cols">
              <div class="info-item full-width">
                <span class="info-label">文件名</span>
                <span class="info-value filename-text">{{ task.source.file_name }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">文件大小</span>
                <span class="info-value">{{ formatBytes(task.source.file_size) }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">来源类型</span>
                <span class="info-value">{{ task.source.type }}</span>
              </div>
              <div class="info-item full-width">
                <span class="info-label">原始路径</span>
                <span class="info-value text-muted break-all">{{ task.source.path }}</span>
              </div>
            </div>

            <el-divider />

            <div class="info-section-title">目标解析 (Target)</div>
            <div class="info-grid three-cols">
              <div class="info-item">
                <span class="info-label">TMDB ID</span>
                <span class="info-value font-mono">{{ task.target.tmdb_id }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">类型</span>
                <span class="info-value">{{ task.target.item_type }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">识别标题</span>
                <span class="info-value">{{ task.target.title || '-' }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">季 (Season)</span>
                <span class="info-value">{{ task.parsed.season ?? '-' }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">集 (Episode)</span>
                <span class="info-value">{{ task.parsed.episode ?? '-' }}</span>
              </div>
              <div class="info-item">
                <span class="info-label">特别篇</span>
                <span class="info-value">{{ task.parsed.is_special ? '是' : '否' }}</span>
              </div>
            </div>
          </el-card>

          <!-- Execution Card: Download & Upload -->
          <el-card class="detail-card" v-if="task">
            <template #header>
              <div class="card-title">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"></polyline></svg>
                执行状态
              </div>
            </template>
            
            <div class="exec-blocks">
              <div class="exec-block">
                <div class="exec-header">
                  <div class="exec-title">
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path><polyline points="7 10 12 15 17 10"></polyline><line x1="12" y1="15" x2="12" y2="3"></line></svg>
                    下载阶段
                  </div>
                  <StatusTag :status="task.download.status || 'pending'" size="small" />
                </div>
                <div class="exec-progress">
                  <div class="progress-labels">
                    <span>{{ formatBytes(task.download.completed_bytes) }} / {{ formatBytes(task.download.total_bytes) }}</span>
                    <span>{{ formatSpeed(task.download.speed) }} · {{ formatRemaining(task.download.completed_bytes, task.download.total_bytes, task.download.speed) }}</span>
                  </div>
                  <el-progress :percentage="Math.round(task.download.progress || 0)" :stroke-width="6" :show-text="false" />
                </div>
                <div class="info-grid two-cols mt-3">
                  <div class="info-item full-width">
                    <span class="info-label">本地落盘路径</span>
                    <span class="info-value text-muted break-all">{{ task.download.local_path || '-' }}</span>
                  </div>
                </div>
              </div>

              <div class="exec-block">
                <div class="exec-header">
                  <div class="exec-title">
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path><polyline points="17 8 12 3 7 8"></polyline><line x1="12" y1="3" x2="12" y2="15"></line></svg>
                    上传阶段 (Emos)
                  </div>
                  <StatusTag :status="task.upload.status || 'pending'" size="small" />
                </div>
                <div class="exec-progress">
                  <div class="progress-labels">
                    <span>{{ formatBytes(task.upload.uploaded_bytes) }} / {{ formatBytes(task.upload.total_bytes) }}</span>
                    <span>{{ formatSpeed(task.upload.speed) }} · {{ formatRemaining(task.upload.uploaded_bytes, task.upload.total_bytes, task.upload.speed) }}</span>
                  </div>
                  <el-progress :percentage="Math.round(task.upload.progress || 0)" :stroke-width="6" :show-text="false" status="success" />
                </div>
                <div class="info-grid two-cols mt-3">
                  <div class="info-item">
                    <span class="info-label">存储节点</span>
                    <span class="info-value">{{ task.upload.storage || '-' }}</span>
                  </div>
                  <div class="info-item">
                    <span class="info-label">File ID</span>
                    <span class="info-value font-mono">{{ task.upload.file_id || '-' }}</span>
                  </div>
                  <div class="info-item full-width">
                    <span class="info-label">保存重试次数</span>
                    <span class="info-value">{{ task.upload.save_retry_count }}</span>
                  </div>
                  <div class="info-item full-width" v-if="task.upload.last_save_error">
                    <span class="info-label text-danger">最近保存错误</span>
                    <span class="info-value text-danger">{{ task.upload.last_save_error }}</span>
                  </div>
                </div>
              </div>
            </div>
            
            <div class="error-banner" v-if="task.result.error_message">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="8" x2="12" y2="12"></line><line x1="12" y1="16" x2="12.01" y2="16"></line></svg>
              <div class="error-banner-content">
                <strong>全局执行异常 ({{ task.result.error_stage || '未知阶段' }})</strong>
                <p>错误码: {{ task.result.error_code || '无' }}</p>
                <p class="msg">{{ task.result.error_message }}</p>
              </div>
            </div>

          </el-card>
        </div>
      </el-col>

      <!-- Right Column: Sticky Sidebar -->
      <el-col :xs="24" :lg="8">
        <div class="sticky-sidebar">
          
          <el-card class="sidebar-card action-card" v-if="task">
            <div class="main-status">
              <span class="status-label">当前主状态</span>
              <StatusTag :status="task.status" size="large" />
            </div>
            <div class="action-buttons">
              <el-button
                type="danger"
                plain
                class="sidebar-btn"
                :disabled="!task || !canCancel(task.status)"
                @click="cancelTask"
              >
                取消任务
              </el-button>
              <el-button 
                type="warning" 
                plain 
                class="sidebar-btn"
                :disabled="!task || !canRetry(task.status)" 
                @click="retryTask"
              >
                重新入队
              </el-button>
            </div>
          </el-card>

          <el-card class="sidebar-card logs-card">
            <template #header>
              <div class="card-title">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="8" y1="6" x2="21" y2="6"></line><line x1="8" y1="12" x2="21" y2="12"></line><line x1="8" y1="18" x2="21" y2="18"></line><line x1="3" y1="6" x2="3.01" y2="6"></line><line x1="3" y1="12" x2="3.01" y2="12"></line><line x1="3" y1="18" x2="3.01" y2="18"></line></svg>
                运行日志
              </div>
            </template>
            <div class="logs-container">
              <el-timeline v-if="logEntries.length">
                <el-timeline-item
                  v-for="entry in logEntries"
                  :key="entry.id"
                  :timestamp="formatTime(entry.time)"
                  :type="entry.level === 'error' ? 'danger' : entry.level === 'warn' ? 'warning' : 'primary'"
                  :color="entry.level === 'info' ? 'var(--brand)' : ''"
                  size="small"
                >
                  <div class="log-message">{{ entry.message }}</div>
                </el-timeline-item>
              </el-timeline>
              <el-empty v-else description="暂无日志记录" :image-size="60" />
            </div>
          </el-card>
          
        </div>
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
import type { TaskLogEntry, TaskStatus } from '@/types/api'
import { formatBytes, formatRemaining, formatSpeed, formatTime } from '@/utils/format'

const route = useRoute()
const taskStore = useTaskStore()

const task = computed(() => taskStore.activeTask)
const logEntries = computed<TaskLogEntry[]>(() => {
  const items = [...(taskStore.activeTaskLog?.items ?? [])]
  const currentTask = task.value
  if (!currentTask) return items

  const resultError = currentTask.result?.error_message?.trim()
  if (resultError && !items.some((item) => item.level === 'error' && item.message.includes(resultError))) {
    const stage = currentTask.result.error_stage || 'unknown'
    const code = currentTask.result.error_code || 'unknown'
    items.push({
      id: `snapshot-error-${currentTask.id}`,
      level: 'error',
      message: `任务错误快照：stage=${stage}, code=${code}, message=${resultError}`,
      time: currentTask.result.last_error_at || currentTask.updated_at,
    })
  }

  const saveError = currentTask.upload?.last_save_error?.trim()
  if (saveError && !items.some((item) => item.message.includes(saveError))) {
    items.push({
      id: `snapshot-save-error-${currentTask.id}`,
      level: 'warn',
      message: `最近保存错误：${saveError}`,
      time: currentTask.updated_at,
    })
  }

  return items.sort((a, b) => new Date(a.time).getTime() - new Date(b.time).getTime())
})

let timer: number | undefined

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
.task-detail-view {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.data-panels {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.detail-card {
  border-radius: 12px;
}

.card-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  font-size: 16px;
}

.info-section-title {
  font-weight: 600;
  color: var(--text-main);
  margin-bottom: 16px;
  font-size: 14px;
}

.info-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
}

.info-grid.three-cols {
  grid-template-columns: repeat(3, 1fr);
}

.info-grid.two-cols {
  grid-template-columns: repeat(2, 1fr);
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.info-item.full-width {
  grid-column: 1 / -1;
}

.info-label {
  font-size: 12px;
  color: var(--text-subtle);
}

.info-value {
  font-size: 14px;
  color: var(--text-main);
  font-weight: 500;
}

.filename-text {
  font-weight: 600;
  color: var(--brand);
}

.font-mono {
  font-family: monospace;
}

.break-all {
  word-break: break-all;
}

.text-muted {
  color: var(--text-muted);
}

.text-danger {
  color: #ef4444;
}

.exec-blocks {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.exec-block {
  background: var(--bg-hover);
  border-radius: 8px;
  padding: 16px;
  border: 1px solid var(--line-soft);
}

.exec-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.exec-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 600;
  color: var(--text-main);
}

.exec-progress {
  margin-bottom: 12px;
}

.progress-labels {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: var(--text-subtle);
  margin-bottom: 4px;
}

.mt-3 {
  margin-top: 12px;
}

.error-banner {
  margin-top: 24px;
  background: rgba(239, 68, 68, 0.1);
  border: 1px solid rgba(239, 68, 68, 0.2);
  border-radius: 8px;
  padding: 16px;
  display: flex;
  gap: 12px;
  color: #ef4444;
}

.error-banner-content strong {
  display: block;
  margin-bottom: 4px;
}

.error-banner-content p {
  margin: 0 0 4px 0;
  font-size: 13px;
}

.error-banner-content .msg {
  font-family: monospace;
  opacity: 0.9;
}

/* Sidebar */
.sticky-sidebar {
  position: sticky;
  top: 80px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.sidebar-card {
  border-radius: 12px;
}

.main-status {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 16px 0 24px;
}

.status-label {
  font-size: 13px;
  color: var(--text-subtle);
}

.action-buttons {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.sidebar-btn {
  width: 100%;
  margin-left: 0 !important;
  border-radius: 8px;
}

.logs-card :deep(.el-card__body) {
  padding: 16px 12px;
}

.logs-container {
  max-height: 500px;
  overflow-y: auto;
  padding-right: 8px;
}

.log-message {
  font-size: 12px;
  color: var(--text-main);
  line-height: 1.4;
  word-break: break-word;
}

/* Custom Scrollbar for logs */
.logs-container::-webkit-scrollbar {
  width: 6px;
}
.logs-container::-webkit-scrollbar-track {
  background: transparent;
}
.logs-container::-webkit-scrollbar-thumb {
  background: var(--line-heavy);
  border-radius: 3px;
}

@media (max-width: 1200px) {
  .info-grid.three-cols {
    grid-template-columns: repeat(2, 1fr);
  }
  .sticky-sidebar {
    position: static;
  }
}

@media (max-width: 768px) {
  .info-grid {
    grid-template-columns: 1fr !important;
  }
  .btn-label { display: none; }
  .header-actions .el-button { padding: 8px 12px; min-width: unset; }
  .exec-block { padding: 12px; }
  .logs-container { max-height: 300px; }
  .task-detail-view { gap: 10px; }
  .error-banner { flex-direction: column; }
}
</style>
