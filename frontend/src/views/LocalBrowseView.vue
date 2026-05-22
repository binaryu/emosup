<template>
  <div>
    <PageHeaderCard
      title="本地空间浏览"
      subtitle="浏览 aria2 下载目录中的本地文件，选择文件或目录发起扫描。"
    >
      <el-button :loading="loading" @click="loadEntries">刷新</el-button>
    </PageHeaderCard>

    <el-row :gutter="16">
      <el-col :xs="24" :lg="9">
        <el-card class="panel-card">
          <template #header>扫描输入</template>
          <el-form label-position="top">
            <el-form-item label="当前路径">
              <el-input v-model="currentPath" readonly />
            </el-form-item>
            <el-form-item label="TMDB ID">
              <el-input-number v-model="tmdbId" :min="1" />
            </el-form-item>
            <el-form-item label="Video Type">
              <el-select v-model="videoType" clearable placeholder="自动判断">
                <el-option label="自动判断" value="" />
                <el-option label="TV" value="tv" />
                <el-option label="Movie" value="movie" />
              </el-select>
            </el-form-item>
            <el-form-item>
              <el-button
                type="primary"
                :loading="scanStore.loading"
                :disabled="!selectedPath"
                @click="createScanDir"
              >
                扫描所选目录
              </el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </el-col>

      <el-col :xs="24" :lg="15">
        <el-card class="panel-card">
          <template #header>
            <div class="browse-header">
              <span>目录浏览：{{ displayPath }}</span>
              <div class="browse-actions">
                <el-button
                  v-if="selectedFiles.length > 0"
                  type="warning"
                  size="small"
                  @click="scanSelectedFiles"
                >
                  扫描所选 ({{ selectedFiles.length }})
                </el-button>
                <el-button
                  v-if="displayPath !== '/'"
                  link
                  type="primary"
                  @click="goUp"
                >
                  返回上级
                </el-button>
              </div>
            </div>
          </template>
          <el-table :data="entries" stripe highlight-current-row @current-change="onSelectRow" @selection-change="onFileSelectionChange">
            <el-table-column type="selection" width="42" :selectable="(r: OpenListEntry) => !r.is_dir && isVideoFile(r.name)" />
            <el-table-column prop="name" label="名称" min-width="200" />
            <el-table-column label="类型" width="80" align="center">
              <template #default="{ row }">
                <el-tag :type="row.is_dir ? 'success' : ''" size="small">
                  {{ row.is_dir ? '目录' : '文件' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="大小" width="110" align="right">
              <template #default="{ row }">
                {{ row.is_dir ? '-' : formatSizeInMB(row.size) }}
              </template>
            </el-table-column>
            <el-table-column label="操作" width="180">
              <template #default="{ row }">
                <el-button v-if="row.is_dir" link type="primary" @click="enterDirectory(row.path)">
                  进入
                </el-button>
                <el-button v-if="row.is_dir" link @click="selectForScan(row.path)">
                  选择
                </el-button>
                <el-button
                  v-if="!row.is_dir && isVideoFile(row.name)"
                  link
                  type="warning"
                  @click="scanSingleFile(row)"
                >
                  扫描此文件
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'

import PageHeaderCard from '@/components/PageHeaderCard.vue'
import { useScanStore } from '@/stores/scans'
import type { OpenListEntry } from '@/types/api'
import { parseApiResponse } from '@/utils/api'
import { formatSizeInMB } from '@/utils/format'

const router = useRouter()
const scanStore = useScanStore()

const loading = ref(false)
const currentPath = ref('/')
const displayPath = ref('/')
const selectedPath = ref('')
const tmdbId = ref<number>(1100)
const videoType = ref('')
const entries = ref<OpenListEntry[]>([])
const selectedFiles = ref<OpenListEntry[]>([])

function onFileSelectionChange(rows: OpenListEntry[]) {
  selectedFiles.value = rows
}

async function loadEntries() {
  loading.value = true
  try {
    const data = await parseApiResponse<{ path: string; items: OpenListEntry[] }>(
      await fetch(`/api/local/list?path=${encodeURIComponent(currentPath.value)}`),
    )
    displayPath.value = data.path
    entries.value = data.items
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '目录加载失败')
  } finally {
    loading.value = false
  }
}

function enterDirectory(path: string) {
  currentPath.value = path
  selectedPath.value = ''
  loadEntries()
}

function goUp() {
  const parts = currentPath.value.replace(/\/$/, '').split('/')
  parts.pop()
  currentPath.value = parts.join('/') || '/'
  selectedPath.value = ''
  loadEntries()
}

function selectForScan(path: string) {
  selectedPath.value = path
}

function onSelectRow(row: OpenListEntry | null) {
  if (row && row.is_dir) {
    selectForScan(row.path)
  }
}

async function createScanDir() {
  try {
    const created = await scanStore.createScan(selectedPath.value, tmdbId.value, videoType.value, '', 'local')
    if (created) {
      ElMessage.success(`扫描完成，共 ${created.total_count} 个视频文件`)
      router.push('/scans')
    }
  } catch (error) {
    ElMessage.error(error instanceof Error ? error.message : '扫描失败')
  }
}

async function scanSingleFile(row: OpenListEntry) {
  try {
    const created = await scanStore.createScan(
      row.path,
      tmdbId.value,
      videoType.value,
      row.path,
      'local',
    )
    if (created) {
      ElMessage.success(`单文件扫描完成`)
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
      'local',
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

function isVideoFile(name: string): boolean {
  const ext = name.split('.').pop()?.toLowerCase() || ''
  return ['mp4', 'mkv', 'avi', 'mov', 'wmv', 'flv', 'm4v', 'ts', 'mpg', 'mpeg', 'webm'].includes(ext)
}

onMounted(() => {
  loadEntries()
})
</script>

<style scoped>
.panel-card {
  border-radius: 20px;
}

.browse-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.browse-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
