<template>
  <div class="login-page">
    <div class="login-card">
      <div class="login-brand">
        <span class="brand-badge">EM</span>
        <div>
          <h1>Emos Upload</h1>
          <p>登录以管理转存任务</p>
        </div>
      </div>

      <el-form class="login-form" label-position="top" @submit.prevent="onSubmit">
        <el-form-item label="用户名">
          <el-input
            v-model="username"
            placeholder="admin"
            autocomplete="username"
            :disabled="loading"
            size="large"
          />
        </el-form-item>
        <el-form-item label="密码">
          <el-input
            v-model="password"
            type="password"
            placeholder="请输入密码"
            show-password
            autocomplete="current-password"
            :disabled="loading"
            size="large"
            @keyup.enter="onSubmit"
          />
        </el-form-item>

        <el-alert
          v-if="error"
          :title="error"
          type="error"
          show-icon
          :closable="false"
          class="login-error"
        />

        <el-button
          type="primary"
          size="large"
          class="login-btn"
          :loading="loading"
          native-type="submit"
          @click="onSubmit"
        >
          登录
        </el-button>
      </el-form>

      <p class="login-hint">默认账号 admin / admin，请登录后在系统配置中修改。</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
const router = useRouter()
const route = useRoute()

const username = ref('admin')
const password = ref('')
const loading = ref(false)
const error = ref('')

async function onSubmit() {
  error.value = ''
  if (!username.value.trim() || !password.value) {
    error.value = '请输入用户名和密码'
    return
  }

  loading.value = true
  try {
    await authStore.login(username.value.trim(), password.value)
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/tasks'
    await router.replace(redirect.startsWith('/') ? redirect : '/tasks')
  } catch (e) {
    error.value = e instanceof Error ? e.message : '登录失败'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 24px;
  background:
    radial-gradient(circle at 15% 15%, var(--brand-soft), transparent 45%),
    radial-gradient(circle at 85% 85%, var(--brand-soft), transparent 40%),
    var(--bg-app);
}

.login-card {
  width: min(440px, 100%);
  background: var(--bg-panel);
  border: 1px solid var(--line-soft);
  border-radius: 18px;
  box-shadow: var(--shadow-lg);
  padding: 36px 32px 28px;
  backdrop-filter: blur(16px);
}

.login-brand {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 30px;
}

.login-brand h1 {
  margin: 0;
  font-size: 24px;
  font-weight: 700;
  color: var(--text-main);
  letter-spacing: -0.02em;
}

.login-brand p {
  margin: 4px 0 0;
  color: var(--text-subtle);
  font-size: 14px;
}

.brand-badge {
  display: grid;
  place-items: center;
  width: 52px;
  height: 52px;
  border-radius: 14px;
  background: var(--brand-soft);
  border: 1px solid var(--brand-border);
  color: var(--brand);
  font-weight: 700;
  font-size: 20px;
  flex-shrink: 0;
}

.login-form :deep(.el-form-item) {
  margin-bottom: 20px;
}

.login-error {
  margin-bottom: 16px;
  border-radius: 8px;
}

.login-btn {
  width: 100%;
  margin-top: 8px;
  font-weight: 600;
  height: 44px;
  border-radius: 10px;
}

.login-hint {
  margin: 20px 0 0;
  text-align: center;
  color: var(--text-muted);
  font-size: 12.5px;
  line-height: 1.5;
}
</style>
