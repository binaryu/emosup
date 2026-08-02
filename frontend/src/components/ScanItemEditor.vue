<template>
  <div class="expand-panel">
    <div class="expand-section">
      <div class="expand-title">人工修正</div>
      <div class="edit-grid">
        <div class="edit-field">
          <span class="edit-label">item_id</span>
          <el-input
            :model-value="itemInputs[row.id] ?? (row.selected_item_type && row.selected_item_id ? row.selected_item_type + '-' + row.selected_item_id : (row.selected_item_type || ''))"
            @update:model-value="(val: string) => { itemInputs = { ...itemInputs, [row.id]: val } }"
            placeholder="ve-1829946"
            size="small"
            @blur="() => parseItemID(row)"
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

<script setup lang="ts">
import { ref } from 'vue'

import type { ScanItem, MatchCandidate } from '@/types/api'
import { apiFetch } from '@/utils/api'

defineProps<{
  row: ScanItem
}>()

const itemInputs = ref<Record<string, string>>({})

function parseItemID(row: ScanItem) {
  const value = (itemInputs.value[row.id] || '').trim()
  if (!value) return
  const idx = value.indexOf('-')
  if (idx > 0) {
    row.selected_item_type = value.substring(0, idx)
    row.selected_item_id = parseInt(value.substring(idx + 1), 10) || 0
  } else {
    row.selected_item_type = value
  }
  fetchTitle(row)
}

function applyCandidate(row: ScanItem, candidate: MatchCandidate) {
  row.selected_item_type = candidate.item_type
  row.selected_item_id = candidate.item_id
  row.selected_title = candidate.title
  itemInputs.value = { ...itemInputs.value, [row.id]: candidate.item_type + '-' + candidate.item_id }
}

let fetchTitleTimer: ReturnType<typeof setTimeout> | null = null
async function fetchTitle(row: ScanItem) {
  const itemType = row.selected_item_type?.trim()
  const itemId = row.selected_item_id
  if (!itemType || !itemId || itemId <= 0) return

  if (fetchTitleTimer) clearTimeout(fetchTitleTimer)
  fetchTitleTimer = setTimeout(async () => {
    try {
      const resp = await apiFetch(
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
</script>

<style scoped>
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

@media (max-width: 960px) {
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
