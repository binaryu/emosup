<template>
  <el-dialog
    v-model="visible"
    :title="isBatch ? `批量手动上传 (${batchRows.length} 个文件)` : '手动指定 ID 上传'"
    :width="isBatch ? '820px' : '560px'"
    destroy-on-close
    append-to-body
    class="manual-upload-dialog"
    @opened="onDialogOpened"
  >
    <div class="dialog-inner">
      <!-- 批量模式快速填充面板 -->
      <div v-if="isBatch" class="batch-quick-panel">
        <div class="quick-panel-header">
          <div class="quick-title">
            <span class="dot-indicator"></span>
            <span>批量序列填充</span>
          </div>
          <span class="quick-tip">设置起始 ID 与步长，按顺序快速填充下方所有文件</span>
        </div>

        <div class="quick-form-row">
          <div class="quick-field">
            <span class="field-label">类型</span>
            <el-radio-group v-model="batchDefaultType" size="default" @change="applyBatchType">
              <el-radio-button value="ve">ve (分集)</el-radio-button>
              <el-radio-button value="vl">vl (单片)</el-radio-button>
            </el-radio-group>
          </div>

          <div class="quick-field">
            <span class="field-label">起始 ID</span>
            <el-input
              v-model="batchStartRaw"
              size="default"
              placeholder="如 ve1001 或 1001"
              style="width: 140px"
              @keyup.enter="applyBatchSequence"
            />
          </div>

          <div class="quick-field">
            <span class="field-label">步长</span>
            <el-input-number
              v-model="batchStep"
              :min="0"
              :max="10"
              size="default"
              controls-position="right"
              style="width: 85px"
            />
          </div>

          <el-button type="primary" size="default" class="quick-apply-btn" @click="applyBatchSequence">
            一键填充
          </el-button>
        </div>
      </div>

      <!-- 单文件模式卡片 -->
      <div v-if="!isBatch && singleItem" class="single-panel">
        <!-- 待上传文件简报 -->
        <div class="file-card">
          <div class="file-avatar">🎬</div>
          <div class="file-details">
            <div class="file-primary-name" :title="singleItem.name">{{ singleItem.name }}</div>
            <div class="file-secondary-meta">
              <span class="meta-tag size">{{ formatSizeInMB(singleItem.size) }}</span>
              <span class="meta-tag source">{{ sourceLabel }}</span>
            </div>
          </div>
        </div>

        <!-- 核心输入区域 -->
        <div class="input-card">
          <div class="input-card-title">EMOS 目标配置</div>

          <div class="form-row">
            <label class="form-label">条目类型与 ID</label>
            <div class="combo-input-wrapper">
              <el-select
                v-model="singleType"
                class="type-select"
                size="large"
                @change="onSingleTypeChange"
              >
                <el-option label="ve · 剧集分集" value="ve" />
                <el-option label="vl · 电影单片" value="vl" />
              </el-select>
              <el-input
                ref="singleInputRef"
                v-model="singleRawInput"
                size="large"
                placeholder="直接输入 ID（如 ve1829946 或 1024），回车直接上传"
                clearable
                class="id-input"
                @input="onSingleInput"
                @blur="verifySingleItem"
                @keyup.enter="handleSingleEnter"
              />
              <el-button
                type="primary"
                size="large"
                :loading="submitting"
                :disabled="!canSubmit"
                class="quick-submit-btn"
                @click="submitTasks"
              >
                <el-icon><Upload /></el-icon>
                <span>立即上传</span>
              </el-button>
            </div>
            <div class="field-hint">
              支持直接粘贴 <code>ve1829946</code>、<code>vl-2048</code> 或纯数字 ID；输入 ID 后按回车或直接点击【立即上传】即可入队上传。
            </div>
          </div>

          <!-- EMOS 校验结果回显状态条 -->
          <div v-if="singleParsed.valid" class="echo-strip" :class="echoStatusClass">
            <div v-if="echoLoading" class="echo-state loading">
              <el-icon class="is-loading"><Loading /></el-icon>
              <span>正在获取 EMOS 标题...</span>
            </div>
            <div v-else-if="echoTitle" class="echo-state success">
              <span class="echo-badge">已关联</span>
              <span class="echo-title-text" :title="echoTitle">{{ echoTitle }}</span>
            </div>
            <div v-else-if="echoChecked" class="echo-state notfound">
              <span class="echo-badge warn">未查到</span>
              <span>EMOS 中未查到该 ID，请确认是否已录入（仍可点击上传）</span>
            </div>
          </div>

          <div class="form-row" style="margin-top: 14px">
            <label class="form-label">任务自定义标题 (可选)</label>
            <el-input
              v-model="singleCustomTitle"
              :placeholder="echoTitle || singleItem.name"
              size="default"
              clearable
              @keyup.enter="handleSingleEnter"
            />
          </div>
        </div>
      </div>

      <!-- 批量表格区域 -->
      <div v-else class="batch-table-wrapper">
        <el-table
          :data="batchRows"
          stripe
          max-height="360"
          size="default"
          class="batch-table"
        >
          <el-table-column label="视频文件" min-width="220" show-overflow-tooltip>
            <template #default="{ row }">
              <div class="table-file-cell">
                <span class="file-icon">🎬</span>
                <span class="file-name">{{ row.name }}</span>
              </div>
            </template>
          </el-table-column>

          <el-table-column label="大小" width="90" align="right">
            <template #default="{ row }">
              <span class="table-size">{{ formatSizeInMB(row.size) }}</span>
            </template>
          </el-table-column>

          <el-table-column label="类型" width="105">
            <template #default="{ row }">
              <el-select v-model="row.item_type" size="small" @change="() => onRowTypeChange(row)">
                <el-option label="ve (分集)" value="ve" />
                <el-option label="vl (电影)" value="vl" />
              </el-select>
            </template>
          </el-table-column>

          <el-table-column label="EMOS ID" width="150">
            <template #default="{ row, $index }">
              <el-input
                :ref="(el: any) => setBatchInputRef(el, $index)"
                v-model="row.raw_input"
                size="small"
                placeholder="如 ve1001"
                clearable
                @input="() => onRowInput(row)"
                @blur="() => verifyRowItem(row)"
                @keyup.enter="() => handleBatchRowEnter($index)"
              />
            </template>
          </el-table-column>

          <el-table-column label="EMOS 标题回显" min-width="150" show-overflow-tooltip>
            <template #default="{ row }">
              <div v-if="row.loading" class="row-echo loading">
                <el-icon class="is-loading"><Loading /></el-icon>
                <span>查询中...</span>
              </div>
              <span v-else-if="row.echoTitle" class="row-echo success" :title="row.echoTitle">
                {{ row.echoTitle }}
              </span>
              <span v-else-if="row.item_id > 0 && row.checked" class="row-echo warn">
                未查到
              </span>
              <span v-else class="muted-text">-</span>
            </template>
          </el-table-column>

          <el-table-column label="操作" width="60" align="center">
            <template #default="{ $index }">
              <el-button link type="danger" size="small" @click="removeBatchRow($index)">移除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <!-- 本地文件保留开关 -->
      <div v-if="source === 'local'" class="keep-local-bar">
        <el-checkbox v-model="keepLocalFile">
          <span class="keep-label">上传完成后保留本地文件（不清理源文件）</span>
        </el-checkbox>
      </div>
    </div>

    <template #footer>
      <div class="dialog-foot">
        <div class="foot-info">
          <template v-if="isBatch">
            已填写有效 ID: <strong>{{ validBatchCount }}</strong> / {{ batchRows.length }}
          </template>
          <template v-else-if="singleParsed.valid">
            目标: <strong>{{ singleParsed.item_type }}-{{ singleParsed.item_id }}</strong>
            <span v-if="echoTitle" class="foot-echo-title">({{ echoTitle }})</span>
          </template>
        </div>
        <div class="foot-btns">
          <el-button @click="visible = false">取消</el-button>
          <el-button
            type="primary"
            :loading="submitting"
            :disabled="!canSubmit"
            class="submit-action-btn"
            @click="submitTasks"
          >
            <el-icon style="margin-right: 4px"><Upload /></el-icon>
            {{ isBatch ? `立即批量上传 (${validBatchCount})` : '立即上传' }}
          </el-button>
        </div>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type InputInstance } from 'element-plus'
import { Loading, Upload } from '@element-plus/icons-vue'

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

const singleInputRef = ref<InputInstance>()
const batchInputRefs = ref<Record<number, InputInstance | null>>({})

function setBatchInputRef(el: any, index: number) {
  if (el) {
    batchInputRefs.value[index] = el
  }
}

function onDialogOpened() {
  nextTick(() => {
    if (!isBatch.value) {
      singleInputRef.value?.focus()
    }
  })
}

const sourceLabel = computed(() => {
  if (props.source === 'local') return '本地媒体'
  if (props.source === 'bt') return 'BT 下载'
  return 'OpenList 网盘'
})

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

function handleSingleEnter() {
  if (canSubmit.value) {
    submitTasks()
  } else if (!singleRawInput.value.trim()) {
    ElMessage.warning('请先输入 EMOS ID')
  } else if (!singleParsed.value.valid) {
    ElMessage.warning('请输入有效的 EMOS ID（如 ve1001 或纯数字）')
  }
}

function handleBatchRowEnter(index: number) {
  if (canSubmit.value) {
    submitTasks()
    return
  }
  const nextIndex = index + 1
  if (nextIndex < batchRows.value.length && batchInputRefs.value[nextIndex]) {
    batchInputRefs.value[nextIndex]?.focus()
  }
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

  if (singleTimer) {
    clearTimeout(singleTimer)
    singleTimer = null
  }

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
.dialog-inner {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

/* 顶部批量快速填充面板 */
.batch-quick-panel {
  background: var(--bg-hover);
  border: 1px solid var(--line-soft);
  border-radius: 12px;
  padding: 14px 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.quick-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
}
.quick-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-main);
}
.dot-indicator {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--brand);
}
.quick-tip {
  font-size: 12px;
  color: var(--text-muted);
}
.quick-form-row {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}
.quick-field {
  display: flex;
  align-items: center;
  gap: 8px;
}
.field-label {
  font-size: 13px;
  color: var(--text-subtle);
  font-weight: 500;
}
.quick-apply-btn {
  font-weight: 600;
}

/* 单文件面板 */
.single-panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.file-card {
  display: flex;
  align-items: center;
  gap: 14px;
  background: var(--bg-hover);
  border: 1px solid var(--line-soft);
  border-radius: 12px;
  padding: 14px 16px;
}
.file-avatar {
  font-size: 26px;
  width: 44px;
  height: 44px;
  border-radius: 10px;
  background: var(--brand-soft);
  display: grid;
  place-items: center;
  flex-shrink: 0;
}
.file-details {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.file-primary-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-main);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.file-secondary-meta {
  display: flex;
  align-items: center;
  gap: 8px;
}
.meta-tag {
  font-size: 12px;
  padding: 2px 8px;
  border-radius: 6px;
}
.meta-tag.size {
  background: var(--line-subtle);
  color: var(--text-subtle);
  font-family: monospace;
}
.meta-tag.source {
  background: var(--brand-soft);
  color: var(--brand);
  font-weight: 500;
}

.input-card {
  background: var(--bg-panel);
  border: 1px solid var(--line-soft);
  border-radius: 12px;
  padding: 18px 20px;
  box-shadow: var(--shadow-sm);
}
.input-card-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-main);
  margin-bottom: 14px;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--line-subtle);
}

.form-row {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.form-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-main);
}
.combo-input-wrapper {
  display: flex;
  align-items: center;
  gap: 10px;
}
.type-select {
  width: 140px;
  flex-shrink: 0;
}
.id-input {
  flex: 1;
}
.quick-submit-btn {
  flex-shrink: 0;
  font-weight: 600;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
@media (max-width: 600px) {
  .combo-input-wrapper {
    flex-wrap: wrap;
  }
  .type-select {
    width: 100%;
  }
  .id-input {
    width: 100%;
  }
  .quick-submit-btn {
    width: 100%;
    margin-top: 4px;
  }
}
.field-hint {
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.4;
  margin-top: 4px;
}
.field-hint code {
  background: var(--bg-hover);
  color: var(--brand);
  padding: 1px 5px;
  border-radius: 4px;
  font-family: monospace;
}

/* 回显结果条 */
.echo-strip {
  margin-top: 10px;
  padding: 10px 14px;
  border-radius: 8px;
  font-size: 13px;
  border: 1px solid transparent;
  transition: all 0.2s ease;
}
.echo-strip.status-loading {
  background: rgba(245, 158, 11, 0.08);
  border-color: rgba(245, 158, 11, 0.2);
  color: var(--color-warning);
}
.echo-strip.status-success {
  background: rgba(16, 185, 129, 0.08);
  border-color: rgba(16, 185, 129, 0.2);
  color: var(--color-success);
}
.echo-strip.status-warning {
  background: rgba(245, 158, 11, 0.08);
  border-color: rgba(245, 158, 11, 0.2);
  color: var(--color-warning);
}

.echo-state {
  display: flex;
  align-items: center;
  gap: 8px;
}
.echo-badge {
  font-size: 11px;
  font-weight: 600;
  background: var(--color-success);
  color: #fff;
  padding: 1px 6px;
  border-radius: 4px;
  flex-shrink: 0;
}
.echo-badge.warn {
  background: var(--color-warning);
}
.echo-title-text {
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 批量表格 */
.batch-table-wrapper {
  border: 1px solid var(--line-soft);
  border-radius: 12px;
  overflow: hidden;
}
.batch-table {
  width: 100%;
}
.table-file-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}
.table-file-cell .file-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-main);
}
.table-size {
  font-family: monospace;
  color: var(--text-muted);
  font-size: 12px;
}
.row-echo {
  font-size: 12px;
  display: flex;
  align-items: center;
  gap: 6px;
}
.row-echo.loading {
  color: var(--color-warning);
}
.row-echo.success {
  color: var(--color-success);
  font-weight: 500;
}
.row-echo.warn {
  color: var(--color-warning);
}
.muted-text {
  color: var(--text-muted);
}

.keep-local-bar {
  padding: 10px 14px;
  background: var(--bg-hover);
  border-radius: 8px;
  border: 1px solid var(--line-soft);
}
.keep-label {
  font-size: 13px;
  color: var(--text-main);
}

/* 底部操作 */
.dialog-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}
.foot-info {
  font-size: 13px;
  color: var(--text-muted);
}
.foot-info strong {
  color: var(--text-main);
}
.foot-echo-title {
  color: var(--color-success);
  margin-left: 4px;
  font-weight: 500;
}
.foot-btns {
  display: flex;
  gap: 10px;
}
.submit-action-btn {
  font-weight: 600;
}
</style>
