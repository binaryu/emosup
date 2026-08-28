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
  gap: 20px;
  align-items: center;
  padding: 20px 24px;
  background: var(--bg-panel);
  border: 1px solid var(--line-soft);
  border-radius: 14px;
  box-shadow: var(--shadow-sm);
}

.header-extra {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

@media (max-width: 768px) {
  .header-card {
    flex-direction: column;
    align-items: stretch;
    padding: 16px 18px;
    gap: 14px;
  }
  .header-extra {
    width: 100%;
    justify-content: flex-start;
  }
}
</style>

