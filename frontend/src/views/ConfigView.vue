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
                </el-select>
              </el-form-item>
            </el-col>
          </el-row>
        </el-form>
      </el-card>

      <!-- aria2 下载器 -->
      <el-card class="setting-card">
        <template #header>
          <div class="card-title">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path><polyline points="7 10 12 15 17 10"></polyline><line x1="12" y1="15" x2="12" y2="3"></line></svg>
            Aria2 下载器
          </div>
        </template>
        <el-form label-position="top" class="compact-form">
          <el-form-item label="RPC 地址">
            <el-input v-model="configStore.config.aria2.rpc_url" placeholder="http://127.0.0.1:6800/jsonrpc" />
          </el-form-item>
          <el-row :gutter="12">
            <el-col :span="10">
              <el-form-item label="密钥">
                <el-input v-model="configStore.config.aria2.secret" type="password" show-password />
              </el-form-item>
            </el-col>
            <el-col :span="14">
              <el-form-item label="下载目录">
                <el-input v-model="configStore.config.aria2.download_dir" placeholder="./data/downloads" />
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
            <el-form-item label="分片(MB)">
              <el-input-number v-model="configStore.config.worker.upload_chunk_size_mb" :min="1" :max="64" class="full-width" :controls="false" />
            </el-form-item>
            <el-form-item label="重试间隔(s)">
              <el-input-number v-model="configStore.config.worker.save_retry_interval_seconds" :min="5" class="full-width" :controls="false" />
            </el-form-item>
            <el-form-item label="最大重试">
              <el-input-number v-model="configStore.config.worker.save_retry_max_attempts" :min="1" :max="20" class="full-width" :controls="false" />
            </el-form-item>
          </div>
        </el-form>
      </el-card>

    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import PageHeaderCard from '@/components/PageHeaderCard.vue'
import { useConfigStore } from '@/stores/config'

const configStore = useConfigStore()

onMounted(() => {
  configStore.fetchConfig()
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

@media (max-width: 480px) {
  .cards-grid {
    grid-template-columns: 1fr;
  }
  .metrics-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}
</style>
