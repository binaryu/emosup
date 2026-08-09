<template>
  <div class="scan-results-page">
    <PageHeaderCard
      title="扫描结果"
      subtitle="确认匹配结果后，可以直接把扫描项批量创建成任务快照，进入后续任务队列。"
    >
      <el-checkbox v-model="keepLocalFileOnCreate">保留本地文件</el-checkbox>
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
                :type="scan.source === 'openlist' ? 'primary' : 'warning'"
                size="small"
                effect="plain"
                style="margin-left: 10px"
              >
                {{ scan.source === 'openlist' ? 'OpenList' : scan.source === 'bt' ? 'BT 下载' : '本地' }}
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
                type="danger"
                plain
                :disabled="(selectedItemIdsByScan[scan.id] || []).length === 0"
                :loading="scanStore.deleting"
                @click="deleteSelectedItems(scan.id)"
              >
                删除选中
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
                :loading="scanStore.deleting"
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
          <!-- Table View (Desktop) -->
          <el-table
            v-if="viewMode === 'table'"
            :ref="(el: unknown) => setTableRef(scan.id, el)"
            :data="scan.items"
            row-key="id"
            stripe
            @select="(rows: ScanItem[]) => { selectedItemIdsByScan = { ...selectedItemIdsByScan, [scan.id]: rows.map(r => r.id) } }"
            @select-all="(rows: ScanItem[]) => { selectedItemIdsByScan = { ...selectedItemIdsByScan, [scan.id]: rows.map(r => r.id) } }"
            @row-click="(row: ScanItem, _column: unknown, event: MouseEvent) => onRowClick(scan.id, row, event)"
          >
            <el-table-column type="expand" width="48">
              <template #default="{ row }">
                <ScanItemEditor :row="row" />
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

          <!-- Card View (Mobile) -->
          <div v-else class="item-card-list">
            <el-empty v-if="!scan.items.length" description="暂无扫描项" />
            <div
              v-for="row in scan.items"
              :key="row.id"
              class="item-card"
            >
              <div class="item-card-main" @click="toggleExpand(row.id)">
                <el-checkbox
                  class="item-check"
                  :model-value="isItemSelected(scan.id, row.id)"
                  :disabled="!canCreateTask(row, scan.source)"
                  @click.stop
                  @change="toggleItemSelect(scan.id, row)"
                />
                <div class="item-thumb">{{ row.is_video ? '🎬' : '📄' }}</div>
                <div class="item-info">
                  <div class="item-name">{{ row.file_name }}</div>
                  <div class="item-meta">
                    <span>S{{ row.parsed.season ?? '-' }}E{{ row.parsed.episode ?? '-' }}<template v-if="row.parsed.is_special"> · 特别篇</template></span>
                    <span>{{ formatSizeInMB(row.file_size) }}</span>
                    <el-tag v-if="row.has_media === false" type="danger" size="small" effect="plain">空</el-tag>
                    <el-tag v-else-if="row.has_media === true" type="success" size="small" effect="plain">有</el-tag>
                  </div>
                  <div class="item-target">
                    <span v-if="row.selected_title" class="target-title">{{ row.selected_title }}</span>
                    <span v-else class="muted-text">{{ row.match_reason || '未匹配' }}</span>
                  </div>
                </div>
                <div class="item-side">
                  <StatusTag :status="row.match_status" />
                  <svg
                    class="item-chevron"
                    :class="{ open: isExpanded(row.id) }"
                    width="16"
                    height="16"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                  ><polyline points="6 9 12 15 18 9"/></svg>
                </div>
              </div>

              <div v-if="isExpanded(row.id)" class="item-edit-panel">
                <ScanItemEditor :row="row" />
              </div>

              <div class="item-card-actions">
                <button type="button" class="item-action-btn primary" @click="saveItem(scan.id, row.id, row)">
                  保存
                </button>
                <button
                  type="button"
                  class="item-action-btn success"
                  :class="{ disabled: !canCreateTask(row, scan.source) }"
                  :disabled="!canCreateTask(row, scan.source)"
                  @click="createSingleTask(scan.id, row)"
                >
                  入队
                </button>
                <button type="button" class="item-action-btn danger" @click="deleteItem(scan.id, row)">
                  删除
                </button>
              </div>
            </div>
          </div>
        </div>
      </el-card>
    </el-space>
  </div>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { TableInstance } from 'element-plus'
import { useRouter } from 'vue-router'

import PageHeaderCard from '@/components/PageHeaderCard.vue'
import ScanItemEditor from '@/components/ScanItemEditor.vue'
import StatusTag from '@/components/StatusTag.vue'
import { useScanStore } from '@/stores/scans'
import { useTaskStore } from '@/stores/tasks'
import type { ScanItem, ScanSession } from '@/types/api'
import { formatSizeInMB } from '@/utils/format'

const router = useRouter()
const scanStore = useScanStore()
const taskStore = useTaskStore()

const selectedItemIdsByScan = ref<Record<string, string[]>>({})
const lastClickedIndexByScan = ref<Record<string, number>>({})
const viewMode = ref<'table' | 'card'>(typeof window !== 'undefined' && window.innerWidth < 768 ? 'card' : 'table')
const expandedItemIds = ref<Set<string>>(new Set())
const tableRefs = ref<Record<string, TableInstance | null>>({})
const keepLocalFileOnCreate = ref(false)

function syncViewMode() {
  if (typeof window === 'undefined') return
  const next = window.innerWidth < 768 ? 'card' : 'table'
  if (next !== viewMode.value) {
    viewMode.value = next
    if (next === 'table') {
      restoreTableSelection()
    }
  }
}

function setTableRef(scanId: string, el: unknown) {
  tableRefs.value[scanId] = (el as TableInstance | null) || null
}

function tableEl(scanId: string): HTMLElement | null {
  const table = tableRefs.value[scanId]
  if (!table) return null
  return ((table as unknown as { $el: HTMLElement }).$el as HTMLElement) || null
}

function syncTableSelection(scanId: string) {
  const table = tableRefs.value[scanId]
  if (!table) return
  const scan = scanStore.scans.find((s) => s.id === scanId)
  if (!scan) return
  const ids = new Set(selectedItemIdsByScan.value[scanId] ?? [])
  for (const row of scan.items) {
    table.toggleRowSelection(row, ids.has(row.id))
  }
}

function restoreTableSelection() {
  nextTick(() => {
    for (const scan of scanStore.scans) {
      syncTableSelection(scan.id)
    }
  })
}

function onRowClick(scanId: string, row: ScanItem, event: MouseEvent) {
  const target = event.target as HTMLElement
  if (target.closest('.el-checkbox, .el-table__expand-icon, button, a, input, textarea')) {
    return
  }
  const scan = scanStore.scans.find((s) => s.id === scanId)
  if (!scan || !canCreateTask(row, scan.source)) return

  const idx = scan.items.indexOf(row)
  if (idx < 0) return

  if (event.shiftKey && lastClickedIndexByScan.value[scanId] !== undefined) {
    // Shift+click: select the whole range from the last clicked row.
    const [from, to] = [lastClickedIndexByScan.value[scanId], idx].sort((a, b) => a - b)
    const ids = new Set(selectedItemIdsByScan.value[scanId] ?? [])
    for (let i = from; i <= to; i++) {
      const r = scan.items[i]
      if (canCreateTask(r, scan.source)) ids.add(r.id)
    }
    selectedItemIdsByScan.value = { ...selectedItemIdsByScan.value, [scanId]: [...ids] }
    syncTableSelection(scanId)
  } else if (event.ctrlKey || event.metaKey) {
    // Ctrl+click: toggle this row.
    const table = tableRefs.value[scanId]
    table?.toggleRowSelection(row, !isItemSelected(scanId, row.id))
  }
  lastClickedIndexByScan.value = { ...lastClickedIndexByScan.value, [scanId]: idx }
}

// Ctrl+wheel: hold Ctrl and scroll — every row passing under the mouse cursor
// gets selected, for fast bulk selection of long lists.
function onCtrlWheel(event: WheelEvent) {
  if (!event.ctrlKey || viewMode.value !== 'table') return
  const hit = document.elementFromPoint(event.clientX, event.clientY)
  if (!hit) return
  const rowEl = (hit as HTMLElement).closest('tr.el-table__row') as HTMLElement | null
  if (!rowEl) return

  for (const scan of scanStore.scans) {
    const el = tableEl(scan.id)
    if (!el || !el.contains(rowEl)) continue
    const rows = Array.from(el.querySelectorAll('tr.el-table__row')) as HTMLElement[]
    const idx = rows.indexOf(rowEl)
    const row = scan.items[idx]
    if (row && canCreateTask(row, scan.source)) {
      tableRefs.value[scan.id]?.toggleRowSelection(row, true)
    }
    break
  }
}

function isItemSelected(scanId: string, itemId: string) {
  return (selectedItemIdsByScan.value[scanId] ?? []).includes(itemId)
}

function toggleItemSelect(scanId: string, row: ScanItem) {
  const current = new Set(selectedItemIdsByScan.value[scanId] ?? [])
  if (current.has(row.id)) {
    current.delete(row.id)
  } else {
    current.add(row.id)
  }
  selectedItemIdsByScan.value = { ...selectedItemIdsByScan.value, [scanId]: [...current] }
}

function isExpanded(itemId: string) {
  return expandedItemIds.value.has(itemId)
}

function toggleExpand(itemId: string) {
  const next = new Set(expandedItemIds.value)
  if (next.has(itemId)) {
    next.delete(itemId)
  } else {
    next.add(itemId)
  }
  expandedItemIds.value = next
}

function canCreateTask(row: ScanItem, scanSource?: string) {
  const isLocal = scanSource === 'local' || scanSource === 'bt'
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
    const scan = await scanStore.deleteScanItem(scanId, row.id)
    ElMessage.success('已删除')
    handleScanEmptied(scanId, scan)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '删除失败')
  }
}

async function deleteSelectedItems(scanId: string) {
  const itemIds = selectedItemIdsByScan.value[scanId] ?? []
  if (!itemIds.length) {
    ElMessage.warning('请先勾选要删除的扫描项')
    return
  }

  try {
    await ElMessageBox.confirm(
      `确定要删除选中的 ${itemIds.length} 个扫描项吗？此操作不可撤销。`,
      '批量删除扫描项',
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
    const scan = await scanStore.deleteScanItems(scanId, itemIds)
    selectedItemIdsByScan.value = { ...selectedItemIdsByScan.value, [scanId]: [] }
    ElMessage.success(`已删除 ${itemIds.length} 个扫描项`)
    handleScanEmptied(scanId, scan)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '删除失败')
  }
}

function handleScanEmptied(scanId: string, scan: ScanSession) {
  if (scan.total_count !== 0) return
  const exists = scanStore.scans.some((s) => s.id === scanId)
  if (!exists) {
    if (scanStore.scans.length === 0) router.push('/tasks')
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
    const result = await taskStore.batchCreateTasks(scanId, [row.id], keepLocalFileOnCreate.value ? true : undefined)
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
    const result = await taskStore.batchCreateTasks(scanId, itemIds, keepLocalFileOnCreate.value ? true : undefined)
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
  syncViewMode()
  window.addEventListener('resize', syncViewMode)
  window.addEventListener('wheel', onCtrlWheel, { passive: true })
  scanStore.fetchScans()
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', syncViewMode)
  window.removeEventListener('wheel', onCtrlWheel)
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

/* ---- Card View (Mobile) ---- */
.item-card-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.item-card {
  border: 1px solid var(--line-soft);
  border-radius: 12px;
  background: var(--bg-panel);
  overflow: hidden;
}

.item-card-main {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px;
  cursor: pointer;
  -webkit-tap-highlight-color: transparent;
}

.item-check { flex-shrink: 0; }

.item-thumb {
  width: 38px;
  height: 38px;
  display: grid;
  place-items: center;
  font-size: 20px;
  background: var(--bg-hover);
  border-radius: 10px;
  flex-shrink: 0;
}

.item-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.item-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-main);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  word-break: break-all;
}

.item-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  font-size: 12px;
  color: var(--text-muted);
}

.item-target {
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.item-side {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 6px;
  flex-shrink: 0;
}

.item-chevron {
  color: var(--text-muted);
  transition: transform 0.2s ease;
}
.item-chevron.open {
  transform: rotate(180deg);
}

.item-edit-panel {
  padding: 4px 12px 12px;
  border-top: 1px dashed var(--line-soft);
  background: var(--bg-hover);
}
.item-edit-panel :deep(.expand-panel) {
  grid-template-columns: 1fr;
  padding: 8px 0;
}
.item-edit-panel :deep(.edit-grid) {
  grid-template-columns: 1fr;
}
.item-edit-panel :deep(.title-field) {
  grid-column: 1;
}

.item-card-actions {
  display: flex;
  border-top: 1px solid var(--line-soft);
}

.item-action-btn {
  flex: 1;
  padding: 10px 6px;
  border: none;
  background: transparent;
  font-size: 13px;
  font-weight: 600;
  font-family: inherit;
  cursor: pointer;
  transition: background 0.15s;
}
.item-action-btn:not(:last-child) { border-right: 1px solid var(--line-soft); }
.item-action-btn.primary { color: var(--brand); }
.item-action-btn.success { color: var(--el-color-success); }
.item-action-btn.danger { color: #ef4444; }
.item-action-btn:active { background: var(--bg-hover); }
.item-action-btn.disabled { opacity: 0.35; cursor: not-allowed; }

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
}
</style>
