<template>
  <div class="config-view">
    <PageHeaderCard title="系统配置" subtitle="管理外部服务集成与核心任务调度参数。修改后点击保存生效。">
      <el-button type="primary" :loading="configStore.loading" @click="configStore.saveConfig()">
        保存所有配置
      </el-button>
    </PageHeaderCard>

    <div class="cards-grid">
      
      <!-- 服务监听 -->
      <el-card class="setting-card">
        <template #header>
          <div class="card-title">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="2" width="20" height="8" rx="2" ry="2"></rect><rect x="2" y="14" width="20" height="8" rx="2" ry="2"></rect><line x1="6" y1="6" x2="6.01" y2="6"></line><line x1="6" y1="18" x2="6.01" y2="18"></line></svg>
            基础服务
          </div>
        </template>
        <el-form label-position="top" class="compact-form">
          <el-row :gutter="12">
            <el-col :span="14">
              <el-form-item label="监听地址">
                <el-input v-model="configStore.config.server.host" placeholder="0.0.0.0" />
              </el-form-item>
            </el-col>
            <el-col :span="10">
              <el-form-item label="端口">
                <el-input-number v-model="configStore.config.server.port" :min="1" :max="65535" class="full-width" :controls="false" />
              </el-form-item>
            </el-col>
          </el-row>
        </el-form>
      </el-card>

      <!-- 本地媒体目录 -->
      <el-card class="setting-card">
        <template #header>
          <div class="card-title">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path></svg>
            本地媒体
          </div>
        </template>
        <el-form label-position="top" class="compact-form">
          <el-form-item label="浏览根目录">
            <el-input
              v-model="configStore.config.local.root"
              placeholder="留空则使用下载目录；例如 /home/user/videos 或 /mnt/media"
              clearable
            />
          </el-form-item>
          <el-form-item label="下载缓存目录">
            <el-input
              v-model="configStore.config.aria2.download_dir"
              placeholder="OpenList 任务下载到此目录"
            />
          </el-form-item>
          <p class="field-hint">
            「本地媒体」扫描使用浏览根目录（须为已存在的绝对路径）。二进制部署可设为任意本机路径，如 <code>/home/user/downloads</code>。
            环境变量 <code>EMOSUP_LOCAL_ROOT</code> 优先级更高（Docker 常用）。
          </p>
        </el-form>
      </el-card>

      <!-- 登录认证 -->
      <el-card class="setting-card">
        <template #header>
          <div class="card-title">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect><path d="M7 11V7a5 5 0 0 1 10 0v4"></path></svg>
            登录认证
          </div>
        </template>
        <el-form label-position="top" class="compact-form">
          <el-form-item label="用户名">
            <el-input v-model="configStore.config.auth.username" placeholder="admin" />
          </el-form-item>
          <el-form-item label="新密码">
            <el-input
              v-model="configStore.config.auth.password"
              type="password"
              show-password
              placeholder="留空则不修改"
            />
          </el-form-item>
          <el-form-item label="Token 有效期 (小时)">
            <el-input-number
              v-model="configStore.config.auth.token_ttl_hours"
              :min="1"
              :max="8760"
              class="full-width"
              :controls="false"
            />
          </el-form-item>
          <p class="field-hint">密码以 bcrypt 哈希存储；JWT 密钥由服务端自动生成，不会返回给前端。</p>
        </el-form>
      </el-card>

      <!-- Emos 后端 -->
      <el-card class="setting-card">
        <template #header>
          <div class="card-title">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><ellipse cx="12" cy="5" rx="9" ry="3"></ellipse><path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"></path><path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"></path></svg>
            Emos 后端
          </div>
        </template>
        <el-form label-position="top" class="compact-form">
          <el-form-item label="接口地址">
            <el-input v-model="configStore.config.emos.base_url" placeholder="https://emos.best" />
          </el-form-item>
          <el-row :gutter="12">
            <el-col :span="14">
              <el-form-item label="Token">
                <el-input v-model="configStore.config.emos.token" type="password" show-password />
              </el-form-item>
            </el-col>
            <el-col :span="10">
              <el-form-item label="存储位置">
                <el-select v-model="configStore.config.emos.storage" class="full-width">
                  <el-option label="默认" value="default" />
                  <el-option label="国际" value="global" />
                  <el-option label="国内" value="internal" />
                  <el-option label="Zn存档服R2" value="zn_r2_upload" />
                </el-select>
              </el-form-item>
            </el-col>
          </el-row>
        </el-form>
      </el-card>

      <!-- OpenList 挂载 -->
      <el-card class="setting-card">
        <template #header>
          <div class="card-title">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path></svg>
            OpenList 挂载
          </div>
        </template>
        <el-form label-position="top" class="compact-form">
          <el-form-item label="接口地址">
            <el-input v-model="configStore.config.openlist.base_url" placeholder="http://..." />
          </el-form-item>
          <el-row :gutter="12">
            <el-col :span="12">
              <el-form-item label="用户名">
                <el-input v-model="configStore.config.openlist.username" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="密码">
                <el-input v-model="configStore.config.openlist.password" type="password" show-password />
              </el-form-item>
            </el-col>
          </el-row>
          <el-form-item label="独立 Token (覆盖账密)">
            <el-input v-model="configStore.config.openlist.token" placeholder="可选填" />
          </el-form-item>
        </el-form>
      </el-card>

      <!-- 任务调度 (Worker) -->
      <el-card class="setting-card worker-card">
        <template #header>
          <div class="card-title">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="3"></circle><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"></path></svg>
            Worker 调度参数
          </div>
        </template>
        <el-form label-position="top" class="compact-form">
          <div class="metrics-grid">
            <el-form-item label="轮询间隔(s)">
              <el-input-number v-model="configStore.config.worker.poll_interval_seconds" :min="1" :max="60" class="full-width" :controls="false" />
            </el-form-item>
            <el-form-item label="最大并发">
              <el-input-number v-model="configStore.config.worker.max_concurrency" :min="1" :max="20" class="full-width" :controls="false" />
            </el-form-item>
            <el-form-item label="下载线程">
              <el-input-number v-model="configStore.config.worker.download_threads" :min="1" :max="16" class="full-width" :controls="false" />
              <span style="font-size:11px;color:var(--text-subtle)">多线程分段下载，4线程约4倍速</span>
            </el-form-item>
            <el-form-item label="分片(MB)">
              <el-input-number v-model="configStore.config.worker.upload_chunk_size_mb" :min="1" :max="512" class="full-width" :controls="false" />
            </el-form-item>
            <el-form-item label="分片并发">
              <el-input-number v-model="configStore.config.worker.upload_part_concurrency" :min="1" :max="10" class="full-width" :controls="false" />
              <span style="font-size:11px;color:var(--text-subtle)">multipart 预签名分片并发上传</span>
            </el-form-item>
            <el-form-item label="重试间隔(s)">
              <el-input-number v-model="configStore.config.worker.save_retry_interval_seconds" :min="5" class="full-width" :controls="false" />
            </el-form-item>
            <el-form-item label="最大重试">
              <el-input-number v-model="configStore.config.worker.save_retry_max_attempts" :min="1" :max="20" class="full-width" :controls="false" />
            </el-form-item>
            <el-form-item label="TMDB API Key">
              <el-input v-model="configStore.config.worker.tmdb_api_key" placeholder="api.themoviedb.org 申请" />
            </el-form-item>
            <el-form-item label="代理下载目录名">
              <el-input v-model="configStore.config.worker.proxy_backends" placeholder="quark,夸克,115" />
              <span style="font-size: 11px; color: var(--text-subtle)">逗号分隔的根目录名，匹配到的走本地代理下载（用于夸克等 302 不兼容的网盘）</span>
            </el-form-item>
          </div>
        </el-form>
      </el-card>

      <!-- 关于与升级 -->
      <el-card class="setting-card about-card">
        <template #header>
          <div class="card-title">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"></polygon></svg>
            关于与升级
          </div>
        </template>

        <div class="about-body">
          <div class="about-row">
            <span class="about-label">当前版本</span>
            <span class="about-value font-mono">{{ currentVersion || '…' }}</span>
            <el-tag v-if="upgradeCheck && upgradeCheck.has_update" type="danger" size="small" effect="plain">有更新</el-tag>
            <el-tag v-else-if="upgradeCheck" type="success" size="small" effect="plain">已是最新</el-tag>
          </div>
          <div class="about-row">
            <span class="about-label">最新版本</span>
            <span class="about-value font-mono">{{ upgradeCheck?.latest || '—' }}</span>
            <span v-if="upgradeCheck?.published_at" class="about-date">{{ formatDate(upgradeCheck.published_at) }}</span>
          </div>

          <div v-if="upgradeLoading" class="about-progress">
            <el-progress :percentage="upgradeProgress" :stroke-width="8" :show-text="true" :status="upgradeError ? 'exception' : undefined" />
            <div class="about-progress-text">{{ upgradeStatusText }}</div>
          </div>

          <div v-if="upgradeCheck?.body" class="about-changelog">
            <div class="about-changelog-title">更新内容</div>
            <div class="about-changelog-body">{{ upgradeCheck.body }}</div>
          </div>

          <div class="about-actions">
            <el-button :loading="checking" @click="checkUpgrade">检查更新</el-button>
            <el-button
              type="primary"
              :disabled="!upgradeCheck?.has_update || upgrading"
              :loading="upgrading"
              @click="runUpgrade"
            >
              立即升级
            </el-button>
            <span v-if="upgradeError" class="about-error">{{ upgradeError }}</span>
          </div>
        </div>
      </el-card>

    </div>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import PageHeaderCard from '@/components/PageHeaderCard.vue'
import { useConfigStore } from '@/stores/config'
import type { UpgradeCheck } from '@/types/api'
import { apiFetch } from '@/utils/api'

const configStore = useConfigStore()

const currentVersion = ref('')
const upgradeCheck = ref<UpgradeCheck | null>(null)
const checking = ref(false)
const upgrading = ref(false)
const upgradeError = ref('')
const upgradeLoading = ref(false)
const upgradeProgress = ref(0)
const upgradeStatusText = ref('')
let healthTimer: number | undefined

function formatDate(iso: string) {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleDateString('zh-CN', { year: 'numeric', month: 'long', day: 'numeric' })
}

async function fetchVersion() {
  try {
    const resp = await apiFetch('/api/system/version')
    const data = await resp.json()
    if (data.success && data.data?.version) {
      const v = String(data.data.version)
      currentVersion.value = v.startsWith('v') ? v : `v${v}`
    }
  } catch {
    currentVersion.value = ''
  }
}

async function checkUpgrade() {
  checking.value = true
  upgradeError.value = ''
  try {
    const resp = await apiFetch('/api/upgrade/check')
    const data = await resp.json()
    if (!data.success) {
      upgradeError.value = data.message || '检查更新失败'
      return
    }
    upgradeCheck.value = data.data
    if (!upgradeCheck.value?.has_update) {
      ElMessage.success('当前已是最新版本')
    }
  } catch (e) {
    upgradeError.value = e instanceof Error ? e.message : '检查更新失败'
  } finally {
    checking.value = false
  }
}

async function runUpgrade() {
  if (!upgradeCheck.value?.has_update) return
  try {
    await ElMessageBox.confirm(
      `确定升级到 v${upgradeCheck.value.latest} 吗？升级期间服务将短暂中断，完成后自动重启。`,
      '升级确认',
      { confirmButtonText: '升级', cancelButtonText: '取消', type: 'warning' },
    )
  } catch {
    return
  }

  upgrading.value = true
  upgradeError.value = ''
  upgradeLoading.value = true
  upgradeProgress.value = 5
  upgradeStatusText.value = '正在下载升级包…'

  try {
    // The server exits right after responding, so a failed fetch is expected.
    await apiFetch('/api/upgrade/run', { method: 'POST' }).catch(() => undefined)
  } catch {
    // connection drop is expected
  }

  upgradeProgress.value = 60
  upgradeStatusText.value = '升级包已就位，等待服务重启…'
  waitForRestart()
}

function waitForRestart() {
  let attempts = 0
  healthTimer = window.setInterval(async () => {
    try {
      const resp = await fetch('/api/health', { cache: 'no-store' })
      if (resp.ok) {
        window.clearInterval(healthTimer)
        upgradeProgress.value = 100
        upgradeStatusText.value = '升级完成，正在刷新…'
        await new Promise((r) => setTimeout(r, 600))
        window.location.reload()
        return
      }
    } catch {
      // server still down — keep polling
    }
    attempts += 1
    upgradeProgress.value = Math.min(60 + attempts * 2, 95)
    upgradeStatusText.value = `等待服务重启… (${attempts * 2}s)`
  }, 2000)
}

onMounted(() => {
  configStore.fetchConfig()
  fetchVersion()
  checkUpgrade()
})

onBeforeUnmount(() => {
  if (healthTimer) window.clearInterval(healthTimer)
})
</script>

<style scoped>
.config-view {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.cards-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(340px, 1fr));
  gap: 16px;
  align-items: start;
}

.setting-card {
  border-radius: 12px;
  height: 100%;
}

.setting-card :deep(.el-card__header) {
  padding: 12px 16px;
  background-color: var(--bg-hover);
  border-bottom: 1px solid var(--line-soft);
}

.card-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 15px;
  font-weight: 600;
  color: var(--text-main);
}

.setting-card :deep(.el-card__body) {
  padding: 16px;
}

.compact-form :deep(.el-form-item) {
  margin-bottom: 14px;
}

.compact-form :deep(.el-form-item__label) {
  padding-bottom: 4px !important;
  font-size: 13px;
  color: var(--text-subtle);
  line-height: 1.2;
}

.full-width {
  width: 100%;
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
}

.field-hint {
  margin: 0;
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.5;
}

.field-hint code {
  font-size: 11px;
  padding: 1px 4px;
  border-radius: 4px;
  background: var(--bg-hover);
}

/* About & Upgrade */
.about-card {
  grid-column: 1 / -1;
}

.about-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.about-row {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 14px;
}

.about-label {
  color: var(--text-subtle);
  font-size: 13px;
  min-width: 72px;
}

.about-value {
  font-weight: 600;
  color: var(--text-main);
}

.about-date {
  font-size: 12px;
  color: var(--text-muted);
}

.font-mono {
  font-family: monospace;
}

.about-changelog {
  border: 1px solid var(--line-soft);
  border-radius: 10px;
  padding: 12px;
  background: var(--bg-hover);
}

.about-changelog-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-main);
  margin-bottom: 8px;
}

.about-changelog-body {
  font-size: 12px;
  color: var(--text-subtle);
  line-height: 1.6;
  max-height: 180px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-word;
}

.about-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.about-progress {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.about-progress-text {
  font-size: 12px;
  color: var(--text-subtle);
}

.about-error {
  font-size: 12px;
  color: #ef4444;
  word-break: break-all;
}

@media (max-width: 480px) {
  .cards-grid {
    grid-template-columns: 1fr;
  }
  .metrics-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>
