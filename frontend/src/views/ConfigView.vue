<template>
  <div>
    <PageHeaderCard title="配置页" subtitle="管理 OpenList、aria2、Emos 与 Worker 基础配置。">
      <el-button type="primary" :loading="configStore.loading" @click="configStore.saveConfig()">
        保存配置
      </el-button>
    </PageHeaderCard>

    <el-row :gutter="16">
      <el-col :xs="24" :lg="12">
        <el-card class="config-card">
          <template #header>服务端</template>
          <el-form label-width="120px">
            <el-form-item label="Host">
              <el-input v-model="configStore.config.server.host" />
            </el-form-item>
            <el-form-item label="Port">
              <el-input-number v-model="configStore.config.server.port" :min="1" :max="65535" />
            </el-form-item>
          </el-form>
        </el-card>
      </el-col>
      <el-col :xs="24" :lg="12">
        <el-card class="config-card">
          <template #header>OpenList</template>
          <el-form label-width="120px">
            <el-form-item label="Base URL">
              <el-input v-model="configStore.config.openlist.base_url" />
            </el-form-item>
            <el-form-item label="Username">
              <el-input v-model="configStore.config.openlist.username" />
            </el-form-item>
            <el-form-item label="Password">
              <el-input v-model="configStore.config.openlist.password" type="password" show-password />
            </el-form-item>
            <el-form-item label="Token">
              <el-input v-model="configStore.config.openlist.token" />
            </el-form-item>
          </el-form>
        </el-card>
      </el-col>
      <el-col :xs="24" :lg="12">
        <el-card class="config-card">
          <template #header>aria2</template>
          <el-form label-width="120px">
            <el-form-item label="RPC URL">
              <el-input v-model="configStore.config.aria2.rpc_url" />
            </el-form-item>
            <el-form-item label="Secret">
              <el-input v-model="configStore.config.aria2.secret" />
            </el-form-item>
            <el-form-item label="Download Dir">
              <el-input v-model="configStore.config.aria2.download_dir" />
            </el-form-item>
            <el-form-item label="Poll Interval">
              <el-input-number v-model="configStore.config.aria2.poll_interval_seconds" :min="1" />
            </el-form-item>
            <el-form-item label="Connect Timeout">
              <el-input-number v-model="configStore.config.aria2.connect_timeout_seconds" :min="1" />
            </el-form-item>
          </el-form>
        </el-card>
      </el-col>
      <el-col :xs="24" :lg="12">
        <el-card class="config-card">
          <template #header>Emos / Worker</template>
          <el-form label-width="120px">
            <el-form-item label="Emos URL">
              <el-input v-model="configStore.config.emos.base_url" />
            </el-form-item>
            <el-form-item label="Emos Token">
              <el-input v-model="configStore.config.emos.token" type="password" show-password />
            </el-form-item>
            <el-form-item label="Storage">
              <el-input v-model="configStore.config.emos.storage" />
            </el-form-item>
            <el-form-item label="Poll Interval">
              <el-input-number v-model="configStore.config.worker.poll_interval_seconds" :min="1" />
            </el-form-item>
            <el-form-item label="Chunk Size MB">
              <el-input-number v-model="configStore.config.worker.upload_chunk_size_mb" :min="1" />
            </el-form-item>
            <el-form-item label="Save Interval">
              <el-input-number v-model="configStore.config.worker.save_retry_interval_seconds" :min="5" />
            </el-form-item>
            <el-form-item label="Save Retries">
              <el-input-number v-model="configStore.config.worker.save_retry_max_attempts" :min="1" />
            </el-form-item>
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
</style>
