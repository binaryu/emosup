<template>
  <el-tag :type="tagType" effect="light" round>{{ status }}</el-tag>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  status: string
}>()

const tagType = computed(() => {
  if (['completed', 'matched'].includes(props.status)) return 'success'
  if (['queued', 'download_completed', 'upload_pending', 'saving', 'pending', 'uploaded', 'unmatched', 'ambiguous'].includes(props.status)) return 'warning'
  if (['download_failed', 'upload_failed', 'canceled', 'invalid', 'failed'].includes(props.status)) return 'danger'
  if (['downloading', 'uploading'].includes(props.status)) return ''
  return 'info'
})
</script>
