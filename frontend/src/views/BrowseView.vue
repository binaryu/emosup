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
              <el-radio-group v-model="source" @change="loadEntries">
                <el-radio-button value="openlist">OpenList</el-radio-button>
                <el-radio-button value="local">本地下载目录</el-radio-button>
              </el-radio-group>
            </el-form-item>

            <el-form-item label="搜索影片">
              <el-select
                v-model="tmdbId"
                filterable
                remote
                reserve-keyword
                placeholder="输入剧名/电影名搜索"
                :remote-method="searchTMDB"
                :loading="tmdbLoading"
                value-key="tmdb_id"
                style="width: 100%"
                clearable
                popper-class="tmdb-search-dropdown"
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
const tmdbId = ref<number>(0)
const videoType = ref('')
const entries = ref<OpenListEntry[]>([])
const selectedFiles = ref<OpenListEntry[]>([])
const tmdbLoading = ref(false)
const tmdbResults = ref<{ tmdb_id: number; title: string; year: string; type: string }[]>([])

const apiBase = () => source.value === 'local' ? '/api/local/list' : '/api/openlist/list'

async function loadEntries() {
  loading.value = true
  try {
    const data = await parseApiResponse<{ path: string; items: OpenListEntry[] }>(
      await fetch(`${apiBase()}?path=${encodeURIComponent(currentPath.value)}`),
    )
    displayPath.value = data.path
    entries.value = data.items
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '加载失败')
  } finally { loading.value = false }
}

function enterDirectory(path: string) { currentPath.value = path; selectedPath.value = ''; loadEntries() }
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
  const created = await scanStore.createScan(path, tmdbId.value, videoType.value, filePath, source.value, filePaths)
  if (created) {
    ElMessage.success(`扫描完成，${created.total_count} 个视频文件`)
    router.push('/scans')
  }
}

async function scanDir() {
  if (!selectedPath.value) return
  try { await doScan(selectedPath.value) }
  catch (e) { ElMessage.error(e instanceof Error ? e.message : '扫描失败') }
}

async function scanSingleFile(row: OpenListEntry) {
  try { await doScan(row.path, row.path) }
  catch (e) { ElMessage.error(e instanceof Error ? e.message : '扫描失败') }
}

async function scanSelectedFiles() {
  try { await doScan(currentPath.value, '', selectedFiles.value.map(f => f.path)) }
  catch (e) { ElMessage.error(e instanceof Error ? e.message : '扫描失败') }
}

function isVideoFile(name: string): boolean {
  const ext = name.split('.').pop()?.toLowerCase() || ''
  return ['mp4', 'mkv', 'avi', 'mov', 'wmv', 'flv', 'm4v', 'ts', 'mpg', 'mpeg', 'webm'].includes(ext)
}

watch(source, () => { currentPath.value = '/'; selectedPath.value = ''; loadEntries() })
onMounted(loadEntries)
</script>

<style scoped>
.browse-view { width: 100%; }
.panel-card { border-radius: 20px; margin-bottom: 16px; height: fit-content; }
.browse-header { display: flex; align-items: center; justify-content: space-between; }
.browse-actions { display: flex; align-items: center; gap: 8px; }
.path-display { font-size: 12px; color: var(--text-subtle); word-break: break-all; }
</style>
