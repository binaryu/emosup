<template>
  <el-dialog
    v-model="visible"
    title="手动上传 / 直接入队"
    :width="isBatch ? '720px' : '520px'"
    destroy-on-close
    append-to-body
  >
    <div class="manual-upload-dialog">
      <el-alert
        title="跳过 TMDB 与剧集扫描，直接指定 EMOS 的 ve (分集) 或 vl (单片) ID 并创建上传任务。"
        type="info"
        :closable="false"
        show-icon
        style="margin-bottom: 16px"
      />

      <!-- 单文件模式 -->
      <div v-if="!isBatch && singleItem">
        <el-form label-position="top">
          <el-form-item label="待上传文件">
            <div class="file-preview-box">
              <span class="file-name">{{ singleItem.name }}</span>
              <span class="file-size">{{ formatSizeInMB(singleItem.size) }}</span>
            </div>
          </el-form-item>

          <el-form-item label="目标类型">
            <el-radio-group v-model="singleForm.item_type" @change="fetchSingleBaseTitle">
              <el-radio-button value="ve">ve (剧集分集)</el-radio-button>
              <el-radio-button value="vl">vl (电影 / 独立视频)</el-radio-button>
            </el-radio-group>
          </el-form-item>

          <el-form-item label="EMOS Item ID">
            <el-input-number
              v-model="singleForm.item_id"
              :min="1"
              :step="1"
              style="width: 100%"
              placeholder="请输入 EMOS 对应的 Item ID"
              controls-position="right"
              @change="fetchSingleBaseTitle"
            />
            <div v-if="singleEchoTitle" class="echo-title success">
              <span>回显标题：</span>
              <strong>{{ singleEchoTitle }}</strong>
            </div>
            <div v-else-if="singleEchoLoading" class="echo-title loading">
              <span>正在获取 EMOS 标题...</span>
            </div>
          </el-form-item>

          <el-form-item label="自定义任务标题 (可选)">
            <el-input
              v-model="singleForm.item_title"
              :placeholder="singleEchoTitle || singleItem.name"
              clearable
            />
          </el-form-item>

          <el-form-item v-if="source === 'local'">
            <el-checkbox v-model="keepLocalFile">上传完成后保留本地文件</el-checkbox>
          </el-form-item>
        </el-form>
      </div>

      <!-- 多文件批量模式 -->
      <div v-else>
        <div class="batch-toolbar">
          <div class="batch-setting-item">
            <span class="label">类型:</span>
            <el-radio-group v-model="batchItemType" size="small" @change="applyBatchType">
              <el-radio-button value="ve">ve (分集)</el-radio-button>
              <el-radio-button value="vl">vl (电影)</el-radio-button>
            </el-radio-group>
          </div>

          <div class="batch-setting-item">
            <span class="label">起始 ID:</span>
            <el-input-number
              v-model="batchStartId"
              :min="1"
              size="small"
              placeholder="如 1001"
              controls-position="right"
              style="width: 130px"
            />
          </div>

          <div class="batch-setting-item">
            <span class="label">递增:</span>
            <el-input-number
              v-model="batchStep"
              :min="0"
              :max="10"
              size="small"
              controls-position="right"
              style="width: 90px"
            />
          </div>

          <el-button type="primary" size="small" @click="applyBatchSeq">
            快速填充
          </el-button>
        </div>

        <el-table :data="batchItems" stripe max-height="360" size="small" style="width: 100%">
          <el-table-column label="文件名" min-width="180" show-overflow-tooltip>
            <template #default="{ row }">
              🎬 {{ row.name }}
            </template>
          </el-table-column>
          <el-table-column label="大小" width="80" align="right">
            <template #default="{ row }">
              {{ formatSizeInMB(row.size) }}
            </template>
          </el-table-column>
          <el-table-column label="类型" width="90">
            <template #default="{ row }">
              <el-select v-model="row.item_type" size="small" style="width: 75px">
                <el-option label="ve" value="ve" />
                <el-option label="vl" value="vl" />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column label="Item ID" width="130">
            <template #default="{ row }">
              <el-input-number
                v-model="row.item_id"
                :min="1"
                size="small"
                controls-position="right"
                style="width: 110px"
                @change="() => fetchRowEchoTitle(row)"
              />
            </template>
          </el-table-column>
          <el-table-column label="回显 / 标题" min-width="130" show-overflow-tooltip>
            <template #default="{ row }">
              <span v-if="row.echoTitle" class="echo-text">{{ row.echoTitle }}</span>
              <span v-else class="muted-text">-</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="55" align="center">
            <template #default="{ $index }">
              <el-button link type="danger" size="small" @click="removeBatchRow($index)">移除</el-button>
            </template>
          </el-table-column>
        </el-table>

        <div v-if="source === 'local'" style="margin-top: 12px">
          <el-checkbox v-model="keepLocalFile">上传完成后保留本地文件</el-checkbox>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="dialog-footer">
        <el-button @click="visible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="submitting"
          :disabled="!canSubmit"
          @click="submitTasks"
        >
          立即创建入队
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'

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

// Data state
const singleItem = ref<OpenListEntry | null>(null)
const singleForm = ref<{
  item_type: 've' | 'vl'
  item_id: number | undefined
  item_title: string
}>({
  item_type: 've',
  item_id: undefined,
  item_title: '',
})
const singleEchoTitle = ref('')
const singleEchoLoading = ref(false)

interface BatchRowItem extends OpenListEntry {
  item_type: 've' | 'vl'
  item_id: number | undefined
  item_title?: string
  echoTitle?: string
}
const batchItems = ref<BatchRowItem[]>([])
const batchItemType = ref<'ve' | 'vl'>('ve')
const batchStartId = ref<number | undefined>(undefined)
const batchStep = ref(1)

const isBatch = computed(() => batchItems.value.length > 1 || (batchItems.value.length === 1 && !singleItem.value))

const canSubmit = computed(() => {
  if (submitting.value) return false
  if (!isBatch.value) {
    return Boolean(singleItem.value && singleForm.value.item_type && Number(singleForm.value.item_id) > 0)
  }
  return batchItems.value.length > 0 && batchItems.value.every((row) => row.item_type && Number(row.item_id) > 0)
})

function open(items: OpenListEntry[]) {
  if (!items.length) return

  keepLocalFile.value = false
  if (items.length === 1) {
    singleItem.value = items[0]
    batchItems.value = []
    singleForm.value = {
      item_type: 've',
      item_id: undefined,
      item_title: '',
    }
    singleEchoTitle.value = ''
    singleEchoLoading.value = false
  } else {
    singleItem.value = null
    batchItemType.value = 've'
    batchStartId.value = undefined
    batchStep.value = 1
    batchItems.value = items.map((item) => ({
      ...item,
      item_type: 've',
      item_id: undefined,
      echoTitle: '',
    }))
  }
  visible.value = true
}

async function fetchSingleBaseTitle() {
  const type = singleForm.value.item_type
  const id = singleForm.value.item_id
  if (!id || id <= 0) {
    singleEchoTitle.value = ''
    return
  }
  singleEchoLoading.value = true
  try {
    const res = await apiGet<{ title: string }>(`/api/emos/video/base?item_type=${type}&item_id=${id}`)
    singleEchoTitle.value = res.title || ''
  } catch {
    singleEchoTitle.value = ''
  } finally {
    singleEchoLoading.value = false
  }
}

async function fetchRowEchoTitle(row: BatchRowItem) {
  if (!row.item_id || row.item_id <= 0) {
    row.echoTitle = ''
    return
  }
  try {
    const res = await apiGet<{ title: string }>(`/api/emos/video/base?item_type=${row.item_type}&item_id=${row.item_id}`)
    row.echoTitle = res.title || ''
  } catch {
    row.echoTitle = ''
  }
}

function applyBatchType() {
  for (const item of batchItems.value) {
    item.item_type = batchItemType.value
  }
}

function applyBatchSeq() {
  if (!batchStartId.value || batchStartId.value <= 0) {
    ElMessage.warning('请先输入有效的起始 Item ID')
    return
  }
  let currentId = batchStartId.value
  const step = batchStep.value ?? 1
  for (const item of batchItems.value) {
    item.item_type = batchItemType.value
    item.item_id = currentId
    fetchRowEchoTitle(item)
    currentId += step
  }
}

function removeBatchRow(index: number) {
  batchItems.value.splice(index, 1)
  if (batchItems.value.length === 0) {
    visible.value = false
  }
}

async function submitTasks() {
  if (!canSubmit.value) return

  const payloadItems: CreateManualTaskItemPayload[] = []
  if (!isBatch.value && singleItem.value) {
    payloadItems.push({
      path: singleItem.value.path,
      file_name: singleItem.value.name,
      file_size: singleItem.value.size,
      item_type: singleForm.value.item_type,
      item_id: Number(singleForm.value.item_id),
      item_title: singleForm.value.item_title.trim() || singleEchoTitle.value || singleItem.value.name,
    })
  } else {
    for (const row of batchItems.value) {
      payloadItems.push({
        path: row.path,
        file_name: row.name,
        file_size: row.size,
        item_type: row.item_type,
        item_id: Number(row.item_id),
        item_title: row.item_title?.trim() || row.echoTitle || row.name,
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
.manual-upload-dialog {
  padding: 4px 0;
}
.file-preview-box {
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: var(--bg-soft, #f6f8fa);
  padding: 8px 12px;
  border-radius: 8px;
  width: 100%;
}
.file-name {
  font-weight: 500;
  color: var(--text-main, #303133);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.file-size {
  color: var(--text-muted, #909399);
  font-size: 13px;
  flex-shrink: 0;
  margin-left: 12px;
}
.echo-title {
  margin-top: 6px;
  font-size: 13px;
  line-height: 1.4;
}
.echo-title.success {
  color: #67c23a;
}
.echo-title.loading {
  color: #e6a23c;
}
.echo-text {
  color: #67c23a;
  font-weight: 500;
}
.muted-text {
  color: var(--text-muted, #909399);
}
.batch-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  background: var(--bg-soft, #f8f9fa);
  padding: 10px 12px;
  border-radius: 8px;
  margin-bottom: 12px;
}
.batch-setting-item {
  display: flex;
  align-items: center;
  gap: 6px;
}
.batch-setting-item .label {
  font-size: 13px;
  color: var(--text-muted, #606266);
}
.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
</style>
