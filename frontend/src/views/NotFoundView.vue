<template>
  <div class="glass-panel not-found">
    <h1 class="page-title">页面不存在</h1>
    <p class="page-subtitle">请从左侧导航返回配置、扫描或任务页面。</p>
    <RouterLink to="/config">
      <el-button type="primary">返回首页</el-button>
    </RouterLink>
  </div>
</template>

<script setup lang="ts">
</script>

<style scoped>
.not-found {
  padding: 48px;
}
</style>
