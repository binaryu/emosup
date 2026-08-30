<template>
  <el-dialog
    v-model="visible"
    title="手动上传 / 直接入队"
    :width="isBatch ? '760px' : '520px'"
    destroy-on-close
    append-to-body
    class="manual-upload-modal"
  >
    <div class="manual-dialog-body">
      <!-- 顶部功能说明横幅 -->
      <div class="header-banner">
        <div class="banner-icon">⚡</div>
        <div class="banner-content">
          <div class="banner-title">免扫描直传模式</div>
          <div class="banner-desc">
            支持直接粘贴 <code>ve1024</code>、<code>vl2048</code> 或纯数字 ID，系统会自动提取并联网校验 EMOS 视频条目。
          </div>
        </div>
      </div>

      <!-- 单文件模式 -->
      <div v-if="!isBatch && singleItem" class="single-mode-wrapper">
        <!-- 文件信息卡片 -->
        <div class="file-summary-card">
          <div class="file-icon">🎬</div>
          <div class="file-meta">
            <div class="file-name" :title="singleItem.name">{{ singleItem.name }}</div>
            <div class="file-tags">
              <span class="file-size-badge">{{ formatSizeInMB(singleItem.size) }}</span>
              <el-tag size="small" effect="plain" :type="source === 'local' ? 'warning' : 'primary'">
                {{ source === 'local' ? '本地媒体' : source === 'bt' ? 'BT 下载' : 'OpenList 网盘' }}
              </el-tag>
            </div>
          </div>
        </div>

        <el-form label-position="top" class="custom-form">
          <!-- 目标类型与输入框 -->
          <el-form-item label="EMOS 条目 ID">
            <div class="id-input-group">
              <el-select
                v-model="singleType"
                style="width: 130px; flex-shrink: 0"
                size="large"
                @change="onSingleTypeChange"
              >
                <el-option label="ve (分集)" value="ve" />
                <el-option label="vl (电影)" value="vl" />
              </el-select>
              <el-input
                v-model="singleRawInput"
                size="large"
                placeholder="例如: ve1024 或 1024"
                clearable
                class="id-input-field"
                @input="onSingleInput"
                @blur="verifySingleItem"
              >
                <template #prefix>
                  <span class="input-prefix-icon">🆔</span>
                </template>
              </el-input>
            </div>
            <div class="form-hint">
              可直接粘贴类似 <code>ve1829946</code>、<code>vl-2048</code> 或纯数字 ID
            </div>

            <!-- 回显信息卡片 -->
            <div v-if="singleParsed.valid" class="echo-result-card" :class="echoStatusClass">
              <div v-if="echoLoading" class="echo-loading">
                <el-icon class="is-loading"><Loading /></el-icon>
                <span>正在查询 EMOS 条目信息...</span>
              </div>
              <div v-else-if="echoTitle" class="echo-success">
                <span class="echo-badge">已匹配</span>
                <span class="echo-title-text" :title="echoTitle">{{ echoTitle }}</span>
              </div>
              <div v-else-if="echoChecked" class="echo-warning">
                <span class="echo-warn-badge">未查到</span>
                <span>未在 EMOS 找到该 ID 对应的条目，请确认 ID 是否正确</span>
              </div>
            </div>
          </el-form-item>

          <!-- 自定义任务标题 -->
          <el-form-item label="任务展示标题 (可选)">
            <el-input
              v-model="singleCustomTitle"
              :placeholder="echoTitle || singleItem.name"
              size="default"
              clearable
            />
          </el-form-item>

          <!-- 本地保留设置 -->
          <div v-if="source === 'local'" class="local-keep-row">
            <el-checkbox v-model="keepLocalFile">
              <span class="checkbox-label">上传完成后保留本地文件（不删除源文件）</span>
            </el-checkbox>
          </div>
        </el-form>
      </div>

      <!-- 多文件批量模式 -->
      <div v-else class="batch-mode-wrapper">
        <!-- 批量填充快捷工具栏 -->
        <div class="batch-smart-bar">
          <div class="smart-bar-title">⚡ 序列填充</div>
          <div class="smart-bar-controls">
            <div class="control-unit">
              <span class="unit-label">类型:</span>
              <el-radio-group v-model="batchDefaultType" size="small" @change="applyBatchType">
                <el-radio-button value="ve">ve (分集)</el-radio-button>
                <el-radio-button value="vl">vl (单片)</el-radio-button>
              </el-radio-group>
            </div>

            <div class="control-unit">
              <span class="unit-label">起始 ID:</span>
              <el-input
                v-model="batchStartRaw"
                size="small"
                placeholder="如 ve1001"
                style="width: 110px"
                @keyup.enter="applyBatchSequence"
              />
            </div>

            <div class="control-unit">
              <span class="unit-label">步长:</span>
              <el-input-number
                v-model="batchStep"
                :min="0"
                :max="10"
                size="small"
                controls-position="right"
                style="width: 75px"
              />
            </div>

            <el-button type="primary" size="small" plain @click="applyBatchSequence">
              一键填充
            </el-button>
          </div>
        </div>

        <!-- 批量表格 -->
        <div class="batch-table-container">
          <el-table
            :data="batchRows"
            stripe
            max-height="340"
            size="small"
            class="custom-batch-table"
          >
            <el-table-column label="待传文件" min-width="200" show-overflow-tooltip>
              <template #default="{ row }">
                <span class="table-file-name">🎬 {{ row.name }}</span>
              </template>
            </el-table-column>
            <el-table-column label="大小" width="85" align="right">
              <template #default="{ row }">
                <span class="table-file-size">{{ formatSizeInMB(row.size) }}</span>
              </template>
            </el-table-column>
            <el-table-column label="类型" width="95">
              <template #default="{ row }">
                <el-select v-model="row.item_type" size="small" @change="() => onRowTypeChange(row)">
                  <el-option label="ve" value="ve" />
                  <el-option label="vl" value="vl" />
                </el-select>
              </template>
            </el-table-column>
            <el-table-column label="EMOS ID (可输 ve/vl/数字)" width="160">
              <template #default="{ row }">
                <el-input
                  v-model="row.raw_input"
                  size="small"
                  placeholder="如: ve1001"
                  clearable
                  @input="() => onRowInput(row)"
                  @blur="() => verifyRowItem(row)"
                />
              </template>
            </el-table-column>
            <el-table-column label="EMOS 回显" min-width="140" show-overflow-tooltip>
              <template #default="{ row }">
                <div v-if="row.loading" class="row-echo loading">
                  <el-icon class="is-loading"><Loading /></el-icon>
                  <span>查询中...</span>
                </div>
                <span v-else-if="row.echoTitle" class="row-echo success" :title="row.echoTitle">
                  ✓ {{ row.echoTitle }}
                </span>
                <span v-else-if="row.item_id > 0 && row.checked" class="row-echo warn">
                  ? 未查到
                </span>
                <span v-else class="muted-text">-</span>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="55" align="center">
              <template #default="{ $index }">
                <el-button link type="danger" size="small" @click="removeBatchRow($index)">移除</el-button>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <div v-if="source === 'local'" class="local-keep-row" style="margin-top: 12px">
          <el-checkbox v-model="keepLocalFile">
            <span class="checkbox-label">上传完成后保留本地文件（不删除源文件）</span>
          </el-checkbox>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="dialog-actions">
        <div class="summary-info">
          <span v-if="isBatch">已选 {{ batchRows.length }} 个文件，准备入队</span>
          <span v-else-if="singleParsed.valid && echoTitle">目标：{{ singleParsed.item_type }}-{{ singleParsed.item_id }} · {{ echoTitle }}</span>
        </div>
        <div class="button-group">
          <el-button @click="visible = false">取消</el-button>
          <el-button
            type="primary"
            :loading="submitting"
            :disabled="!canSubmit"
            class="submit-btn"
            @click="submitTasks"
          >
            立即创建入队 {{ isBatch ? `(${validBatchCount})` : '' }}
          </el-button>
        </div>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Loading } from '@element-plus/icons-vue'

import { useTaskStore } from '@/stores/tasks'
import type { CreateManualTaskItemPayload, OpenListEntry } from '@/types/api'
import { apiGet } from '@/utils/api'
import { formatSizeInMB } from '@/utils/format'

const props = defineProps<{
  source: 'openlist' | 'local' | 'bt'
}>()

const router = useRouter()
const taskStore = useTaskStore()

const visible = ref(false)
const submitting = ref(false)
const keepLocalFile = ref(false)

// ----------------------------------------------------
// 智能解析函数：支持 ve12345, ve-12345, vl12345, 12345 等
// ----------------------------------------------------
function parseEmosId(raw: string, fallbackType: 've' | 'vl' = 've'): { item_type: 've' | 'vl'; item_id: number; valid: boolean } {
  if (!raw) return { item_type: fallbackType, item_id: 0, valid: false }
  const text = raw.trim()

  // 匹配以 ve 或 vl 开头（支持 ve1024, ve-1024, ve_1024, VL2048 等）
  const prefixMatch = text.match(/(ve|vl)[-_/:\s]*(\d+)/i)
  if (prefixMatch) {
    const type = prefixMatch[1].toLowerCase() as 've' | 'vl'
    const id = parseInt(prefixMatch[2], 10)
    return { item_type: type, item_id: id, valid: id > 0 }
  }

  // 匹配纯数字提取
  const digitMatch = text.match(/(\d+)/)
  if (digitMatch) {
    const id = parseInt(digitMatch[1], 10)
    return { item_type: fallbackType, item_id: id, valid: id > 0 }
  }

  return { item_type: fallbackType, item_id: 0, valid: false }
}

// ----------------------------------------------------
// 单文件状态
// ----------------------------------------------------
const singleItem = ref<OpenListEntry | null>(null)
const singleType = ref<'ve' | 'vl'>('ve')
const singleRawInput = ref('')
const singleCustomTitle = ref('')
const echoTitle = ref('')
const echoLoading = ref(false)
const echoChecked = ref(false)

const singleParsed = computed(() => parseEmosId(singleRawInput.value, singleType.value))

const echoStatusClass = computed(() => {
  if (echoLoading.value) return 'status-loading'
  if (echoTitle.value) return 'status-success'
  if (echoChecked.value) return 'status-warning'
  return ''
})

// ----------------------------------------------------
// 多文件批量状态
// ----------------------------------------------------
interface BatchRowItem extends OpenListEntry {
  raw_input: string
  item_type: 've' | 'vl'
  item_id: number
  custom_title?: string
  echoTitle?: string
  loading?: boolean
  checked?: boolean
}
const batchRows = ref<BatchRowItem[]>([])
const batchDefaultType = ref<'ve' | 'vl'>('ve')
const batchStartRaw = ref('')
const batchStep = ref(1)

const isBatch = computed(() => batchRows.value.length > 1 || (batchRows.value.length === 1 && !singleItem.value))

const validBatchCount = computed(() => {
  return batchRows.value.filter((r) => r.item_id > 0).length
})

const canSubmit = computed(() => {
  if (submitting.value) return false
  if (!isBatch.value) {
    return Boolean(singleItem.value && singleParsed.value.valid && singleParsed.value.item_id > 0)
  }
  return batchRows.value.length > 0 && batchRows.value.every((r) => r.item_id > 0)
})

// ----------------------------------------------------
// 打开弹窗入口
// ----------------------------------------------------
function open(items: OpenListEntry[]) {
  if (!items.length) return
  keepLocalFile.value = false

  if (items.length === 1) {
    singleItem.value = items[0]
    batchRows.value = []
    singleType.value = 've'
    singleRawInput.value = ''
    singleCustomTitle.value = ''
    echoTitle.value = ''
    echoLoading.value = false
    echoChecked.value = false
  } else {
    singleItem.value = null
    batchDefaultType.value = 've'
    batchStartRaw.value = ''
    batchStep.value = 1
    batchRows.value = items.map((item) => ({
      ...item,
      raw_input: '',
      item_type: 've',
      item_id: 0,
      echoTitle: '',
      loading: false,
      checked: false,
    }))
  }

  visible.value = true
}

// ----------------------------------------------------
// 单文件事件
// ----------------------------------------------------
function onSingleInput(val: string) {
  const parsed = parseEmosId(val, singleType.value)
  if (parsed.item_type !== singleType.value) {
    singleType.value = parsed.item_type
  }
  if (parsed.valid) {
    verifySingleItem()
  } else {
    echoTitle.value = ''
    echoChecked.value = false
  }
}

function onSingleTypeChange() {
  if (singleParsed.value.valid) {
    verifySingleItem()
  }
}

let singleTimer: ReturnType<typeof setTimeout> | null = null
async function verifySingleItem() {
  const parsed = singleParsed.value
  if (!parsed.valid || parsed.item_id <= 0) return

  if (singleTimer) clearTimeout(singleTimer)
  singleTimer = setTimeout(async () => {
    echoLoading.value = true
    echoChecked.value = false
    try {
      const res = await apiGet<{ title: string }>(
        `/api/emos/video/base?item_type=${parsed.item_type}&item_id=${parsed.item_id}`,
      )
      echoTitle.value = res.title || ''
    } catch {
      echoTitle.value = ''
    } finally {
      echoLoading.value = false
      echoChecked.value = true
    }
  }, 250)
}

// ----------------------------------------------------
// 批量行事件
// ----------------------------------------------------
function onRowInput(row: BatchRowItem) {
  const parsed = parseEmosId(row.raw_input, row.item_type)
  row.item_type = parsed.item_type
  row.item_id = parsed.item_id
  if (parsed.valid) {
    verifyRowItem(row)
  } else {
    row.echoTitle = ''
    row.checked = false
  }
}

function onRowTypeChange(row: BatchRowItem) {
  if (row.item_id > 0) {
    verifyRowItem(row)
  }
}

async function verifyRowItem(row: BatchRowItem) {
  if (!row.item_id || row.item_id <= 0) return
  row.loading = true
  row.checked = false
  try {
    const res = await apiGet<{ title: string }>(
      `/api/emos/video/base?item_type=${row.item_type}&item_id=${row.item_id}`,
    )
    row.echoTitle = res.title || ''
  } catch {
    row.echoTitle = ''
  } finally {
    row.loading = false
    row.checked = true
  }
}

function applyBatchType() {
  for (const row of batchRows.value) {
    row.item_type = batchDefaultType.value
    if (row.item_id > 0) {
      verifyRowItem(row)
    }
  }
}

function applyBatchSequence() {
  const parsed = parseEmosId(batchStartRaw.value, batchDefaultType.value)
  if (!parsed.valid || parsed.item_id <= 0) {
    ElMessage.warning('请先在「起始 ID」输入有效的 ID（如 ve1001 或 1001）')
    return
  }

  let currentId = parsed.item_id
  const targetType = parsed.item_type
  batchDefaultType.value = targetType
  const step = batchStep.value ?? 1

  for (const row of batchRows.value) {
    row.item_type = targetType
    row.item_id = currentId
    row.raw_input = `${targetType}${currentId}`
    verifyRowItem(row)
    currentId += step
  }
  ElMessage.success(`已为 ${batchRows.value.length} 个文件填充序列 ID`)
}

function removeBatchRow(index: number) {
  batchRows.value.splice(index, 1)
  if (batchRows.value.length === 0) {
    visible.value = false
  }
}

// ----------------------------------------------------
// 提交创建入队
// ----------------------------------------------------
async function submitTasks() {
  if (!canSubmit.value) return

  const payloadItems: CreateManualTaskItemPayload[] = []
  if (!isBatch.value && singleItem.value) {
    const parsed = singleParsed.value
    payloadItems.push({
      path: singleItem.value.path,
      file_name: singleItem.value.name,
      file_size: singleItem.value.size,
      item_type: parsed.item_type,
      item_id: parsed.item_id,
      item_title: singleCustomTitle.value.trim() || echoTitle.value || singleItem.value.name,
    })
  } else {
    for (const row of batchRows.value) {
      payloadItems.push({
        path: row.path,
        file_name: row.name,
        file_size: row.size,
        item_type: row.item_type,
        item_id: row.item_id,
        item_title: row.custom_title?.trim() || row.echoTitle || row.name,
      })
    }
  }

  submitting.value = true
  try {
    const res = await taskStore.createManualTasks({
      source: props.source,
      items: payloadItems,
      keep_local_file: props.source === 'local' ? keepLocalFile.value : false,
    })

    if (res.created.length > 0) {
      ElMessage.success(`成功创建 ${res.created.length} 个任务，已进入任务队列`)
      visible.value = false
      router.push('/tasks')
    }
    if (res.failed.length > 0) {
      ElMessage.warning(`有 ${res.failed.length} 个任务创建失败: ${res.failed.map((f) => f.reason).join('; ')}`)
    }
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '创建任务失败')
  } finally {
    submitting.value = false
  }
}

defineExpose({
  open,
})
</script>

<style scoped>
.manual-dialog-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

/* 顶部横幅 */
.header-banner {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  background: var(--brand-soft);
  border: 1px solid var(--brand-border);
  padding: 12px 14px;
  border-radius: 12px;
}
.banner-icon {
  font-size: 20px;
  line-height: 1;
}
.banner-content {
  flex: 1;
}
.banner-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--brand);
  margin-bottom: 3px;
}
.banner-desc {
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.5;
}
.banner-desc code {
  background: var(--brand-soft);
  color: var(--brand);
  padding: 1px 5px;
  border-radius: 4px;
  font-family: monospace;
}

/* 文件概览卡片 */
.file-summary-card {
  display: flex;
  align-items: center;
  gap: 12px;
  background: var(--bg-hover);
  border: 1px solid var(--line-soft);
  border-radius: 12px;
  padding: 12px 14px;
}
.file-icon {
  font-size: 24px;
  flex-shrink: 0;
}
.file-meta {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.file-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-main);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.file-tags {
  display: flex;
  align-items: center;
  gap: 8px;
}
.file-size-badge {
  font-size: 12px;
  color: var(--text-muted);
  font-family: monospace;
}

/* 表单与输入框 */
.custom-form {
  margin-top: 4px;
}
.id-input-group {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
}
.id-input-field {
  flex: 1;
}
.input-prefix-icon {
  font-size: 14px;
  margin-right: 4px;
}
.form-hint {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 5px;
}
.form-hint code {
  background: var(--bg-hover);
  padding: 1px 4px;
  border-radius: 4px;
  color: var(--text-main);
}

/* 回显结果卡片 */
.echo-result-card {
  margin-top: 10px;
  padding: 10px 14px;
  border-radius: 10px;
  font-size: 13px;
  transition: all 0.2s ease;
  background: var(--bg-hover);
  border: 1px solid var(--line-soft);
}
.echo-result-card.status-loading {
  color: var(--color-warning);
  background: rgba(245, 158, 11, 0.1);
  border-color: rgba(245, 158, 11, 0.3);
}
.echo-result-card.status-success {
  color: var(--color-success);
  background: rgba(16, 185, 129, 0.1);
  border-color: rgba(16, 185, 129, 0.3);
}
.echo-result-card.status-warning {
  color: var(--color-warning);
  background: rgba(245, 158, 11, 0.1);
  border-color: rgba(245, 158, 11, 0.3);
}
.echo-loading {
  display: flex;
  align-items: center;
  gap: 8px;
}
.echo-success {
  display: flex;
  align-items: center;
  gap: 10px;
}
.echo-badge {
  background: var(--color-success);
  color: #fff;
  font-size: 11px;
  font-weight: 600;
  padding: 2px 6px;
  border-radius: 6px;
  flex-shrink: 0;
}
.echo-title-text {
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.echo-warning {
  display: flex;
  align-items: center;
  gap: 8px;
}
.echo-warn-badge {
  background: var(--color-warning);
  color: #fff;
  font-size: 11px;
  font-weight: 600;
  padding: 2px 6px;
  border-radius: 6px;
  flex-shrink: 0;
}

.local-keep-row {
  margin-top: 4px;
  padding: 8px 12px;
  background: var(--bg-hover);
  border-radius: 8px;
}
.checkbox-label {
  font-size: 13px;
  color: var(--text-main);
}

/* 批量模式工具栏 */
.batch-smart-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  background: var(--bg-hover);
  border: 1px solid var(--line-soft);
  padding: 10px 14px;
  border-radius: 12px;
}
.smart-bar-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--brand);
}
.smart-bar-controls {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
.control-unit {
  display: flex;
  align-items: center;
  gap: 6px;
}
.unit-label {
  font-size: 12px;
  color: var(--text-muted);
  flex-shrink: 0;
}

/* 批量表格 */
.batch-table-container {
  border: 1px solid var(--line-soft);
  border-radius: 12px;
  overflow: hidden;
}
.table-file-name {
  font-weight: 500;
  color: var(--text-main);
}
.table-file-size {
  font-family: monospace;
  color: var(--text-muted);
}
.row-echo {
  font-size: 12px;
  font-weight: 500;
  display: flex;
  align-items: center;
  gap: 4px;
}
.row-echo.loading {
  color: var(--color-warning);
}
.row-echo.success {
  color: var(--color-success);
}
.row-echo.warn {
  color: var(--color-warning);
}
.muted-text {
  color: var(--text-muted);
}

/* 底部操作 */
.dialog-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}
.summary-info {
  font-size: 12px;
  color: var(--text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 50%;
}
.button-group {
  display: flex;
  gap: 10px;
}
.submit-btn {
  font-weight: 600;
}
</style>
