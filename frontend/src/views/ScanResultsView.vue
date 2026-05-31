<template>
  <div class="scan-results-page">
    <PageHeaderCard
      title="扫描结果"
      subtitle="确认匹配结果后，可以直接把扫描项批量创建成任务快照，进入后续任务队列。"
    >
      <el-button @click="router.push('/tasks')">前往任务队列</el-button>
      <el-button :loading="scanStore.loading" @click="scanStore.fetchScans()">刷新</el-button>
    </PageHeaderCard>

    <el-space direction="vertical" fill size="large" style="width: 100%">
      <el-card v-for="scan in scanStore.scans" :key="scan.id" class="scan-card">
        <template #header>
          <div class="scan-header">
            <div>
              <strong>{{ scan.path }}</strong>
              <el-tag
                :type="scan.source === 'local' ? 'warning' : 'primary'"
                size="small"
                effect="plain"
                style="margin-left: 10px"
              >
                {{ scan.source === 'local' ? '本地' : 'OpenList' }}
              </el-tag>
              <p>
                TMDB ID: {{ scan.tmdb_id }} / 类型: {{ scan.video_type || '自动' }} / 共
                {{ scan.total_count }} 项 / 已匹配 {{ scan.matched_count }} 项
              </p>
            </div>

            <div class="scan-actions">
              <span class="selection-text">已选 {{ (selectedItemIdsByScan[scan.id] || []).length }} 项</span>
              <el-button
                type="primary"
                :disabled="(selectedItemIdsByScan[scan.id] || []).length === 0"
                :loading="taskStore.loading"
                @click="createTasks(scan.id)"
              >
                创建任务
              </el-button>
              <el-button
                type="warning"
                plain
                size="small"
                @click="selectEmptyMedia(scan)"
              >
                全选空资源
              </el-button>
              <el-button
                type="danger"
                plain
                size="small"
                @click="deleteScan(scan.id)"
              >
                删除扫描
              </el-button>
              <el-button
                type="primary"
                plain
                size="small"
                :loading="scanStore.loading"
                @click="rescan(scan)"
              >
                重新扫描
              </el-button>
            </div>
          </div>
        </template>

        <div class="table-scroll">
          <el-table
            :data="scan.items"
            row-key="id"
            stripe
            @select="(rows: ScanItem[]) => { selectedItemIdsByScan = { ...selectedItemIdsByScan, [scan.id]: rows.map(r => r.id) } }"
            @select-all="(rows: ScanItem[]) => { selectedItemIdsByScan = { ...selectedItemIdsByScan, [scan.id]: rows.map(r => r.id) } }"
          >
            <el-table-column type="expand" width="48">
              <template #default="{ row }">
                <div class="expand-panel">
                  <div class="expand-section">
                    <div class="expand-title">人工修正</div>
                    <div class="edit-grid">
                      <div class="edit-field">
                        <span class="edit-label">item_id</span>
                        <el-input
                          :model-value="row.selected_item_type && row.selected_item_id ? row.selected_item_type + '-' + row.selected_item_id : ''"
                          placeholder="ve-1829946"
                          size="small"
                          @blur="(e: FocusEvent) => parseItemID(row, (e.target as HTMLInputElement).value)"
                        />
                      </div>
                      <div class="edit-field title-field">
                        <span class="edit-label">title</span>
                        <el-input v-model="row.selected_title" placeholder="目标标题" size="small" />
                      </div>
                      <div class="edit-field">
                        <span class="edit-label">确认</span>
                        <el-switch
                          v-model="row.confirmed"
                          inline-prompt
                          active-text="已确认"
                          inactive-text="未确认"
                          size="small"
                        />
                      </div>
                    </div>
                  </div>
                  <div class="expand-section">
                    <div class="expand-title">匹配候选</div>
                    <div v-if="row.match_candidates?.length" class="candidate-list">
                      <el-tag
                        v-for="candidate in row.match_candidates"
                        :key="`${candidate.item_type}-${candidate.item_id}`"
                        size="small"
                        class="candidate-tag"
                        @click="applyCandidate(row, candidate)"
                      >
                        {{ candidate.item_type }}-{{ candidate.item_id }} {{ candidate.title }}
                      </el-tag>
                    </div>
                    <span v-else class="muted-text">无</span>
                  </div>
                </div>
              </template>
            </el-table-column>
          <el-table-column
            type="selection"
            width="42"
            :selectable="(row: ScanItem) => canCreateTask(row, scan.source)"
          />
          <el-table-column prop="file_name" label="文件名" min-width="200" show-overflow-tooltip />
          <el-table-column label="大小" width="100" align="right">
            <template #default="{ row }">
              {{ formatSizeInMB(row.file_size) }}
            </template>
          </el-table-column>
          <el-table-column label="解析结果" width="150">
            <template #default="{ row }">
              <span class="parse-info">
                S{{ row.parsed.season ?? '-' }}E{{ row.parsed.episode ?? '-' }}
                <template v-if="row.parsed.is_special"> · 特别篇</template>
              </span>
            </template>
          </el-table-column>
          <el-table-column label="匹配" width="90" align="center">
            <template #default="{ row }">
              <StatusTag :status="row.match_status" />
            </template>
          </el-table-column>
          <el-table-column label="资源" width="70" align="center">
            <template #default="{ row }">
              <el-tag v-if="row.has_media === false" type="danger" size="small" effect="plain">空</el-tag>
              <el-tag v-else-if="row.has_media === true" type="success" size="small" effect="plain">有</el-tag>
              <span v-else class="muted-text">-</span>
            </template>
          </el-table-column>
          <el-table-column label="目标" min-width="180" show-overflow-tooltip>
            <template #default="{ row }">
              <span v-if="row.selected_title" class="target-title">{{ row.selected_title }}</span>
              <span v-else class="muted-text">{{ row.match_reason || '未匹配' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="200" fixed="right">
            <template #default="{ row }">
              <el-button type="primary" link size="small" @click="saveItem(scan.id, row.id, row)">保存</el-button>
              <el-button
                type="success"
                link
                size="small"
                :disabled="!canCreateTask(row, scan.source)"
                @click="createSingleTask(scan.id, row)"
              >
                入队
              </el-button>
              <el-button type="danger" link size="small" @click="deleteItem(scan.id, row)">删除</el-button>
            </template>
          </el-table-column>
          </el-table>
        </div>
      </el-card>
    </el-space>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRouter } from 'vue-router'

import PageHeaderCard from '@/components/PageHeaderCard.vue'
import StatusTag from '@/components/StatusTag.vue'
import { useScanStore } from '@/stores/scans'
import { useTaskStore } from '@/stores/tasks'
import type { ScanItem, ScanSession, MatchCandidate } from '@/types/api'
import { formatSizeInMB } from '@/utils/format'

const router = useRouter()
const scanStore = useScanStore()
const taskStore = useTaskStore()

const selectedItemIdsByScan = ref<Record<string, string[]>>({})

function canCreateTask(row: ScanItem, scanSource?: string) {
  const isLocal = scanSource === 'local'
  return Boolean(
    row.confirmed &&
      row.selected_item_type &&
      row.selected_item_id > 0 &&
      row.is_video &&
      (isLocal || row.raw_url),
  )
}

async function rescan(scan: ScanSession) {
  try {
    const created = await scanStore.createScan(scan.path, scan.tmdb_id, scan.video_type, '', scan.source || '')
    if (created) ElMessage.success(`重新扫描完成，${created.total_count} 个视频文件`)
  } catch (e) { ElMessage.error(e instanceof Error ? e.message : '扫描失败') }
}

async function selectEmptyMedia(scan: ScanSession) {
  const ids = scan.items.filter(i => i.has_media === false && canCreateTask(i, scan.source)).map(i => i.id)
  selectedItemIdsByScan.value = { ...selectedItemIdsByScan.value, [scan.id]: ids }
}

function parseItemID(row: ScanItem, value: string) {
  const parts = value.trim().split('-')
  if (parts.length >= 2) {
    row.selected_item_type = parts[0]
    row.selected_item_id = parseInt(parts[1], 10) || 0
    fetchTitle(row)
  }
}

function applyCandidate(row: ScanItem, candidate: MatchCandidate) {
  row.selected_item_type = candidate.item_type
  row.selected_item_id = candidate.item_id
  row.selected_title = candidate.title
}

let fetchTitleTimer: ReturnType<typeof setTimeout> | null = null
async function fetchTitle(row: ScanItem) {
  const itemType = row.selected_item_type?.trim()
  const itemId = row.selected_item_id
  if (!itemType || !itemId || itemId <= 0) return

  if (fetchTitleTimer) clearTimeout(fetchTitleTimer)
  fetchTitleTimer = setTimeout(async () => {
    try {
      const resp = await fetch(
        `/api/emos/video/base?item_type=${encodeURIComponent(itemType)}&item_id=${itemId}`,
      )
      const data = await resp.json()
      if (data.success && data.data?.title) {
        row.selected_title = data.data.title
      }
    } catch {
      // ignore network errors
    }
  }, 300)
}

async function persistItem(scanId: string, itemId: string, row: ScanItem, showToast = true) {
  try {
    await scanStore.updateScanItem(scanId, itemId, {
      selected_item_type: row.selected_item_type,
      selected_item_id: row.selected_item_id,
      selected_title: row.selected_title,
      confirmed: row.confirmed,
    })
    if (showToast) {
      ElMessage.success('扫描项已保存')
    }
    return true
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '保存失败')
    return false
  }
}

async function deleteScan(scanId: string) {
  try {
    await ElMessageBox.confirm(
      '确定要删除整个扫描结果吗？所有扫描项将被移除。此操作不可撤销。',
      '删除扫描',
      {
        confirmButtonText: '删除',
        cancelButtonText: '取消',
        type: 'warning',
      },
    )
  } catch {
    return
  }

  try {
    await scanStore.deleteScan(scanId)
    ElMessage.success('扫描已删除')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '删除失败')
  }
}

async function deleteItem(scanId: string, row: ScanItem) {
  try {
    await ElMessageBox.confirm(
      `确定要删除「${row.file_name}」吗？此操作不可撤销。`,
      '删除扫描项',
      {
        confirmButtonText: '删除',
        cancelButtonText: '取消',
        type: 'warning',
      },
    )
  } catch {
    return
  }

  try {
    await scanStore.deleteScanItem(scanId, row.id)
    ElMessage.success('已删除')
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '删除失败')
  }
}

async function saveItem(scanId: string, itemId: string, row: ScanItem) {
  await persistItem(scanId, itemId, row, true)
}

async function createSingleTask(scanId: string, row: ScanItem) {
  if (!row.confirmed) {
    row.confirmed = true
  }

  const scan = scanStore.scans.find((s) => s.id === scanId)
  if (!canCreateTask(row, scan?.source)) {
    ElMessage.warning('当前扫描项信息未补全，无法创建任务')
    return
  }

  const saved = await persistItem(scanId, row.id, row, false)
  if (!saved) {
    return
  }

  try {
    const result = await taskStore.batchCreateTasks(scanId, [row.id])
    if (result.created.length) {
      ElMessage.success('已加入任务队列')
      // Remove from local scan data
      if (scan) {
        scan.items = scan.items.filter((item) => item.id !== row.id)
        scan.total_count = scan.items.length
        scan.matched_count = scan.items.filter((i) => i.match_status === 'matched').length
        scan.unmatched_count = scan.total_count - scan.matched_count
        if (scan.total_count === 0) {
          scanStore.scans = scanStore.scans.filter((s) => s.id !== scanId)
          if (scanStore.scans.length === 0) router.push('/tasks')
        }
      }
    }
    if (result.failed.length) {
      await ElMessageBox.alert(
        result.failed.map((item) => `${item.item_id}: ${item.reason}`).join('\n'),
        `任务创建失败`,
        {
          confirmButtonText: '知道了',
        },
      )
    }
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '创建任务失败')
  }
}

async function createTasks(scanId: string) {
  const itemIds = selectedItemIdsByScan.value[scanId] ?? []
  if (!itemIds.length) {
    ElMessage.warning('请先勾选可创建任务的扫描项')
    return
  }

  try {
    const result = await taskStore.batchCreateTasks(scanId, itemIds)
    selectedItemIdsByScan.value = { ...selectedItemIdsByScan.value, [scanId]: [] }

    // Remove created items from local scan data
    const scan = scanStore.scans.find((s) => s.id === scanId)
    if (scan) {
      const createdIds = new Set(result.created.map((c) => c.item_id))
      scan.items = scan.items.filter((item) => !createdIds.has(item.id))
      scan.total_count = scan.items.length
      scan.matched_count = scan.items.filter((i) => i.match_status === 'matched').length
      scan.unmatched_count = scan.total_count - scan.matched_count
      // Auto-remove empty scan card
      if (scan.total_count === 0) {
        scanStore.scans = scanStore.scans.filter((s) => s.id !== scanId)
        if (scanStore.scans.length === 0) router.push('/tasks')
      }
    }

    if (result.created.length) {
      ElMessage.success(`已创建 ${result.created.length} 个任务`)
    }

    if (result.failed.length) {
      await ElMessageBox.alert(
        result.failed.map((item) => `${item.item_id}: ${item.reason}`).join('\n'),
        `有 ${result.failed.length} 个扫描项未创建成功`,
        {
          confirmButtonText: '知道了',
        },
      )
    }
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '创建任务失败')
  }
}

onMounted(() => {
  scanStore.fetchScans()
})
</script>

<style scoped>
.scan-results-page {
  width: 100%;
}

.scan-card {
  width: 100%;
}

.scan-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.scan-header p,
.muted-text {
  margin: 6px 0 0;
  color: var(--text-subtle);
  font-size: 13px;
}

.scan-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}

.selection-text {
  color: var(--text-subtle);
  font-size: 13px;
}

.table-scroll {
  overflow-x: auto;
}

/* ---- expand panel ---- */
.expand-panel {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
  padding: 12px 8px;
}

.expand-title {
  margin-bottom: 10px;
  font-weight: 600;
  font-size: 13px;
  color: var(--text-main);
}

.expand-section .muted-text {
  font-size: 12px;
}

.edit-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  align-items: start;
}

.edit-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.edit-label {
  font-size: 12px;
  color: var(--text-muted);
}

.title-field {
  grid-column: 1 / -1;
}

.candidate-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.candidate-tag {
  cursor: pointer;
  transition: opacity 0.15s;
}

.candidate-tag:hover {
  opacity: 0.75;
}

.parse-info {
  white-space: nowrap;
  font-size: 13px;
  font-variant-numeric: tabular-nums;
}

.target-title {
  font-size: 13px;
}

@media (max-width: 960px) {
  .scan-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .scan-actions {
    width: 100%;
    flex-wrap: wrap;
    gap: 8px;
  }

  .scan-actions .el-button {
    font-size: 12px;
    padding: 5px 10px;
  }

  .selection-text {
    width: 100%;
    margin-bottom: 4px;
  }

  .expand-panel {
    grid-template-columns: 1fr;
  }

  .edit-grid {
    grid-template-columns: 1fr;
  }

  .title-field {
    grid-column: 1;
  }
}
</style>
