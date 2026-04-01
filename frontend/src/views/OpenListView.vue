<template>
  <div>
    <PageHeaderCard
      title="OpenList 浏览"
      subtitle="先选择目录，再结合 tmdb_id 发起扫描。扫描结果与任务队列严格分离。"
    >
      <el-button type="primary" :loading="loading" @click="loadEntries">刷新目录</el-button>
    </PageHeaderCard>

    <el-row :gutter="16">
      <el-col :xs="24" :lg="9">
        <el-card class="panel-card">
          <template #header>扫描输入</template>
          <el-form label-position="top">
            <el-form-item label="当前路径">
              <el-input v-model="currentPath" />
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
              <el-button type="primary" :loading="scanStore.loading" @click="createScan">
                发起扫描
              </el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </el-col>

      <el-col :xs="24" :lg="15">
        <el-card class="panel-card">
          <template #header>目录浏览</template>
          <el-table :data="entries" stripe>
            <el-table-column prop="name" label="名称" />
            <el-table-column label="类型" width="100">
              <template #default="{ row }">
                <el-tag :type="row.is_dir ? 'success' : 'info'">
                  {{ row.is_dir ? '目录' : '文件' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="size" label="大小" width="120" />
            <el-table-column label="操作" width="120">
              <template #default="{ row }">
                <el-button v-if="row.is_dir" link type="primary" @click="enterDirectory(row.path)">
                  进入
                </el-button>
                <el-button v-if="row.is_dir" link @click="selectDirectory(row.path)">
                  选择
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

const router = useRouter()
const scanStore = useScanStore()

const loading = ref(false)
const currentPath = ref('/')
const tmdbId = ref<number>(1100)
const videoType = ref('')
const entries = ref<OpenListEntry[]>([])

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

function enterDirectory(path: string) {
  currentPath.value = path
  loadEntries()
}

function selectDirectory(path: string) {
  currentPath.value = path
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

onMounted(() => {
  loadEntries()
})
</script>

<style scoped>
.panel-card {
  border-radius: 20px;
}
</style>
