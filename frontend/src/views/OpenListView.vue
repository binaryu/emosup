<template>
  <div class="explorer-view">
    <PageHeaderCard
      title="OpenList 浏览"
      subtitle="像在本地一样浏览 OpenList 挂载的网盘目录，勾选视频即可发起智能扫描。"
    >
      <el-button type="primary" :loading="loading" @click="loadEntries">
        <template #icon>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"></polyline><polyline points="1 20 1 14 7 14"></polyline><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path></svg>
        </template>
        刷新目录
      </el-button>
    </PageHeaderCard>

    <el-row :gutter="20">
      <!-- File Explorer Area -->
      <el-col :xs="24" :lg="17">
        <el-card class="explorer-card" :body-style="{ padding: '0' }">
          <div class="explorer-toolbar">
            <div class="breadcrumb-nav">
              <el-button link class="up-btn" @click="goUp" :disabled="currentPath === '/'">
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M19 12H5"></path><path d="M12 19l-7-7 7-7"></path></svg>
              </el-button>
              <el-breadcrumb separator="/">
                <el-breadcrumb-item v-for="(part, index) in breadcrumbs" :key="index">
                  <a @click.prevent="navigateTo(index)" class="crumb-link">{{ part || '根目录' }}</a>
                </el-breadcrumb-item>
              </el-breadcrumb>
            </div>
            
            <div class="toolbar-actions">
              <el-button
                v-if="selectedFiles.length > 0"
                type="warning"
                :loading="scanStore.loading"
                @click="scanSelectedFiles"
                round
              >
                批量扫描 ({{ selectedFiles.length }})
              </el-button>
            </div>
          </div>

          <el-table 
            :data="entries" 
            stripe 
            class="explorer-table"
            @selection-change="onFileSelectionChange"
            @row-dblclick="handleRowDblClick"
          >
            <el-table-column type="selection" width="50" :selectable="(r: OpenListEntry) => !r.is_dir && isVideoFile(r.name)" />
            <el-table-column label="名称" min-width="300">
              <template #default="{ row }">
                <div class="file-name-cell" :class="{ 'is-dir': row.is_dir }">
                  <svg v-if="row.is_dir" class="file-icon folder" width="24" height="24" viewBox="0 0 24 24" fill="currentColor" stroke="none"><path d="M10 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2h-8l-2-2z"/></svg>
                  <svg v-else-if="isVideoFile(row.name)" class="file-icon video" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="2" width="20" height="20" rx="2.18" ry="2.18"></rect><line x1="7" y1="2" x2="7" y2="22"></line><line x1="17" y1="2" x2="17" y2="22"></line><line x1="2" y1="12" x2="22" y2="12"></line><line x1="2" y1="7" x2="7" y2="7"></line><line x1="2" y1="17" x2="7" y2="17"></line><line x1="17" y1="17" x2="22" y2="17"></line><line x1="17" y1="7" x2="22" y2="7"></line></svg>
                  <svg v-else class="file-icon file" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"></path><polyline points="13 2 13 9 20 9"></polyline></svg>
                  <span class="name-text">{{ row.name }}</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column prop="size" label="大小" width="120" align="right" />
            <el-table-column label="操作" width="180" align="right">
              <template #default="{ row }">
                <el-button v-if="row.is_dir" link type="primary" @click="enterDirectory(row.path)">
                  进入
                </el-button>
                <el-button v-if="row.is_dir" link @click="selectDirectory(row.path)">
                  设为目标
                </el-button>
                <el-button
                  v-if="!row.is_dir && isVideoFile(row.name)"
                  link
                  type="warning"
                  @click="scanSingleFile(row)"
                >
                  扫描
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>

      <!-- Scan Configuration Side Panel -->
      <el-col :xs="24" :lg="7">
        <div class="sticky-panel">
          <el-card class="scan-config-card">
            <template #header>
              <div class="card-header">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"></circle><line x1="21" y1="21" x2="16.65" y2="16.65"></line><line x1="11" y1="8" x2="11" y2="14"></line><line x1="8" y1="11" x2="14" y2="11"></line></svg>
                <span>扫描设置</span>
              </div>
            </template>
            <el-form label-position="top">
              <el-form-item label="目标目录 (支持手动输入路径回车跳转)">
                <el-input v-model="currentPath" @keyup.enter="loadEntries">
                  <template #prefix>
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path></svg>
                  </template>
                </el-input>
              </el-form-item>
              
              <el-form-item label="搜索影片">
                <el-select
                  v-model="tmdbId"
                  filterable
                  remote
                  reserve-keyword
                  placeholder="输入剧名/电影名搜索 TMDB"
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
                    <div style="display: flex; justify-content: space-between; align-items: center">
                      <span>{{ item.title }}</span>
                      <span style="color: var(--text-subtle); font-size: 12px; margin-left: 8px">{{ item.year || '' }}</span>
                    </div>
                  </el-option>
                </el-select>
              </el-form-item>

              <el-row :gutter="16">
                <el-col :span="12">
                  <el-form-item label="类型">
                    <el-select v-model="videoType" clearable placeholder="自动匹配" style="width: 100%">
                      <el-option label="自动匹配" value="" />
                      <el-option label="剧集 (TV)" value="tv" />
                      <el-option label="电影 (Movie)" value="movie" />
                    </el-select>
                  </el-form-item>
                </el-col>
              </el-row>
              
              <div class="submit-action">
                <el-button type="primary" size="large" :loading="scanStore.loading" @click="createScan" class="full-btn">
                  全目录扫描
                </el-button>
                <p class="helper-text">扫描目录下所有识别到的视频文件并加入上传队列</p>
              </div>
            </el-form>
          </el-card>
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'

import PageHeaderCard from '@/components/PageHeaderCard.vue'
import { useScanStore } from '@/stores/scans'
import type { OpenListEntry } from '@/types/api'
import { parseApiResponse } from '@/utils/api'

const router = useRouter()
const scanStore = useScanStore()

const loading = ref(false)
const currentPath = ref('/')
const tmdbId = ref<number>(1100)
const videoType = ref('')
const entries = ref<OpenListEntry[]>([])
const selectedFiles = ref<OpenListEntry[]>([])
const tmdbLoading = ref(false)
const tmdbResults = ref<{ tmdb_id: number; title: string; year: string; type: string }[]>([])

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

const breadcrumbs = computed(() => {
  if (currentPath.value === '/') return ['']
  return currentPath.value.split('/')
})

function onFileSelectionChange(rows: OpenListEntry[]) {
  selectedFiles.value = rows
}

async function loadEntries() {
  loading.value = true
  try {
    const data = await parseApiResponse<{ path: string; items: OpenListEntry[] }>(
      await fetch(`/api/openlist/list?path=${encodeURIComponent(currentPath.value)}`),
    )
    entries.value = data.items
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '目录加载失败')
  } finally {
    loading.value = false
  }
}

function handleRowDblClick(row: OpenListEntry) {
  if (row.is_dir) {
    enterDirectory(row.path)
  }
}

function enterDirectory(path: string) {
  currentPath.value = path
  loadEntries()
}

function selectDirectory(path: string) {
  currentPath.value = path
}

function goUp() {
  if (currentPath.value === '/') return
  const parts = currentPath.value.replace(/\/$/, '').split('/')
  parts.pop()
  currentPath.value = parts.join('/') || '/'
  loadEntries()
}

function navigateTo(index: number) {
  if (index === 0) {
    currentPath.value = '/'
  } else {
    currentPath.value = breadcrumbs.value.slice(0, index + 1).join('/')
  }
  loadEntries()
}

async function createScan() {
  try {
    const created = await scanStore.createScan(currentPath.value, tmdbId.value, videoType.value)
    if (created) {
      ElMessage.success(`扫描完成，共 ${created.total_count} 个视频文件`)
      router.push('/scans')
    }
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '扫描失败')
  }
}

function isVideoFile(name: string): boolean {
  const ext = name.split('.').pop()?.toLowerCase() || ''
  return ['mp4', 'mkv', 'avi', 'mov', 'wmv', 'flv', 'm4v', 'ts', 'mpg', 'mpeg', 'webm'].includes(ext)
}

async function scanSingleFile(row: OpenListEntry) {
  try {
    const created = await scanStore.createScan(
      row.path,
      tmdbId.value,
      videoType.value,
      row.path,
    )
    if (created) {
      ElMessage.success('单文件扫描完成')
      router.push('/scans')
    }
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '扫描失败')
  }
}

async function scanSelectedFiles() {
  const files = selectedFiles.value
  if (!files.length) return

  try {
    const created = await scanStore.createScan(
      currentPath.value,
      tmdbId.value,
      videoType.value,
      '',
      '',
      files.map((f) => f.path),
    )
    if (created) {
      ElMessage.success(`多文件扫描完成，共 ${created.total_count} 个视频文件`)
      router.push('/scans')
    }
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '扫描失败')
  }
}

onMounted(() => {
  loadEntries()
})
</script>

<style scoped>
.explorer-view {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.explorer-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: var(--bg-hover);
  border-bottom: 1px solid var(--line-soft);
}

.breadcrumb-nav {
  display: flex;
  align-items: center;
  gap: 8px;
}

.up-btn {
  padding: 4px;
  height: auto;
  color: var(--text-subtle);
}

.up-btn:hover:not(:disabled) {
  color: var(--brand);
}

.crumb-link {
  cursor: pointer;
  color: var(--text-main);
  font-weight: 500;
}

.crumb-link:hover {
  color: var(--brand);
}

.file-name-cell {
  display: flex;
  align-items: center;
  gap: 10px;
}

.file-name-cell.is-dir .name-text {
  font-weight: 500;
}

.file-icon {
  flex-shrink: 0;
}

.file-icon.folder {
  color: #3b82f6; /* consistent folder blue regardless of theme */
}

.file-icon.video {
  color: #eab308;
}

.file-icon.file {
  color: var(--text-muted);
}

.name-text {
  user-select: none;
}

.explorer-table :deep(.el-table__row) {
  cursor: pointer;
}

.sticky-panel {
  position: sticky;
  top: 80px;
}

.card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  font-size: 16px;
}

.submit-action {
  margin-top: 24px;
  text-align: center;
}

.full-btn {
  width: 100%;
  border-radius: 10px;
}

.helper-text {
  margin: 10px 0 0;
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.4;
}

@media (max-width: 1200px) {
  .sticky-panel {
    position: static;
    margin-top: 16px;
  }
}
</style>
