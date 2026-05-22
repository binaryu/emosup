<template>
  <div>
    <PageHeaderCard title="系统配置" subtitle="配置外部服务与任务调度参数，修改后点击保存生效。">
      <el-button type="primary" :loading="configStore.loading" @click="configStore.saveConfig()">
        保存配置
      </el-button>
    </PageHeaderCard>

    <!-- 服务地址 -->
    <el-card class="config-card server-card">
      <template #header>服务</template>
      <el-form label-position="top" inline>
        <el-form-item label="监听地址">
          <el-input v-model="configStore.config.server.host" placeholder="0.0.0.0" style="width: 140px" />
        </el-form-item>
        <el-form-item label="端口">
          <el-input-number v-model="configStore.config.server.port" :min="1" :max="65535" style="width: 110px" />
        </el-form-item>
      </el-form>
    </el-card>

    <el-row :gutter="16">
      <el-col :xs="24" :md="12">
        <el-card class="config-card">
          <template #header>Emos</template>
          <el-form label-position="top">
            <el-form-item label="接口地址">
              <el-input v-model="configStore.config.emos.base_url" placeholder="https://emos.best" />
            </el-form-item>
            <el-form-item label="Token">
              <el-input v-model="configStore.config.emos.token" type="password" show-password />
            </el-form-item>
            <el-form-item label="存储位置">
              <el-select v-model="configStore.config.emos.storage" style="width: 100%">
                <el-option label="默认" value="default" />
                <el-option label="国际" value="global" />
                <el-option label="国内" value="internal" />
              </el-select>
            </el-form-item>
          </el-form>
        </el-card>

        <el-card class="config-card">
          <template #header>aria2</template>
          <el-form label-position="top">
            <el-form-item label="RPC 地址">
              <el-input v-model="configStore.config.aria2.rpc_url" placeholder="http://127.0.0.1:6800/jsonrpc" />
            </el-form-item>
            <el-row :gutter="12">
              <el-col :span="12">
                <el-form-item label="密钥">
                  <el-input v-model="configStore.config.aria2.secret" />
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item label="下载目录">
                  <el-input v-model="configStore.config.aria2.download_dir" placeholder="./data/downloads" />
                </el-form-item>
              </el-col>
            </el-row>
          </el-form>
        </el-card>
      </el-col>

      <el-col :xs="24" :md="12">
        <el-card class="config-card">
          <template #header>OpenList</template>
          <el-form label-position="top">
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
            <el-form-item label="Token">
              <el-input v-model="configStore.config.openlist.token" />
            </el-form-item>
          </el-form>
        </el-card>

        <el-card class="config-card">
          <template #header>任务调度</template>
          <el-form label-position="top">
            <el-row :gutter="12">
              <el-col :span="8">
                <el-form-item label="轮询间隔(秒)">
                  <el-input-number v-model="configStore.config.worker.poll_interval_seconds" :min="1" :max="60" />
                </el-form-item>
              </el-col>
              <el-col :span="8">
                <el-form-item label="最大并发">
                  <el-input-number v-model="configStore.config.worker.max_concurrency" :min="1" :max="20" />
                </el-form-item>
              </el-col>
              <el-col :span="8">
                <el-form-item label="上传分片(MB)">
                  <el-input-number v-model="configStore.config.worker.upload_chunk_size_mb" :min="1" :max="64" />
                </el-form-item>
              </el-col>
            </el-row>
            <el-row :gutter="12">
              <el-col :span="12">
                <el-form-item label="保存重试间隔(秒)">
                  <el-input-number v-model="configStore.config.worker.save_retry_interval_seconds" :min="5" />
                </el-form-item>
              </el-col>
              <el-col :span="12">
                <el-form-item label="最大重试次数">
                  <el-input-number v-model="configStore.config.worker.save_retry_max_attempts" :min="1" :max="20" />
                </el-form-item>
              </el-col>
            </el-row>
          </el-form>
        </el-card>
      </el-col>
    </el-row>
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
.config-card {
  margin-bottom: 16px;
  border-radius: 20px;
}

.server-card {
  margin-bottom: 16px;
}

.server-card :deep(.el-card__body) {
  padding: 10px 20px;
}

.config-card :deep(.el-form-item) {
  margin-bottom: 12px;
}

.config-card :deep(.el-card__body) {
  padding: 14px 20px;
}

@media (max-width: 600px) {
  .config-card :deep(.el-card__body),
  .server-card :deep(.el-card__body) {
    padding: 12px;
  }
}
</style>
