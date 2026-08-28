<template>
  <el-tag :type="tagType" effect="light" class="status-tag">{{ labelText }}</el-tag>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  status: string
}>()

const statusLabelMap: Record<string, string> = {
  completed: '已完成',
  matched: '已匹配',
  unmatched: '未匹配',
  ambiguous: '待确认',
  invalid: '无效',
  processing: '处理中',
  queued: '排队中',
  pending: '等待中',
  downloading: '下载中',
  download_completed: '下载完成',
  download_failed: '下载失败',
  upload_pending: '等待上传',
  uploading: '上传中',
  uploaded: '已上传',
  upload_failed: '上传失败',
  saving: '转存中',
  canceled: '已取消',
  failed: '失败',
}

const labelText = computed(() => {
  return statusLabelMap[props.status] || props.status
})

const tagType = computed(() => {
  if (['completed', 'matched'].includes(props.status)) return 'success'
  if (['queued', 'download_completed', 'upload_pending', 'saving', 'pending', 'uploaded', 'unmatched', 'ambiguous'].includes(props.status)) return 'warning'
  if (['download_failed', 'upload_failed', 'canceled', 'invalid', 'failed'].includes(props.status)) return 'danger'
  if (['downloading', 'uploading', 'processing'].includes(props.status)) return 'primary'
  return 'info'
})
</script>

<style scoped>
.status-tag {
  font-weight: 500;
  border-radius: 6px;
  padding: 0 8px;
  letter-spacing: 0.02em;
}
</style>
