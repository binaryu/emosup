<template>
  <div class="browse-view">
    <PageHeaderCard title="影片扫描" subtitle="选择来源、搜索影片、选中文件后发起扫描。">
      <el-button type="primary" :loading="scanStore.loading" @click="scanSelectedFiles" :disabled="selectedFiles.length === 0">
        扫描所选 ({{ selectedFiles.length }})
      </el-button>
    </PageHeaderCard>

    <el-row :gutter="16">
      <el-col :xs="24" :md="8">
        <el-card class="panel-card">
          <template #header>扫描设置</template>
          <el-form label-position="top">
            <el-form-item label="文件来源">
              <div class="source-toggle">
                <button
                  type="button"
                  :class="['toggle-btn', { active: source === 'openlist' }]"
                  @click="source = 'openlist'"
                >
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"></path><polyline points="15 3 21 3 21 9"></polyline><line x1="10" y1="14" x2="21" y2="3"></line></svg>
                  OpenList
                </button>
                <button
                  type="button"
                  :class="['toggle-btn', { active: source === 'local' }]"
                  @click="source = 'local'"
                >
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="3" width="20" height="14" rx="2" ry="2"></rect><line x1="8" y1="21" x2="16" y2="21"></line><line x1="12" y1="17" x2="12" y2="21"></line></svg>
                  本地目录
                </button>
              </div>
            </el-form-item>

            <el-form-item label="搜索影片">
              <el-select
                v-model="tmdbId"
                filterable
                remote
                reserve-keyword
                placeholder="输入剧名搜索，或下方手动填 ID"
                :remote-method="searchTMDB"
                :loading="tmdbLoading"
                value-key="tmdb_id"
                style="width: 100%"
                clearable
              >
                <el-option
                  v-for="item in tmdbResults"
                  :key="item.tmdb_id"
                  :label="`${item.title}${item.year ? ' (' + item.year + ')' : ''}`"
                  :value="item.tmdb_id"
                >
                  <div style="display: flex; justify-content: space-between">
                    <span>{{ item.title }}</span>
                    <span style="color: var(--text-subtle); font-size: 12px">{{ item.year || '' }}</span>
                  </div>
                </el-option>
              </el-select>
            </el-form-item>

            <el-form-item label="或手动输入 TMDB ID">
              <el-input-number v-model="tmdbId" :min="0" style="width: 100%" controls-position="right" placeholder="搜不到时直接填数字 ID" />
            </el-form-item>

            <el-form-item label="类型">
              <el-select v-model="videoType" clearable placeholder="自动" style="width: 100%">
                <el-option label="自动" value="" />
                <el-option label="剧集" value="tv" />
                <el-option label="电影" value="movie" />
              </el-select>
            </el-form-item>

            <el-form-item v-if="source === 'openlist' && !selectedPath" label="当前路径">
              <div class="path-display">{{ displayPath }}</div>
            </el-form-item>
            <el-form-item v-if="selectedPath" label="已选目录">
              <el-tag closable @close="selectedPath = ''">{{ selectedPath }}</el-tag>
            </el-form-item>

            <el-form-item>
              <el-button type="primary" :loading="scanStore.loading" :disabled="!selectedPath" @click="scanDir" style="width: 100%">
                扫描目录
              </el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </el-col>

      <el-col :xs="24" :md="16">
        <el-card class="panel-card">
          <template #header>
            <div class="browse-header">
              <span>{{ displayPath }}</span>
              <div class="browse-actions">
                <el-button v-if="displayPath !== '/'" link type="primary" @click="goUp">返回上级</el-button>
                <el-button :loading="loading" @click="loadEntries">刷新</el-button>
              </div>
            </div>
          </template>

          <el-table :data="entries" stripe @selection-change="onFileSelectionChange">
            <el-table-column type="selection" width="42" :selectable="(r: OpenListEntry) => !r.is_dir && isVideoFile(r.name)" />
            <el-table-column prop="name" label="名称" min-width="200" show-overflow-tooltip />
            <el-table-column label="类型" width="70" align="center">
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
            <el-table-column label="操作" width="160" fixed="right">
              <template #default="{ row }">
                <el-button v-if="row.is_dir" link type="primary" size="small" @click="enterDirectory(row.path)">进入</el-button>
                <el-button v-if="row.is_dir" link size="small" @click="selectForScan(row.path)">选此目录</el-button>
                <el-button v-if="!row.is_dir && isVideoFile(row.name)" link type="warning" size="small" @click="scanSingleFile(row)">扫描</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'

import PageHeaderCard from '@/components/PageHeaderCard.vue'
import { useScanStore } from '@/stores/scans'
import type { OpenListEntry } from '@/types/api'
import { parseApiResponse } from '@/utils/api'
import { formatSizeInMB } from '@/utils/format'

const router = useRouter()
const scanStore = useScanStore()

const source = ref<'openlist' | 'local'>('openlist')
const loading = ref(false)
const currentPath = ref('/')
const displayPath = ref('/')
const selectedPath = ref('')
const tmdbId = ref<number | ''>('')
const videoType = ref('')
const entries = ref<OpenListEntry[]>([])
const selectedFiles = ref<OpenListEntry[]>([])
const tmdbLoading = ref(false)
const tmdbResults = ref<{ tmdb_id: number; title: string; year: string; type: string }[]>([])

const apiBase = () => source.value === 'local' ? '/api/local/list' : '/api/openlist/list'

async function loadEntries(showLoading = true) {
  if (showLoading) loading.value = true
  try {
    const data = await parseApiResponse<{ path: string; items: OpenListEntry[] }>(
      await fetch(`${apiBase()}?path=${encodeURIComponent(currentPath.value)}`),
    )
    displayPath.value = data.path
    entries.value = data.items
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载失败')
  } finally { if (showLoading) loading.value = false }
}

function enterDirectory(path: string) { currentPath.value = path; selectedPath.value = ''; loadEntries(); autoDetectTMDB(path) }
function goUp() {
  const parts = currentPath.value.replace(/\/$/, '').split('/')
  parts.pop()
  currentPath.value = parts.join('/') || '/'
  selectedPath.value = ''
  loadEntries()
}
function selectForScan(path: string) { selectedPath.value = path }
function onFileSelectionChange(rows: OpenListEntry[]) { selectedFiles.value = rows }

let tmdbTimer: ReturnType<typeof setTimeout> | undefined
async function searchTMDB(query: string) {
  if (!query || query.length < 2) { tmdbResults.value = []; return }
  if (tmdbTimer) clearTimeout(tmdbTimer)
  tmdbTimer = setTimeout(async () => {
    tmdbLoading.value = true
    try {
      const t = videoType.value || 'tv'
      const resp = await fetch(`/api/tmdb/search?query=${encodeURIComponent(query)}&type=${t}`)
      const data = await resp.json()
      tmdbResults.value = data.success ? data.data : []
    } catch { tmdbResults.value = [] }
    finally { tmdbLoading.value = false }
  }, 400)
}

async function doScan(path: string, filePath = '', filePaths: string[] = []) {
  const created = await scanStore.createScan(path, Number(tmdbId.value) || 0, videoType.value, filePath, source.value, filePaths)
  if (created) {
    ElMessage.success(`扫描完成，${created.total_count} 个视频文件`)
    router.push('/scans')
  }
}

async function scanDir() {
  if (!selectedPath.value) return
  if (!tmdbId.value || Number(tmdbId.value) <= 0) { ElMessage.warning('请先在上方搜索影片'); return }
  try { await doScan(selectedPath.value) }
  catch (e) { ElMessage.error(e instanceof Error ? e.message : '扫描失败') }
}

async function scanSingleFile(row: OpenListEntry) {
  if (!tmdbId.value || Number(tmdbId.value) <= 0) { ElMessage.warning('请先搜索影片或手动填写 TMDB ID'); return }
  try { await doScan(row.path, row.path) }
  catch (e) { ElMessage.error(e instanceof Error ? e.message : '扫描失败') }
}

async function scanSelectedFiles() {
  if (!tmdbId.value || Number(tmdbId.value) <= 0) { ElMessage.warning('请先在上方搜索影片'); return }
  try { await doScan(currentPath.value, '', selectedFiles.value.map(f => f.path)) }
  catch (e) { ElMessage.error(e instanceof Error ? e.message : '扫描失败') }
}

function isVideoFile(name: string): boolean {
  const ext = name.split('.').pop()?.toLowerCase() || ''
  return ['mp4', 'mkv', 'avi', 'mov', 'wmv', 'flv', 'm4v', 'ts', 'mpg', 'mpeg', 'webm'].includes(ext)
}

function extractShowName(path: string): string {
  // Get the last meaningful directory name
  const parts = path.replace(/\/$/, '').split('/')
  const last = parts[parts.length - 1]
  if (!last || last === '') return ''
  // Strip common patterns: [group], season info, year
  let name = last
    .replace(/\[.*?\]/g, ' ')       // remove [bracketed tags]
    .replace(/\(.*?\)/g, ' ')       // remove (parentheses)
    .replace(/S\d{1,2}/gi, ' ')     // remove S01
    .replace(/Season\s*\d+/gi, ' ') // remove Season 1
    .replace(/\d{4}/g, ' ')         // remove years
    .replace(/\s+/g, ' ')           // collapse whitespace
    .trim()
  return name || parts[parts.length - 2]?.replace(/\[.*?\]/g, ' ').trim() || ''
}

async function autoDetectTMDB(path: string) {
  const name = extractShowName(path)
  if (!name || name.length < 2) return
  tmdbLoading.value = true
  try {
    const t = videoType.value || 'tv'
    const resp = await fetch(`/api/tmdb/search?query=${encodeURIComponent(name)}&type=${t}`)
    const data = await resp.json()
    if (data.success && data.data?.length > 0) {
      tmdbResults.value = data.data
    }
  } catch { /* ignore */ }
  finally { tmdbLoading.value = false }
}

watch(source, () => { currentPath.value = '/'; selectedPath.value = ''; loadEntries(false) })
onMounted(() => { loadEntries(); autoDetectTMDB(currentPath.value) })
</script>

<style scoped>
.browse-view { width: 100%; }
.panel-card { border-radius: 20px; margin-bottom: 16px; height: fit-content; }
.browse-header { display: flex; align-items: center; justify-content: space-between; }
.browse-actions { display: flex; align-items: center; gap: 8px; }
.path-display { font-size: 12px; color: var(--text-subtle); word-break: break-all; }

.source-toggle {
  display: flex;
  gap: 2px;
  padding: 4px;
  background: rgba(0,0,0,0.04);
  border-radius: 12px;
}

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
  color: var(--text-subtle);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
  font-family: inherit;
}

.toggle-btn:hover:not(.active) {
  color: var(--text-main);
  background: rgba(0,0,0,0.04);
}

.toggle-btn.active {
  background: #fff;
  color: var(--text-main);
  box-shadow: 0 1px 3px rgba(0,0,0,0.1), 0 1px 2px rgba(0,0,0,0.06);
  font-weight: 600;
}
</style>
