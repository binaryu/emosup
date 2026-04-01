<template>
  <el-tag :type="tagType" effect="light" round>{{ status }}</el-tag>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  status: string
}>()

const tagType = computed(() => {
  if (['completed'].includes(props.status)) return 'success'
  if (['queued', 'download_completed', 'upload_pending', 'saving', 'pending', 'uploaded'].includes(props.status)) return 'warning'
  if (['download_failed', 'upload_failed', 'canceled'].includes(props.status)) return 'danger'
  return 'info'
})
</script>
