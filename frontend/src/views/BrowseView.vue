<template>
  <div class="browse-view">
    <PageHeaderCard title="影片扫描" subtitle="浏览目录 → 勾选文件/文件夹 → 选择 TMDB → 开始扫描。">
      <el-button
        type="primary"
        :loading="scanStore.loading"
        :disabled="!canScan"
        @click="startScan"
      >
        {{ scanButtonLabel }}
      </el-button>
    </PageHeaderCard>

    <el-row :gutter="16">
      <!-- Left: TMDB + settings -->
      <el-col :xs="24" :md="8">
        <el-card class="panel-card">
          <template #header>扫描设置</template>
          <el-form label-position="top">
            <el-form-item label="文件来源">
              <div class="source-toggle">
                <button
                  type="button"
                  :class="['toggle-btn', { active: source === 'openlist' }]"
                  @click="switchSource('openlist')"
                >
                  OpenList
                </button>
                <button
                  type="button"
                  :class="['toggle-btn', { active: source === 'local' }]"
                  @click="switchSource('local')"
                >
                  本地媒体
                </button>
              </div>
              <p class="hint">
                {{ source === 'local'
                  ? '根目录可在「系统配置 → 本地媒体」修改'
                  : '浏览 OpenList 网盘目录' }}
              </p>
            </el-form-item>

            <el-form-item label="搜索影片 (TMDB)">
              <el-input
                v-model="tmdbQuery"
                placeholder="剧名 / 电影名"
                clearable
                @keyup.enter="doSearchTMDB"
              >
                <template #append>
                  <el-button :loading="tmdbLoading" @click="doSearchTMDB">搜索</el-button>
                </template>
              </el-input>
            </el-form-item>

            <div v-if="tmdbResults.length > 0" class="tmdb-results">
              <div
                v-for="item in tmdbResults"
                :key="`${item.media_type}-${item.tmdb_id}`"
                :class="['tmdb-card', { selected: tmdbId === item.tmdb_id && videoType === item.media_type }]"
                @click="selectTMDB(item)"
              >
                <div class="tmdb-poster">
                  <img
                    v-if="item.poster_path"
                    :src="'https://image.tmdb.org/t/p/w92' + item.poster_path"
                    :alt="item.title"
                  />
                  <div v-else class="tmdb-no-poster">—</div>
                </div>
                <div class="tmdb-info">
                  <div class="tmdb-title">{{ item.title }}</div>
                  <div class="tmdb-meta">
                    <span>{{ item.year || '未知' }}</span>
                    <el-tag size="small" effect="plain" :type="item.media_type === 'movie' ? 'warning' : ''">
                      {{ item.media_type === 'movie' ? '电影' : '剧集' }}
                    </el-tag>
                  </div>
                </div>
              </div>
            </div>

            <el-form-item label="TMDB ID">
              <el-input-number
                v-model="tmdbId"
                :min="0"
                style="width: 100%"
                controls-position="right"
                placeholder="也可手动填写"
              />
            </el-form-item>

            <el-form-item label="类型">
              <el-select v-model="videoType" clearable placeholder="自动" style="width: 100%">
                <el-option label="自动" value="" />
                <el-option label="剧集" value="tv" />
                <el-option label="电影" value="movie" />
              </el-select>
            </el-form-item>

            <div v-if="selectionSummary" class="selection-box">
              <div class="selection-title">当前选择</div>
              <div class="selection-text">{{ selectionSummary }}</div>
              <el-button link type="primary" size="small" @click="clearSelection">清空选择</el-button>
            </div>
          </el-form>
        </el-card>
      </el-col>

      <!-- Right: browser -->
      <el-col :xs="24" :md="16">
        <el-card class="panel-card">
          <template #header>
            <div class="browse-header">
              <div class="breadcrumb">
                <button type="button" class="crumb" @click="goTo('/')">根目录</button>
                <template v-for="(seg, idx) in pathSegments" :key="idx">
                  <span class="crumb-sep">/</span>
                  <button type="button" class="crumb" @click="goTo(seg.path)">{{ seg.name }}</button>
                </template>
              </div>
              <div class="browse-actions">
                <el-button v-if="currentPath !== '/'" link type="primary" @click="goUp">上级</el-button>
                <el-button :loading="loading" @click="loadEntries()">刷新</el-button>
              </div>
            </div>
          </template>

          <!-- Table View (Desktop) -->
          <el-table
            v-if="viewMode === 'table'"
            ref="tableRef"
            :data="entries"
            row-key="path"
            stripe
            @selection-change="onSelectionChange"
            @row-dblclick="onRowDblClick"
          >
            <el-table-column type="selection" width="42" :selectable="isSelectable" />
            <el-table-column label="名称" min-width="220" show-overflow-tooltip>
              <template #default="{ row }">
                <button
                  v-if="row.is_dir"
                  type="button"
                  class="name-link"
                  @click="enterDirectory(row.path)"
                >
                  📁 {{ row.name }}
                </button>
                <span v-else>{{ isVideoFile(row.name) ? '🎬' : '📄' }} {{ row.name }}</span>
              </template>
            </el-table-column>
            <el-table-column label="类型" width="72" align="center">
              <template #default="{ row }">
                <el-tag :type="row.is_dir ? 'success' : ''" size="small" effect="plain">
                  {{ row.is_dir ? '目录' : '文件' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="大小" width="100" align="right">
              <template #default="{ row }">
                {{ row.is_dir ? '-' : formatSizeInMB(row.size) }}
              </template>
            </el-table-column>
            <el-table-column label="操作" width="100" fixed="right">
              <template #default="{ row }">
                <el-button v-if="row.is_dir" link type="primary" size="small" @click="enterDirectory(row.path)">
                  进入
                </el-button>
              </template>
            </el-table-column>
          </el-table>

          <!-- Card View (Mobile) -->
          <div v-else class="browse-card-list">
            <el-empty v-if="!entries.length" description="暂无文件" />
            <div
              v-for="row in entries"
              :key="row.path"
              class="browse-card"
              @click="handleCardClick(row)"
            >
              <el-checkbox
                v-if="isSelectable(row)"
                class="card-check"
                :model-value="isCardSelected(row)"
                @click.stop
                @change="toggleSelect(row)"
              />
              <div class="card-icon">{{ row.is_dir ? '📁' : isVideoFile(row.name) ? '🎬' : '📄' }}</div>
              <div class="card-info">
                <div class="card-name">{{ row.name }}</div>
                <div class="card-meta">
                  <span class="card-type">{{ row.is_dir ? '目录' : '文件' }}</span>
                  <span v-if="!row.is_dir" class="card-size">{{ formatSizeInMB(row.size) }}</span>
                </div>
              </div>
              <button
                v-if="row.is_dir"
                type="button"
                class="card-enter"
                @click.stop="enterDirectory(row.path)"
              >
                进入
              </button>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import type { TableInstance } from 'element-plus'

import PageHeaderCard from '@/components/PageHeaderCard.vue'
import { useScanStore } from '@/stores/scans'
import type { OpenListEntry } from '@/types/api'
import { apiFetch, parseApiResponse } from '@/utils/api'
import { formatSizeInMB } from '@/utils/format'
import { extractShowTitle, isVideoFile } from '@/utils/media'

const router = useRouter()
const scanStore = useScanStore()
const tableRef = ref<TableInstance>()

const source = ref<'openlist' | 'local'>('openlist')
const loading = ref(false)
const currentPath = ref('/')
const displayPath = ref('/')
const tmdbId = ref<number | undefined>(undefined)
const tmdbQuery = ref('')
const videoType = ref('')
const entries = ref<OpenListEntry[]>([])
const selected = ref<OpenListEntry[]>([])
const tmdbLoading = ref(false)
const viewMode = ref<'table' | 'card'>(typeof window !== 'undefined' && window.innerWidth < 768 ? 'card' : 'table')
const tmdbResults = ref<Array<{
  tmdb_id: number
  title: string
  year: string
  type: string
  media_type?: string
  poster_path: string
}>>([])

const pathSegments = computed(() => {
  const parts = displayPath.value.replace(/\/+$/, '').split('/').filter(Boolean)
  const segs: Array<{ name: string; path: string }> = []
  let acc = ''
  for (const part of parts) {
    acc += `/${part}`
    segs.push({ name: part, path: acc })
  }
  return segs
})

const selectedDirs = computed(() => selected.value.filter((e) => e.is_dir))
const selectedVideos = computed(() => selected.value.filter((e) => !e.is_dir && isVideoFile(e.name)))

const selectionSummary = computed(() => {
  const d = selectedDirs.value.length
  const f = selectedVideos.value.length
  if (!d && !f) return ''
  const parts: string[] = []
  if (d) parts.push(`${d} 个文件夹`)
  if (f) parts.push(`${f} 个视频`)
  return parts.join(' · ') + '（将递归扫描文件夹内视频）'
})

const canScan = computed(() => {
  const hasTarget = selectedDirs.value.length > 0 || selectedVideos.value.length > 0 || currentPath.value !== ''
  const hasTmdb = Number(tmdbId.value) > 0
  return hasTarget && hasTmdb && !scanStore.loading
})

const scanButtonLabel = computed(() => {
  const n = selectedDirs.value.length + selectedVideos.value.length
  if (n > 0) return `扫描所选 (${n})`
  return '扫描当前目录'
})

const apiBase = () => (source.value === 'local' ? '/api/local/list' : '/api/openlist/list')

function isSelectable(row: OpenListEntry) {
  return row.is_dir || isVideoFile(row.name)
}

function syncViewMode() {
  if (typeof window === 'undefined') return
  viewMode.value = window.innerWidth < 768 ? 'card' : 'table'
}

function isCardSelected(row: OpenListEntry) {
  return selected.value.some((e) => e.path === row.path)
}

function toggleSelect(row: OpenListEntry) {
  const idx = selected.value.findIndex((e) => e.path === row.path)
  if (idx >= 0) {
    selected.value.splice(idx, 1)
  } else {
    selected.value.push(row)
  }
}

function handleCardClick(row: OpenListEntry) {
  if (row.is_dir) {
    enterDirectory(row.path)
    return
  }
  if (isSelectable(row)) {
    toggleSelect(row)
  }
}

function switchSource(next: 'openlist' | 'local') {
  if (source.value === next) return
  source.value = next
  currentPath.value = '/'
  selected.value = []
  loadEntries()
}

async function loadEntries(showLoading = true) {
  if (showLoading) loading.value = true
  try {
    const data = await parseApiResponse<{ path: string; items: OpenListEntry[] }>(
      await apiFetch(`${apiBase()}?path=${encodeURIComponent(currentPath.value)}`),
    )
    displayPath.value = data.path
    entries.value = data.items
    selected.value = []
    await nextTick()
    tableRef.value?.clearSelection()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载失败')
  } finally {
    if (showLoading) loading.value = false
  }
}

function enterDirectory(path: string) {
  currentPath.value = path
  loadEntries()
  const title = extractShowTitle(path)
  if (title) {
    tmdbQuery.value = title
  }
}

function goTo(path: string) {
  currentPath.value = path || '/'
  loadEntries()
}

function goUp() {
  const parts = currentPath.value.replace(/\/$/, '').split('/')
  parts.pop()
  currentPath.value = parts.join('/') || '/'
  loadEntries()
}

function onSelectionChange(rows: OpenListEntry[]) {
  selected.value = rows
}

function onRowDblClick(row: OpenListEntry) {
  if (row.is_dir) enterDirectory(row.path)
}

function clearSelection() {
  selected.value = []
  tableRef.value?.clearSelection()
}
function selectTMDB(item: {
  tmdb_id: number
  title: string
  year: string
  type?: string
  media_type?: string
  poster_path: string
}) {
  tmdbId.value = item.tmdb_id
  const mtype = item.media_type || item.type || ''
  if (mtype === 'tv' || mtype === 'movie') {
    videoType.value = mtype
  }
}

async function doSearchTMDB() {
  const query = tmdbQuery.value.trim()
  if (!query || query.length < 2) {
    tmdbResults.value = []
    return
  }
  tmdbLoading.value = true
  try {
    const [tvResp, mvResp] = await Promise.all([
      apiFetch(`/api/tmdb/search?query=${encodeURIComponent(query)}&type=tv`),
      apiFetch(`/api/tmdb/search?query=${encodeURIComponent(query)}&type=movie`),
    ])
    const [tvData, mvData] = await Promise.all([tvResp.json(), mvResp.json()])
    const tvResults = (tvData.success ? tvData.data : []).map((r: any) => ({ ...r, media_type: 'tv' }))
    const mvResults = (mvData.success ? mvData.data : []).map((r: any) => ({ ...r, media_type: 'movie' }))
    tmdbResults.value = [...tvResults, ...mvResults]
  } catch {
    tmdbResults.value = []
  } finally {
    tmdbLoading.value = false
  }
}

/**
 * Unified scan entry:
 * - If rows selected → pass their paths (files + dirs). Backend expands dirs.
 * - If nothing selected → scan current directory.
 */
async function startScan() {
  if (!tmdbId.value || Number(tmdbId.value) <= 0) {
    ElMessage.warning('请先搜索并选择影片，或手动填写 TMDB ID')
    return
  }

  const targets = selected.value
    .filter((e) => e.is_dir || isVideoFile(e.name))
    .map((e) => e.path)

  const path = currentPath.value || '/'
  const filePaths = targets.length > 0 ? targets : [path]

  try {
    const created = await scanStore.createScan(
      path,
      Number(tmdbId.value),
      videoType.value,
      '',
      source.value,
      filePaths,
    )
    if (created) {
      ElMessage.success(`扫描完成，共 ${created.total_count} 个视频`)
      router.push('/scans')
    }
  } catch (e) {
    ElMessage.error(e instanceof Error ? e.message : '扫描失败')
  }
}

onMounted(() => {
  syncViewMode()
  window.addEventListener('resize', syncViewMode)
  loadEntries()
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', syncViewMode)
})
</script>

<style scoped>
.browse-view { width: 100%; }
.panel-card { border-radius: 16px; margin-bottom: 16px; height: fit-content; }

.browse-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}
.browse-actions { display: flex; align-items: center; gap: 8px; }

.breadcrumb {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 2px;
  font-size: 13px;
  min-width: 0;
}
.crumb {
  border: none;
  background: transparent;
  color: var(--brand);
  cursor: pointer;
  padding: 2px 4px;
  border-radius: 4px;
  font: inherit;
  max-width: 160px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.crumb:hover { background: var(--brand-soft); }
.crumb-sep { color: var(--text-muted); }

.name-link {
  border: none;
  background: none;
  padding: 0;
  color: var(--text-main);
  font: inherit;
  cursor: pointer;
  text-align: left;
}
.name-link:hover { color: var(--brand); }

.source-toggle {
  display: flex;
  gap: 2px;
  padding: 4px;
  background: rgba(0, 0, 0, 0.04);
  border-radius: 12px;
}
html.dark .source-toggle { background: rgba(255, 255, 255, 0.06); }

.toggle-btn {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 10px 12px;
  border: none;
  border-radius: 10px;
  background: transparent;
  color: var(--text-main);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
  font-family: inherit;
}
.toggle-btn:hover:not(.active) { background: rgba(128, 128, 128, 0.1); }
.toggle-btn.active {
  background: var(--el-color-primary);
  color: #fff;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.15);
  font-weight: 600;
}

.hint {
  margin: 8px 0 0;
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.4;
}

.tmdb-results {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 280px;
  overflow-y: auto;
  margin: -4px 0 12px;
}
.tmdb-card {
  display: flex;
  gap: 10px;
  padding: 8px;
  border-radius: 10px;
  cursor: pointer;
  transition: background 0.15s;
  border: 2px solid transparent;
}
.tmdb-card:hover { background: rgba(0, 0, 0, 0.03); }
.tmdb-card.selected {
  border-color: var(--el-color-primary);
  background: rgba(64, 158, 255, 0.06);
}
.tmdb-poster {
  width: 40px;
  height: 56px;
  border-radius: 4px;
  overflow: hidden;
  flex-shrink: 0;
  background: rgba(0, 0, 0, 0.05);
}
.tmdb-poster img { width: 100%; height: 100%; object-fit: cover; }
.tmdb-no-poster {
  width: 100%;
  height: 100%;
  display: grid;
  place-items: center;
  color: var(--text-muted);
  font-size: 12px;
}
.tmdb-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 4px;
}
.tmdb-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-main);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.tmdb-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  color: var(--text-subtle);
}

.selection-box {
  margin-top: 8px;
  padding: 12px;
  border-radius: 10px;
  background: var(--brand-soft);
  border: 1px solid rgba(59, 130, 246, 0.2);
}
.selection-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--brand);
  margin-bottom: 4px;
}
.selection-text {
  font-size: 13px;
  color: var(--text-main);
  margin-bottom: 4px;
}

/* Card View (Mobile) */
.browse-card-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.browse-card {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px;
  border: 1px solid var(--line-soft);
  border-radius: 12px;
  background: var(--bg-panel);
  cursor: pointer;
  transition: border-color 0.15s, box-shadow 0.15s;
  -webkit-tap-highlight-color: transparent;
}
.browse-card:active { border-color: var(--brand); box-shadow: var(--shadow-sm); }

.card-check { flex-shrink: 0; }

.card-icon {
  width: 38px;
  height: 38px;
  display: grid;
  place-items: center;
  font-size: 20px;
  background: var(--bg-hover);
  border-radius: 10px;
  flex-shrink: 0;
}

.card-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.card-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-main);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  word-break: break-all;
}

.card-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--text-muted);
}

.card-type {
  font-size: 11px;
  padding: 1px 6px;
  border-radius: 4px;
  background: var(--bg-hover);
  color: var(--text-subtle);
}

.card-enter {
  flex-shrink: 0;
  border: none;
  background: var(--brand-soft);
  color: var(--brand);
  font-size: 13px;
  font-weight: 600;
  padding: 6px 14px;
  border-radius: 8px;
  cursor: pointer;
  font-family: inherit;
}
.card-enter:active { opacity: 0.8; }
</style>
