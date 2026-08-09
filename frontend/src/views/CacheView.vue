<template>
  <div class="cache-view">
    <PageHeaderCard
      title="缓存管理"
      subtitle="管理下载缓存目录中的本地文件：清理孤儿文件、已删除任务残留与临时分片。"
    >
      <el-button :loading="cacheStore.loading" @click="load" class="tool-btn">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/></svg>
        <span class="btn-label">刷新</span>
      </el-button>
      <el-button type="warning" plain :disabled="!orphanRows.length" class="tool-btn" @click="selectOrphans">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/></svg>
        <span class="btn-label">全选未引用 ({{ orphanRows.length }})</span>
      </el-button>
      <el-button type="danger" :disabled="!selectedPaths.length" :loading="cacheStore.loading" class="tool-btn" @click="deleteSelected">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
        <span class="btn-label">删除选中 ({{ selectedPaths.length }})</span>
      </el-button>
    </PageHeaderCard>

    <el-alert
      v-if="cacheStore.result"
      class="runtime-alert"
      :title="`缓存目录：${cacheStore.result.dir}`"
      type="info"
      :closable="false"
      show-icon
    >
      <template #default>
        <span>
          共 {{ cacheStore.result.entries.length }} 个文件 · {{ formatBytes(cacheStore.result.total_size) }} ·
          未引用 {{ cacheStore.result.orphan_count }} 个
          <template v-if="cacheStore.result.active_ref_count">· 使用中 {{ cacheStore.result.active_ref_count }} 个</template>
        </span>
      </template>
    </el-alert>

    <el-card class="queue-card" :body-style="{ padding: '0' }">
      <el-table
        ref="tableRef"
        :data="cacheStore.result?.entries ?? []"
        row-key="path"
        stripe
        class="task-table"
        @selection-change="onSelectionChange"
      >
        <el-table-column type="selection" width="42" :selectable="(row: CacheEntry) => !isActiveRef(row)" />
        <el-table-column label="文件名" min-width="260" show-overflow-tooltip>
          <template #default="{ row }">
            <div class="task-target">
              <span class="file-name">{{ row.name }}</span>
              <span class="task-meta">
                <el-tag v-if="row.is_temp" type="info" size="small" effect="plain">临时分片</el-tag>
                <el-tag v-if="row.keep_local_file" type="warning" size="small" effect="plain">保留</el-tag>
              </span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="大小" width="120" align="right">
          <template #default="{ row }">{{ formatBytes(row.size) }}</template>
        </el-table-column>
        <el-table-column label="修改时间" width="170">
          <template #default="{ row }">{{ formatTime(row.modified_at) }}</template>
        </el-table-column>
        <el-table-column label="引用状态" width="150">
          <template #default="{ row }">
            <el-tag v-if="isActiveRef(row)" type="danger" size="small" effect="plain">使用中</el-tag>
            <el-tag v-else-if="row.referenced" type="primary" size="small" effect="plain">任务引用</el-tag>
            <el-tag v-else type="info" size="small" effect="plain">未引用</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="关联任务" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">
            <template v-if="row.referenced">
              <div class="task-meta">
                <span class="task-id">{{ row.task_id }}</span>
                <span class="task-status">{{ statusLabel(row.task_status) }}</span>
              </div>
              <div class="task-ep">{{ row.task_file_name }}</div>
            </template>
            <span v-else class="muted-text">-</span>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!cacheStore.result?.entries.length" description="缓存目录为空" style="padding: 48px 0" />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { TableInstance } from 'element-plus'

import PageHeaderCard from '@/components/PageHeaderCard.vue'
import { useCacheStore } from '@/stores/cache'
import type { CacheEntry } from '@/types/api'
import { formatBytes, formatTime } from '@/utils/format'

const cacheStore = useCacheStore()

const tableRef = ref<TableInstance | null>(null)
const selectedRows = ref<CacheEntry[]>([])

const selectedPaths = computed(() => selectedRows.value.map((r) => r.path))
const orphanRows = computed(() =>
  (cacheStore.result?.entries ?? []).filter((r) => !r.referenced && !isActiveRef(r)),
)

function isActiveRef(row: CacheEntry) {
  return row.referenced && ['downloading', 'uploading', 'saving'].includes(row.task_status ?? '')
}

function statusLabel(status?: string) {
  const labels: Record<string, string> = {
    queued: '排队中',
    downloading: '下载中',
    download_completed: '下载完成',
    upload_pending: '待上传',
    uploading: '上传中',
    saving: '保存中',
    completed: '已完成',
    download_failed: '下载失败',
    upload_failed: '上传失败',
    canceled: '已取消',
  }
  return labels[status ?? ''] ?? status ?? '-'
}

function onSelectionChange(rows: CacheEntry[]) {
  selectedRows.value = rows
}

function selectOrphans() {
  const table = tableRef.value
  if (!table) return
  nextTick(() => {
    for (const row of orphanRows.value) {
      table.toggleRowSelection(row, true)
    }
  })
}

async function deleteSelected() {
  const rows = selectedRows.value.filter((r) => !isActiveRef(r))
  if (!rows.length) {
    ElMessage.warning('请先勾选要删除的文件（使用中的文件不可删除）')
    return
  }
  const size = rows.reduce((sum, r) => sum + r.size, 0)
  try {
    await ElMessageBox.confirm(
      `确定删除选中的 ${rows.length} 个文件（${formatBytes(size)}）？\n未引用的为缓存残留，删除后不可恢复。`,
      '删除缓存文件',
      {
        confirmButtonText: '删除',
        cancelButtonText: '取消',
        type: 'warning',
      },
    )
  } catch {
    return
  }

  const { deleted, failed } = await cacheStore.deletePaths(rows.map((r) => r.path))
  if (deleted.length) {
    ElMessage.success(`已删除 ${deleted.length} 个文件`)
  }
  if (Object.keys(failed).length) {
    ElMessage.error(`删除失败 ${Object.keys(failed).length} 个：${Object.values(failed)[0]}`)
  }
  selectedRows.value = []
  await load()
}

async function load() {
  try {
    await cacheStore.fetchCache()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载缓存失败')
  }
}

onMounted(load)
</script>

<style scoped>
.tool-btn {
  display: flex;
  align-items: center;
  gap: 4px;
}
.btn-label {
  margin-left: 6px;
}
.task-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 2px;
}
.task-id {
  font-size: 12px;
  color: var(--text-subtle, #909399);
}
.task-status {
  font-size: 12px;
  color: var(--el-color-primary);
}
.task-ep {
  font-size: 12px;
  color: var(--text-subtle, #909399);
}
</style>
