<template>
  <div>
    <PageHeaderCard
      title="扫描结果"
      subtitle="确认匹配结果后，可以直接把扫描项批量创建成任务快照，进入后续任务队列。"
    >
      <el-button @click="router.push('/tasks')">前往任务队列</el-button>
      <el-button :loading="scanStore.loading" @click="scanStore.fetchScans()">刷新</el-button>
    </PageHeaderCard>

    <el-space direction="vertical" fill size="large">
      <el-card v-for="scan in scanStore.scans" :key="scan.id" class="scan-card">
        <template #header>
          <div class="scan-header">
            <div>
              <strong>{{ scan.path }}</strong>
              <p>
                TMDB ID: {{ scan.tmdb_id }} / 类型: {{ scan.video_type || '自动' }} / 共
                {{ scan.total_count }} 项 / 已匹配 {{ scan.matched_count }} 项
              </p>
            </div>

            <div class="scan-actions">
              <span class="selection-text">已选 {{ getSelectedCount(scan.id) }} 项</span>
              <el-button
                type="primary"
                :disabled="getSelectedCount(scan.id) === 0"
                :loading="taskStore.loading"
                @click="createTasks(scan.id)"
              >
                创建任务
              </el-button>
            </div>
          </div>
        </template>

        <div class="table-scroll">
          <el-table
            :ref="bindTableRef(scan.id)"
            :data="scan.items"
            row-key="id"
            stripe
            @selection-change="onSelectionChange(scan.id)"
          >
            <el-table-column type="expand" width="52">
              <template #default="{ row }">
                <div class="expand-grid">
                  <div class="expand-section">
                    <div class="expand-title">人工修正</div>
                    <div class="edit-grid">
                      <el-input v-model="row.selected_item_type" placeholder="item_type" />
                      <el-input-number v-model="row.selected_item_id" :min="0" />
                      <el-input v-model="row.selected_title" placeholder="title" class="title-input" />
                      <el-switch
                        v-model="row.confirmed"
                        inline-prompt
                        active-text="已确认"
                        inactive-text="未确认"
                      />
                    </div>
                  </div>
                  <div class="expand-section">
                    <div class="expand-title">候选</div>
                    <div v-if="row.match_candidates?.length" class="candidate-list">
                      <div
                        v-for="candidate in row.match_candidates"
                        :key="`${candidate.item_type}-${candidate.item_id}`"
                      >
                        {{ candidate.item_type }} / {{ candidate.item_id }} / {{ candidate.title }}
                      </div>
                    </div>
                    <span v-else class="muted-text">无</span>
                  </div>
                </div>
              </template>
            </el-table-column>
          <el-table-column
            type="selection"
            width="54"
            :selectable="(row: ScanItem) => canCreateTask(row)"
          />
          <el-table-column prop="file_name" label="文件名" min-width="220" />
          <el-table-column label="大小" width="120">
            <template #default="{ row }">
              {{ formatSizeInMB(row.file_size) }}
            </template>
          </el-table-column>
          <el-table-column label="解析结果" min-width="180">
            <template #default="{ row }">
              <span>
                S{{ row.parsed.season ?? '-' }} / E{{ row.parsed.episode ?? '-' }}
                <template v-if="row.parsed.is_special"> / 特别篇</template>
              </span>
            </template>
          </el-table-column>
          <el-table-column label="确认状态" width="110">
            <template #default="{ row }">
              <el-tag :type="row.confirmed ? 'success' : 'info'" effect="light" round>
                {{ row.confirmed ? '已确认' : '未确认' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="match_status" label="匹配状态" width="120" />
          <el-table-column label="最终目标" min-width="240">
            <template #default="{ row }">
              <div>{{ row.selected_item_type || '-' }} / {{ row.selected_item_id || '-' }}</div>
              <div class="muted-text">{{ row.selected_title || row.match_reason || '-' }}</div>
            </template>
          </el-table-column>
          <el-table-column label="是否可创建" width="120">
            <template #default="{ row }">
              <el-tag :type="canCreateTask(row) ? 'success' : 'warning'" effect="light" round>
                {{ canCreateTask(row) ? '可创建' : '待补全' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="160" fixed="right">
            <template #default="{ row }">
              <el-button type="primary" link @click="saveItem(scan.id, row.id, row)">保存</el-button>
              <el-button type="success" link @click="createSingleTask(scan.id, row)">加入队列</el-button>
            </template>
          </el-table-column>
          </el-table>
        </div>
      </el-card>
    </el-space>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRouter } from 'vue-router'

import PageHeaderCard from '@/components/PageHeaderCard.vue'
import { useScanStore } from '@/stores/scans'
import { useTaskStore } from '@/stores/tasks'
import type { ScanItem } from '@/types/api'
import { formatSizeInMB } from '@/utils/format'

const router = useRouter()
const scanStore = useScanStore()
const taskStore = useTaskStore()

const selectedItemIdsByScan = reactive<Record<string, string[]>>({})
const tableRefs = reactive<Record<string, { clearSelection?: () => void } | null>>({})

function canCreateTask(row: ScanItem) {
  return Boolean(
    row.confirmed &&
      row.selected_item_type &&
      row.selected_item_id > 0 &&
      row.raw_url &&
      row.is_video,
  )
}

function setTableRef(scanId: string, table: { clearSelection?: () => void } | null) {
  tableRefs[scanId] = table
}

function bindTableRef(scanId: string) {
  return (table: { clearSelection?: () => void } | null) => {
    setTableRef(scanId, table)
  }
}

function handleSelectionChange(scanId: string, rows: ScanItem[]) {
  selectedItemIdsByScan[scanId] = rows.map((row) => row.id)
}

function onSelectionChange(scanId: string) {
  return (rows: ScanItem[]) => {
    handleSelectionChange(scanId, rows)
  }
}

function getSelectedCount(scanId: string) {
  return selectedItemIdsByScan[scanId]?.length ?? 0
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

async function saveItem(scanId: string, itemId: string, row: ScanItem) {
  await persistItem(scanId, itemId, row, true)
}

async function createSingleTask(scanId: string, row: ScanItem) {
  if (!row.confirmed) {
    row.confirmed = true
  }

  if (!canCreateTask(row)) {
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
  const itemIds = selectedItemIdsByScan[scanId] ?? []
  if (!itemIds.length) {
    ElMessage.warning('请先勾选可创建任务的扫描项')
    return
  }

  try {
    const result = await taskStore.batchCreateTasks(scanId, itemIds)
    tableRefs[scanId]?.clearSelection?.()
    selectedItemIdsByScan[scanId] = []

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
.scan-card {
  border-radius: 20px;
}

.scan-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.scan-header p,
.muted-text {
  margin: 6px 0 0;
  color: #6a746f;
}

.scan-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.selection-text {
  color: #6a746f;
  font-size: 13px;
}

.edit-grid {
  display: grid;
  grid-template-columns: 1fr 120px;
  gap: 8px;
  align-items: center;
}

.title-input {
  grid-column: 1 / -1;
}

.candidate-list {
  display: grid;
  gap: 4px;
  color: #42504a;
}

.table-scroll {
  overflow-x: auto;
}

.expand-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
  padding: 8px 4px;
}

.expand-title {
  margin-bottom: 8px;
  font-weight: 600;
  color: #1f2a24;
}

.expand-section .muted-text {
  font-size: 12px;
}

@media (max-width: 960px) {
  .scan-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .scan-actions {
    width: 100%;
    justify-content: space-between;
  }

  .expand-grid {
    grid-template-columns: 1fr;
  }
}
</style>
