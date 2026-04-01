<template>
  <section class="glass-panel header-card">
    <div>
      <h1 class="page-title">{{ title }}</h1>
      <p class="page-subtitle">{{ subtitle }}</p>
    </div>
    <div class="header-extra">
      <slot />
    </div>
  </section>
</template>

<script setup lang="ts">
defineProps<{
  title: string
  subtitle: string
}>()
</script>

<style scoped>
.header-card {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: center;
  padding: 24px;
  margin-bottom: 20px;
}

.header-extra {
  display: flex;
  align-items: center;
  gap: 12px;
}

@media (max-width: 768px) {
  .header-card {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>

