<template>
  <div class="page-shell">
    <aside class="sidebar">
      <div class="brand">
        <span class="brand-badge">EM</span>
        <div>
          <strong>Emos Upload Panel</strong>
          <p>Queue-first MVP</p>
        </div>
      </div>

      <nav class="nav-list">
        <RouterLink v-for="item in navItems" :key="item.to" :to="item.to" class="nav-link">
          {{ item.label }}
        </RouterLink>
      </nav>
    </aside>

    <main class="page-main">
      <RouterView />
    </main>
  </div>
</template>

<script setup lang="ts">
const navItems = [
  { label: '配置页', to: '/config' },
  { label: 'OpenList 浏览', to: '/openlist' },
  { label: '扫描结果', to: '/scans' },
  { label: '任务队列', to: '/tasks' },
]
</script>

<style scoped>
.sidebar {
  width: 280px;
  padding: 24px;
  background: linear-gradient(180deg, #173c32 0%, #102b24 100%);
  color: #f6f2e9;
}

.brand {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 28px;
}

.brand strong {
  display: block;
  font-size: 18px;
}

.brand p {
  margin: 6px 0 0;
  color: rgba(246, 242, 233, 0.72);
  font-size: 13px;
}

.brand-badge {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.1);
  font-weight: 700;
}

.nav-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.nav-link {
  padding: 12px 14px;
  border-radius: 14px;
  color: rgba(246, 242, 233, 0.88);
  transition: background-color 0.2s ease, transform 0.2s ease;
}

.nav-link.router-link-active {
  background: rgba(255, 255, 255, 0.12);
  color: #fff;
}

.nav-link:hover {
  background: rgba(255, 255, 255, 0.08);
  transform: translateX(2px);
}

@media (max-width: 960px) {
  .sidebar {
    width: 100%;
    padding-bottom: 16px;
  }

  .nav-list {
    flex-direction: row;
    flex-wrap: wrap;
  }
}
</style>
