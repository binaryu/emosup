<template>
  <div class="bt-view">
    <PageHeaderCard
      title="BT 下载"
      subtitle="通过 qBittorrent 添加磁力链接，下载完成后直接扫描匹配并转存（文件保留供做种）。"
    >
      <el-button :loading="qbStore.loading" @click="load" class="tool-btn">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/></svg>
        <span class="btn-label">刷新</span>
      </el-button>
      <el-button type="primary" :loading="qbStore.loading" @click="openAddDialog">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
        <span class="btn-label">添加磁力链接</span>
      </el-button>
    </PageHeaderCard>

    <el-alert
      v-if="!configured"
      class="runtime-alert"
      title="qBittorrent 尚未配置"
      type="warning"
      :closable="false"
      show-icon
    >
      <template #default>
        <span>请在「系统配置 → qBittorrent」填写 WebUI 地址与账号。</span>
        <el-button link type="primary" style="margin-left: 8px" @click="router.push('/config')">去配置</el-button>
      </template>
    </el-alert>

    <el-alert
      v-if="configured && loadError"
      class="runtime-alert"
      :title="loadError"
      type="error"
      :closable="false"
      show-icon
    >
      <template #default>
        <span>请确认 qBittorrent WebUI 已开启（工具 → 选项 → Web UI）且 emosup 可访问。</span>
      </template>
    </el-alert>

    <el-card class="queue-card" :body-style="{ padding: '0' }">
      <el-table :data="qbStore.torrents" row-key="hash" stripe class="task-table">
        <el-table-column label="种子" min-width="260" show-overflow-tooltip>
          <template #default="{ row }">
            <div class="task-target">
              <span class="file-name">{{ row.name }}</span>
              <span class="task-meta">
                <span class="task-id">{{ row.hash.slice(0, 12) }}</span>
                <el-tag v-if="isSeeding(row)" type="success" size="small" effect="plain">做种中</el-tag>
              </span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="进度" min-width="200">
          <template #default="{ row }">
            <div class="progress-cell">
              <el-progress
                :percentage="Math.round(row.progress * 100)"
                :stroke-width="6"
                :status="row.progress >= 1 ? 'success' : undefined"
                :show-text="false"
              />
              <span class="progress-text">{{ formatBytes(row.size) }} · {{ torrentStateLabel(row) }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="做种" width="110" align="right">
          <template #default="{ row }">
            <div class="ratio-cell">
              <span class="ratio-value">{{ row.ratio.toFixed(2) }}</span>
              <span class="ratio-label">↑ {{ formatBytes(row.upspeed) }}/s</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="250" fixed="right">
          <template #default="{ row }">
            <el-button
              type="primary"
              link
              size="small"
              :disabled="row.progress < 1"
              @click="scanTorrent(row)"
            >
              扫描
            </el-button>
            <el-button link size="small" @click="showFiles(row)">文件</el-button>
            <el-button
              :type="row.state === 'pausedUP' || row.state === 'pausedDL' ? 'success' : 'warning'"
              link
              size="small"
              @click="togglePause(row)"
            >
              {{ row.state === 'pausedUP' || row.state === 'pausedDL' ? '继续' : '暂停' }}
            </el-button>
            <el-button type="danger" link size="small" @click="removeTorrent(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!qbStore.torrents.length" description="暂无 BT 任务" style="padding: 48px 0" />
    </el-card>

    <!-- 添加磁力对话框 -->
    <el-dialog v-model="addDialogVisible" title="添加磁力链接" width="min(560px, 92vw)">
      <el-input
        v-model="addUrlsText"
        type="textarea"
        :rows="6"
        placeholder="每行一个磁力链接 (magnet:?xt=...) 或 .torrent 直链"
      />
      <p class="field-hint" style="margin-top: 8px">
        下载到 qBittorrent 保存目录（系统配置可改），完成后点「扫描」转存，本地文件保留做种。
      </p>
      <template #footer>
        <el-button @click="addDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="adding" @click="addTorrents">添加</el-button>
      </template>
    </el-dialog>

    <!-- 文件列表对话框 -->
    <el-dialog v-model="filesDialogVisible" :title="filesDialogTitle" width="min(640px, 92vw)">
      <el-table :data="filesList" row-key="index" size="small" max-height="420">
        <el-table-column prop="name" label="文件名" min-width="240" show-overflow-tooltip />
        <el-table-column label="大小" width="110" align="right">
          <template #default="{ row }">{{ formatBytes(row.size) }}</template>
        </el-table-column>
        <el-table-column label="进度" width="140">
          <template #default="{ row }">
            <el-progress :percentage="Math.round(row.progress * 100)" :stroke-width="5" :show-text="false" />
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRouter } from 'vue-router'

import PageHeaderCard from '@/components/PageHeaderCard.vue'
import { useConfigStore } from '@/stores/config'
import { useQBittorrentStore } from '@/stores/qbittorrent'
import type { QBittorrentTorrent } from '@/types/api'
import { formatBytes } from '@/utils/format'

const router = useRouter()
const configStore = useConfigStore()
const qbStore = useQBittorrentStore()

const loadError = ref('')
const addDialogVisible = ref(false)
const addUrlsText = ref('')
const adding = ref(false)
const filesDialogVisible = ref(false)
const filesDialogTitle = ref('')
const filesList = ref<Array<{ index: number; name: string; size: number; progress: number }>>([])

const configured = computed(() => Boolean(configStore.config.qbittorrent.base_url))

const SEEDING_STATES = ['uploading', 'stalledUP', 'forcedUP']

function isSeeding(row: QBittorrentTorrent) {
  return row.progress >= 1 && (SEEDING_STATES.includes(row.state) || row.state.startsWith('paused'))
}

function torrentStateLabel(row: QBittorrentTorrent) {
  if (row.progress < 1) {
    if (row.dlspeed > 0) return `下载中 ${formatBytes(row.dlspeed)}/s`
    return '等待中'
  }
  if (SEEDING_STATES.includes(row.state)) return '做种中'
  if (row.state.startsWith('paused')) return '已暂停'
  if (row.state.startsWith('error')) return '错误'
  return row.state || '-'
}

function openAddDialog() {
  addUrlsText.value = ''
  addDialogVisible.value = true
}

async function addTorrents() {
  const urls = addUrlsText.value
    .split('\n')
    .map((s) => s.trim())
    .filter((s) => s.length > 0)
  if (!urls.length) {
    ElMessage.warning('请输入至少一个磁力链接')
    return
  }
  adding.value = true
  try {
    await qbStore.addTorrents(urls)
    addDialogVisible.value = false
    ElMessage.success(`已添加 ${urls.length} 个 BT 任务`)
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '添加失败')
  } finally {
    adding.value = false
  }
}

async function scanTorrent(row: QBittorrentTorrent) {
  if (row.progress < 1) {
    ElMessage.warning('种子尚未下载完成，完成后再扫描')
    return
  }
  try {
    const { value: tmdbIdRaw } = await ElMessageBox.prompt('输入 TMDB ID 用于匹配影片', '扫描 BT 资源', {
      confirmButtonText: '开始扫描',
      cancelButtonText: '取消',
      inputType: 'number',
      inputPlaceholder: '例如 1396 (Breaking Bad)',
      inputValidator: (v: string) => (Number(v) > 0 ? true : 'TMDB ID 必须为正整数'),
    })
    const { value: videoType } = await ElMessageBox.prompt('视频类型', '扫描 BT 资源', {
      confirmButtonText: '开始扫描',
      cancelButtonText: '取消',
      inputValue: 'tv',
      inputPlaceholder: 'tv 或 movie',
    })
    const scan = await qbStore.scanTorrent(row.hash, Number(tmdbIdRaw), videoType.trim() || 'tv')
    if (scan) {
      ElMessage.success(`扫描完成：${scan.total_count} 个视频文件`)
      router.push('/scans')
    }
  } catch (error) {
    if (error === 'cancel' || error === 'close') return
    ElMessage.error(error instanceof Error ? error.message : '扫描失败')
  }
}

async function showFiles(row: QBittorrentTorrent) {
  try {
    const files = await qbStore.fetchFiles(row.hash)
    filesList.value = files.map((f) => ({ index: f.index, name: f.name, size: f.size, progress: f.progress }))
    filesDialogTitle.value = `文件列表 · ${row.name}`
    filesDialogVisible.value = true
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '获取文件列表失败')
  }
}

async function togglePause(row: QBittorrentTorrent) {
  try {
    if (row.state === 'pausedUP' || row.state === 'pausedDL') {
      await qbStore.resume([row.hash])
    } else {
      await qbStore.pause([row.hash])
    }
    await load()
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '操作失败')
  }
}

async function removeTorrent(row: QBittorrentTorrent) {
  try {
    const action = await ElMessageBox.confirm(
      `确定删除种子「${row.name}」？可同时删除本地文件（会中断做种）。`,
      '删除 BT 任务',
      {
        confirmButtonText: '仅删种子',
        cancelButtonText: '取消',
        distinguishCancelAndClose: true,
        showClose: false,
      },
    )
    await qbStore.remove([row.hash], false)
    await load()
  } catch (error) {
    if (error === 'cancel') return
    ElMessage.error(error instanceof Error ? error.message : '删除失败')
  }
}

async function load() {
  loadError.value = ''
  try {
    await qbStore.fetchTorrents()
  } catch (error) {
    loadError.value = error instanceof Error ? error.message : '连接 qBittorrent 失败'
  }
}

onMounted(async () => {
  await configStore.fetchConfig()
  if (configured.value) {
    await load()
  }
})
</script>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
.btn-label {
  margin-left: 6px;
}
.progress-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.progress-text {
  font-size: 12px;
  color: var(--text-subtle, #909399);
}
.ratio-cell {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 2px;
}
.ratio-value {
  font-weight: 600;
}
.ratio-label {
  font-size: 12px;
  color: var(--text-subtle, #909399);
}
.field-hint {
  font-size: 12px;
  color: var(--text-subtle, #909399);
  line-height: 1.6;
}
</style>
